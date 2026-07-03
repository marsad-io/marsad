package loki

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/marsad-io/marsad/internal/connector"
)

// fakeLoki implements just enough of the Loki HTTP API to assert the
// connector sends the right requests and parses real response shapes.
func fakeLoki(t *testing.T) (*httptest.Server, *[]*url.URL) {
	t.Helper()
	var requests []*url.URL
	mux := http.NewServeMux()
	mux.HandleFunc("/loki/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"streams","result":[
			{"stream":{"app":"api","level":"info"},"values":[
				["1751536800000000000","request started"],
				["1751536860000000000","request finished"]]},
			{"stream":{"app":"api","level":"error"},"values":[
				["1751536830000000000","upstream timeout"]]}]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &requests
}

func newTestConnector(t *testing.T, url string) *Connector {
	t.Helper()
	c, err := New("loki-test", url, Auth{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type logEntry struct {
	Timestamp string            `json:"timestamp"`
	Labels    map[string]string `json:"labels"`
	Line      string            `json:"line"`
}

type searchResult struct {
	Entries []logEntry `json:"entries"`
	Count   int        `json:"count"`
}

func decodeSearchResult(t *testing.T, res connector.ToolResult) searchResult {
	t.Helper()
	b, err := json.Marshal(res.Content)
	if err != nil {
		t.Fatal(err)
	}
	var out searchResult
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("result %s is not the neutral log shape: %v", b, err)
	}
	return out
}

func TestSearchLogsMapsToQueryRange(t *testing.T) {
	srv, requests := fakeLoki(t)
	c := newTestConnector(t, srv.URL)

	res, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "search_logs",
		Args: map[string]any{
			"query":     `{app="api"}`,
			"start":     "2026-07-03T10:00:00Z",
			"end":       "2026-07-03T11:00:00Z",
			"limit":     float64(500),
			"direction": "forward",
		},
	})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	if len(*requests) != 1 || (*requests)[0].Path != "/loki/api/v1/query_range" {
		t.Fatalf("requests = %v, want one /loki/api/v1/query_range call", *requests)
	}
	q := (*requests)[0].Query()
	if got := q.Get("query"); got != `{app="api"}` {
		t.Errorf("query param = %q", got)
	}
	// Unified times become nanosecond epochs, the format Loki always accepts.
	if got := q.Get("start"); got != "1783072800000000000" {
		t.Errorf("start param = %q, want nanosecond epoch for 2026-07-03T10:00:00Z", got)
	}
	if got := q.Get("end"); got != "1783076400000000000" {
		t.Errorf("end param = %q, want nanosecond epoch for 2026-07-03T11:00:00Z", got)
	}
	if got := q.Get("limit"); got != "500" {
		t.Errorf("limit param = %q, want 500", got)
	}
	if got := q.Get("direction"); got != "forward" {
		t.Errorf("direction param = %q, want forward", got)
	}

	out := decodeSearchResult(t, res)
	if out.Count != 3 || len(out.Entries) != 3 {
		t.Fatalf("count = %d, entries = %d, want 3 flattened entries", out.Count, len(out.Entries))
	}
	// forward = ascending by timestamp across streams
	wantLines := []string{"request started", "upstream timeout", "request finished"}
	for i, want := range wantLines {
		if out.Entries[i].Line != want {
			t.Errorf("entries[%d].Line = %q, want %q", i, out.Entries[i].Line, want)
		}
	}
	e := out.Entries[0]
	if e.Labels["app"] != "api" || e.Labels["level"] != "info" {
		t.Errorf("entries[0].Labels = %v", e.Labels)
	}
	if !strings.HasPrefix(e.Timestamp, "2025-07-03T10:00:00") {
		t.Errorf("entries[0].Timestamp = %q, want RFC 3339 for ns epoch 1751536800000000000", e.Timestamp)
	}
}

func TestSearchLogsDefaultsToBackwardAndAcceptsUnixSeconds(t *testing.T) {
	srv, requests := fakeLoki(t)
	c := newTestConnector(t, srv.URL)

	res, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "search_logs",
		Args: map[string]any{
			"query": `{app="api"}`,
			"start": "1751530000",
			"end":   "1751540000",
		},
	})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	q := (*requests)[0].Query()
	if got := q.Get("start"); got != "1751530000000000000" {
		t.Errorf("start param = %q, want unix seconds scaled to nanoseconds", got)
	}
	if got := q.Get("direction"); got != "backward" {
		t.Errorf("direction param = %q, want the backward default", got)
	}
	if q.Get("limit") != "" {
		t.Errorf("limit param = %q, want unset when the agent omits it", q.Get("limit"))
	}

	out := decodeSearchResult(t, res)
	// backward = newest first
	wantLines := []string{"request finished", "upstream timeout", "request started"}
	for i, want := range wantLines {
		if out.Entries[i].Line != want {
			t.Errorf("entries[%d].Line = %q, want %q", i, out.Entries[i].Line, want)
		}
	}
}

