// Package loki implements the log connector for Grafana Loki. It maps the
// unified search_logs and list_log_labels tools to the Loki HTTP API and
// returns logs in the backend-agnostic shape (timestamp, labels, line).
package loki

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/marsad-io/marsad/internal/connector"
	"github.com/marsad-io/marsad/internal/schema"
)

// Auth carries the optional credentials for a Loki endpoint. Values come from
// environment variable references in config, never from inline YAML.
type Auth struct {
	BearerToken   string
	BasicUser     string
	BasicPassword string
}

// Connector talks to one Loki endpoint.
type Connector struct {
	name    string
	baseURL string
	auth    Auth
	client  *http.Client
}

// New builds a connector. client may be nil for http.DefaultClient; the
// gateway injects a transport-restricted client in production.
func New(name, baseURL string, auth Auth, client *http.Client) (*Connector, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("loki connector %q: invalid url %q", name, baseURL)
	}
	if auth.BearerToken != "" && (auth.BasicUser != "" || auth.BasicPassword != "") {
		return nil, fmt.Errorf("loki connector %q: bearer and basic auth are mutually exclusive", name)
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Connector{
		name:    name,
		baseURL: strings.TrimRight(baseURL, "/"),
		auth:    auth,
		client:  client,
	}, nil
}

func (c *Connector) Name() string { return c.name }
func (c *Connector) Type() string { return "loki" }

// Capabilities declares the unified tools this connector serves.
func (c *Connector) Capabilities() []schema.ToolSpec {
	var specs []schema.ToolSpec
	for _, name := range []string{"search_logs", "list_log_labels"} {
		spec, ok := schema.Lookup(name)
		if !ok {
			panic("schema missing tool " + name)
		}
		specs = append(specs, spec)
	}
	return specs
}

// Health checks the readiness endpoint.
func (c *Connector) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ready", nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("loki %q unreachable: %w", c.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki %q not ready: HTTP %d", c.name, resp.StatusCode)
	}
	return nil
}

// Execute maps unified tool calls to the Loki HTTP API.
func (c *Connector) Execute(ctx context.Context, call connector.ToolCall) (connector.ToolResult, error) {
	switch call.Tool {
	case "search_logs":
		return c.searchLogs(ctx, call.Args)
	case "list_log_labels":
		return c.listLogLabels(ctx, call.Args)
	default:
		return connector.ToolResult{}, fmt.Errorf("loki connector does not serve tool %q", call.Tool)
	}
}

// listLogLabels returns label names, or the values of one label when the
// label argument is set. An optional start/end window scopes the answer.
func (c *Connector) listLogLabels(ctx context.Context, args map[string]any) (connector.ToolResult, error) {
	params := url.Values{}
	for _, key := range []string{"start", "end"} {
		raw, _ := args[key].(string)
		if raw == "" {
			continue
		}
		ns, err := toNanos(raw)
		if err != nil {
			return connector.ToolResult{}, fmt.Errorf("invalid %s: %w", key, err)
		}
		params.Set(key, ns)
	}

	label, _ := args["label"].(string)
	endpoint := "/loki/api/v1/labels"
	if label != "" {
		endpoint = "/loki/api/v1/label/" + url.PathEscape(label) + "/values"
	}

	var values []string
	if err := c.getJSON(ctx, endpoint, params, &values); err != nil {
		return connector.ToolResult{}, err
	}
	if values == nil {
		values = []string{}
	}

	if label != "" {
		return connector.ToolResult{Content: map[string]any{"label": label, "values": values}}, nil
	}
	return connector.ToolResult{Content: map[string]any{"labels": values}}, nil
}

// entry is the backend-agnostic log entry shape shared by all log connectors.
type entry struct {
	Timestamp string            `json:"timestamp"`
	Labels    map[string]string `json:"labels"`
	Line      string            `json:"line"`

	nanos int64 // internal sort key
}

func (c *Connector) searchLogs(ctx context.Context, args map[string]any) (connector.ToolResult, error) {
	query, _ := args["query"].(string)
	direction, _ := args["direction"].(string)
	if direction == "" {
		direction = "backward"
	}

	params := url.Values{
		"query":     {query},
		"direction": {direction},
	}
	for _, key := range []string{"start", "end"} {
		raw, _ := args[key].(string)
		ns, err := toNanos(raw)
		if err != nil {
			return connector.ToolResult{}, fmt.Errorf("invalid %s: %w", key, err)
		}
		params.Set(key, ns)
	}
	if limit, ok := args["limit"].(float64); ok {
		params.Set("limit", strconv.Itoa(int(limit)))
	}

	var payload struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"`
		} `json:"result"`
	}
	if err := c.getJSON(ctx, "/loki/api/v1/query_range", params, &payload); err != nil {
		return connector.ToolResult{}, err
	}

	var entries []entry
	for _, stream := range payload.Result {
		for _, v := range stream.Values {
			ns, err := strconv.ParseInt(v[0], 10, 64)
			if err != nil {
				return connector.ToolResult{}, fmt.Errorf("loki %q: bad entry timestamp %q", c.name, v[0])
			}
			entries = append(entries, entry{
				Timestamp: time.Unix(0, ns).UTC().Format(time.RFC3339Nano),
				Labels:    stream.Stream,
				Line:      v[1],
				nanos:     ns,
			})
		}
	}
	// Loki orders entries within a stream; ordering across streams is ours.
	sort.SliceStable(entries, func(i, j int) bool {
		if direction == "forward" {
			return entries[i].nanos < entries[j].nanos
		}
		return entries[i].nanos > entries[j].nanos
	})
	if entries == nil {
		entries = []entry{}
	}

	return connector.ToolResult{Content: map[string]any{
		"entries": entries,
		"count":   len(entries),
	}}, nil
}

// toNanos converts a unified time argument (RFC 3339 or Unix seconds) to the
// nanosecond epoch string Loki accepts unambiguously.
func toNanos(s string) (string, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return strconv.FormatInt(t.UnixNano(), 10), nil
	}
	if secs, err := strconv.ParseFloat(s, 64); err == nil {
		return strconv.FormatInt(int64(secs*float64(time.Second)), 10), nil
	}
	return "", fmt.Errorf("%q is neither RFC 3339 nor Unix seconds", s)
}

// getJSON performs a GET, checks the Loki response envelope, and decodes the
// data field into out.
func (c *Connector) getJSON(ctx context.Context, endpoint string, params url.Values, out any) error {
	u := c.baseURL + endpoint
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("loki %q unreachable: %w", c.name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("loki %q: reading response: %w", c.name, err)
	}

	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("loki %q: HTTP %d with unparsable body: %w", c.name, resp.StatusCode, err)
	}
	if envelope.Status != "success" {
		return fmt.Errorf("loki %q: query failed: HTTP %d: %s", c.name, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *Connector) do(req *http.Request) (*http.Response, error) {
	if c.auth.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.auth.BearerToken)
	}
	if c.auth.BasicUser != "" || c.auth.BasicPassword != "" {
		req.SetBasicAuth(c.auth.BasicUser, c.auth.BasicPassword)
	}
	return c.client.Do(req)
}
