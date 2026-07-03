package prometheus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marsad-io/marsad/internal/connector"
)

// fakeProm implements just enough of the Prometheus HTTP API to assert the
// connector sends the right requests and parses real response shapes.
func fakeProm(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var requests []string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"__name__":"up","job":"prometheus"},"value":[1751536800,"1"]}]}}`))
	})
	mux.HandleFunc("/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
			{"metric":{"__name__":"up"},"values":[[1751536800,"1"],[1751536830,"1"]]}]}}`))
	})
	mux.HandleFunc("/api/v1/label/__name__/values", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":["up","go_goroutines"]}`))
	})
	mux.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &requests
}

func newTestConnector(t *testing.T, url string) *Connector {
	t.Helper()
	c, err := New("prom-test", url, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func resultJSON(t *testing.T, res connector.ToolResult) string {
	t.Helper()
	b, err := json.Marshal(res.Content)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInstantQueryMapsToQueryEndpoint(t *testing.T) {
	srv, requests := fakeProm(t)
	c := newTestConnector(t, srv.URL)

	res, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	if len(*requests) != 1 || !strings.HasPrefix((*requests)[0], "/api/v1/query?") {
		t.Fatalf("requests = %v, want one /api/v1/query call", *requests)
	}
	if !strings.Contains((*requests)[0], "query=up") {
		t.Errorf("request %q missing query param", (*requests)[0])
	}
	got := resultJSON(t, res)
	for _, want := range []string{`"resultType":"vector"`, `"__name__":"up"`, "1751536800"} {
		if !strings.Contains(got, want) {
			t.Errorf("result %s missing %s", got, want)
		}
	}
}

func TestRangeQueryMapsToQueryRangeEndpoint(t *testing.T) {
	srv, requests := fakeProm(t)
	c := newTestConnector(t, srv.URL)

	res, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{
			"query": "up",
			"start": "2026-07-03T10:00:00Z",
			"end":   "2026-07-03T11:00:00Z",
			"step":  "30s",
		},
	})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	if len(*requests) != 1 || !strings.HasPrefix((*requests)[0], "/api/v1/query_range?") {
		t.Fatalf("requests = %v, want one /api/v1/query_range call", *requests)
	}
	req := (*requests)[0]
	for _, param := range []string{"query=up", "start=2026-07-03T10%3A00%3A00Z", "end=2026-07-03T11%3A00%3A00Z", "step=30s"} {
		if !strings.Contains(req, param) {
			t.Errorf("request %q missing param %s", req, param)
		}
	}
	if !strings.Contains(resultJSON(t, res), `"resultType":"matrix"`) {
		t.Errorf("result missing matrix payload: %s", resultJSON(t, res))
	}
}

func TestRangeQueryRequiresEndAndStep(t *testing.T) {
	srv, _ := fakeProm(t)
	c := newTestConnector(t, srv.URL)

	_, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up", "start": "2026-07-03T10:00:00Z"},
	})
	if err == nil {
		t.Fatal("Execute(start without end/step) = nil error")
	}
}

func TestListMetricNames(t *testing.T) {
	srv, requests := fakeProm(t)
	c := newTestConnector(t, srv.URL)

	res, err := c.Execute(context.Background(), connector.ToolCall{Tool: "list_metric_names"})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	if len(*requests) != 1 || (*requests)[0] != "/api/v1/label/__name__/values" {
		t.Fatalf("requests = %v, want /api/v1/label/__name__/values", *requests)
	}
	got := resultJSON(t, res)
	if !strings.Contains(got, "go_goroutines") {
		t.Errorf("result %s missing metric names", got)
	}
}

func TestHealthMapsToReadyEndpoint(t *testing.T) {
	srv, requests := fakeProm(t)
	c := newTestConnector(t, srv.URL)

	if err := c.Health(context.Background()); err != nil {
		t.Fatalf("Health = %v", err)
	}
	if len(*requests) != 1 || (*requests)[0] != "/-/ready" {
		t.Errorf("requests = %v, want /-/ready", *requests)
	}
}

func TestHealthUnreachableBackendReturnsErrorWithoutPanic(t *testing.T) {
	c := newTestConnector(t, "http://127.0.0.1:1") // nothing listens here

	if err := c.Health(context.Background()); err == nil {
		t.Error("Health(unreachable) = nil, want error")
	}
	if _, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	}); err == nil {
		t.Error("Execute(unreachable) = nil, want error")
	}
}

func TestBackendErrorStatusSurfacesMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status":"error","errorType":"bad_data","error":"parse error"}`))
	}))
	t.Cleanup(srv.Close)
	c := newTestConnector(t, srv.URL)

	_, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up{"},
	})
	if err == nil {
		t.Fatal("Execute(backend error) = nil error")
	}
	if !strings.Contains(err.Error(), "parse error") {
		t.Errorf("error %q does not surface the backend message", err)
	}
}

func TestBearerTokenSentWhenConfigured(t *testing.T) {
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	t.Cleanup(srv.Close)

	c, err := New("prom-test", srv.URL, "s3cret", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	}); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer s3cret" {
		t.Errorf("Authorization = %q, want Bearer s3cret", auth)
	}
}

func TestCapabilitiesDeclareMetricsTools(t *testing.T) {
	c := newTestConnector(t, "http://localhost:9090")

	var names []string
	for _, spec := range c.Capabilities() {
		names = append(names, spec.Name)
	}
	got := strings.Join(names, ",")
	if got != "query_metrics,list_metric_names" {
		t.Errorf("Capabilities = %s, want query_metrics,list_metric_names", got)
	}
	if c.Type() != "prometheus" {
		t.Errorf("Type = %q", c.Type())
	}
}