func TestSearchLogsRejectsInvalidTime(t *testing.T) {
	srv, _ := fakeLoki(t)
	c := newTestConnector(t, srv.URL)

	_, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "search_logs",
		Args: map[string]any{"query": `{app="api"}`, "start": "yesterday", "end": "1751540000"},
	})
	if err == nil {
		t.Fatal("Execute(invalid start) = nil error")
	}
	if !strings.Contains(err.Error(), "start") {
		t.Errorf("error %q does not name the bad argument", err)
	}
}

func TestUnknownToolRejected(t *testing.T) {
	srv, _ := fakeLoki(t)
	c := newTestConnector(t, srv.URL)

	_, err := c.Execute(context.Background(), connector.ToolCall{Tool: "query_metrics"})
	if err == nil {
		t.Fatal("Execute(query_metrics) = nil error, loki must not serve metrics tools")
	}
}

func TestCapabilitiesDeclareLogTools(t *testing.T) {
	c := newTestConnector(t, "http://localhost:3100")

	var names []string
	for _, spec := range c.Capabilities() {
		names = append(names, spec.Name)
	}
	if strings.Join(names, ",") != "search_logs,list_log_labels" {
		t.Errorf("capabilities = %v, want search_logs and list_log_labels", names)
	}
	if c.Type() != "loki" {
		t.Errorf("Type() = %q, want loki", c.Type())
	}
}

func fakeLokiLabels(t *testing.T) (*httptest.Server, *[]*url.URL) {
	t.Helper()
	var requests []*url.URL
	mux := http.NewServeMux()
	mux.HandleFunc("/loki/api/v1/labels", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL)
		w.Write([]byte(`{"status":"success","data":["app","level","namespace"]}`))
	})
	mux.HandleFunc("/loki/api/v1/label/app/values", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL)
		w.Write([]byte(`{"status":"success","data":["api","worker"]}`))
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &requests
}

func TestListLogLabelsMapsToLabelsEndpoint(t *testing.T) {
	srv, requests := fakeLokiLabels(t)
	c := newTestConnector(t, srv.URL)

	res, err := c.Execute(context.Background(), connector.ToolCall{Tool: "list_log_labels"})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	if len(*requests) != 1 || (*requests)[0].Path != "/loki/api/v1/labels" {
		t.Fatalf("requests = %v, want one /loki/api/v1/labels call", *requests)
	}
	b, _ := json.Marshal(res.Content)
	for _, want := range []string{`"labels"`, `"app"`, `"namespace"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("result %s missing %s", b, want)
		}
	}
}

func TestListLogLabelValuesMapsToLabelValuesEndpoint(t *testing.T) {
	srv, requests := fakeLokiLabels(t)
	c := newTestConnector(t, srv.URL)

	res, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "list_log_labels",
		Args: map[string]any{"label": "app", "start": "1751530000", "end": "1751540000"},
	})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	if len(*requests) != 1 || (*requests)[0].Path != "/loki/api/v1/label/app/values" {
		t.Fatalf("requests = %v, want one /loki/api/v1/label/app/values call", *requests)
	}
	q := (*requests)[0].Query()
	if q.Get("start") != "1751530000000000000" || q.Get("end") != "1751540000000000000" {
		t.Errorf("window params = start %q end %q, want nanosecond epochs", q.Get("start"), q.Get("end"))
	}
	b, _ := json.Marshal(res.Content)
	for _, want := range []string{`"label":"app"`, `"worker"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("result %s missing %s", b, want)
		}
	}
}

func TestHealthMapsToReadyEndpoint(t *testing.T) {
	srv, requests := fakeLokiLabels(t)
	c := newTestConnector(t, srv.URL)

	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health = %v", err)
	}
	if len(*requests) != 1 || (*requests)[0].Path != "/ready" {
		t.Errorf("requests = %v, want /ready", *requests)
	}
}

func TestHealthReportsUnreachableBackend(t *testing.T) {
	c := newTestConnector(t, "http://127.0.0.1:1")

	err := c.Health(context.Background())
	if err == nil {
		t.Fatal("Health(unreachable) = nil error")
	}
	if !strings.Contains(err.Error(), "loki-test") {
		t.Errorf("error %q does not name the connector", err)
	}
}

func TestHealthReportsNotReady(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := newTestConnector(t, srv.URL)

	err := c.Health(context.Background())
	if err == nil {
		t.Fatal("Health(503) = nil error")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error %q does not carry the status code", err)
	}
}
