// Package prometheus implements the metrics connector for Prometheus and
// API-compatible backends (Thanos, Mimir).
package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/marsad-io/marsad/internal/connector"
	"github.com/marsad-io/marsad/internal/schema"
)

// Connector talks to one Prometheus-compatible endpoint.
type Connector struct {
	name        string
	baseURL     string
	bearerToken string
	client      *http.Client
}

// New builds a connector. client may be nil for http.DefaultClient; the
// gateway injects a transport-restricted client in production.
func New(name, baseURL, bearerToken string, client *http.Client) (*Connector, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("prometheus connector %q: invalid url %q", name, baseURL)
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &Connector{
		name:        name,
		baseURL:     strings.TrimRight(baseURL, "/"),
		bearerToken: bearerToken,
		client:      client,
	}, nil
}

func (c *Connector) Name() string { return c.name }
func (c *Connector) Type() string { return "prometheus" }

// Capabilities declares the unified tools this connector serves.
func (c *Connector) Capabilities() []schema.ToolSpec {
	var specs []schema.ToolSpec
	for _, name := range []string{"query_metrics", "list_metric_names"} {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/-/ready", nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("prometheus %q unreachable: %w", c.name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus %q not ready: HTTP %d", c.name, resp.StatusCode)
	}
	return nil
}

// Execute maps unified tool calls to the Prometheus HTTP API.
func (c *Connector) Execute(ctx context.Context, call connector.ToolCall) (connector.ToolResult, error) {
	switch call.Tool {
	case "query_metrics":
		return c.queryMetrics(ctx, call.Args)
	case "list_metric_names":
		return c.listMetricNames(ctx)
	default:
		return connector.ToolResult{}, fmt.Errorf("prometheus connector does not serve tool %q", call.Tool)
	}
}

func (c *Connector) queryMetrics(ctx context.Context, args map[string]any) (connector.ToolResult, error) {
	query, _ := args["query"].(string)
	start, _ := args["start"].(string)
	end, _ := args["end"].(string)
	step, _ := args["step"].(string)

	isRange := start != "" || end != "" || step != ""
	params := url.Values{"query": {query}}
	endpoint := "/api/v1/query"
	if isRange {
		if start == "" || end == "" || step == "" {
			return connector.ToolResult{}, fmt.Errorf("range query needs start, end, and step together")
		}
		endpoint = "/api/v1/query_range"
		params.Set("start", start)
		params.Set("end", end)
		params.Set("step", step)
	}

	var payload struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	}
	if err := c.getJSON(ctx, endpoint, params, &payload); err != nil {
		return connector.ToolResult{}, err
	}
	return connector.ToolResult{Content: map[string]any{
		"resultType": payload.ResultType,
		"result":     payload.Result,
	}}, nil
}

func (c *Connector) listMetricNames(ctx context.Context) (connector.ToolResult, error) {
	var names []string
	if err := c.getJSON(ctx, "/api/v1/label/__name__/values", nil, &names); err != nil {
		return connector.ToolResult{}, err
	}
	return connector.ToolResult{Content: map[string]any{"metrics": names}}, nil
}

// getJSON performs a GET, checks the Prometheus response envelope, and decodes
// the data field into out.
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
		return fmt.Errorf("prometheus %q unreachable: %w", c.name, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("prometheus %q: reading response: %w", c.name, err)
	}

	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("prometheus %q: HTTP %d with unparsable body: %w", c.name, resp.StatusCode, err)
	}
	if envelope.Status != "success" {
		return fmt.Errorf("prometheus %q: query failed: %s", c.name, envelope.Error)
	}
	return json.Unmarshal(envelope.Data, out)
}

func (c *Connector) do(req *http.Request) (*http.Response, error) {
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	return c.client.Do(req)
}
