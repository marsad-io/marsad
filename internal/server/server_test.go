package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marsad-io/marsad/internal/config"
)

func fakeProm(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
			{"metric":{"__name__":"up"},"value":[1751536800,"1"]}]}}`))
	})
	mux.HandleFunc("/api/v1/label/__name__/values", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"success","data":["up"]}`))
	})
	mux.HandleFunc("/-/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testConfig(promURL string) config.Config {
	return config.Config{
		Connectors: []config.ConnectorConfig{
			{Name: "prom-a", Type: "prometheus", URL: promURL},
		},
		Guardrails: config.GuardrailsConfig{MaxTimeRange: time.Hour},
		Audit:      config.AuditConfig{Sink: "stderr"},
	}
}

// connect builds the gateway server and an in-process MCP client session.
func connect(t *testing.T, cfg config.Config, audit *bytes.Buffer) *mcp.ClientSession {
	t.Helper()
	s, err := New(cfg, "test", WithAuditWriter(audit))
	if err != nil {
		t.Fatal(err)
	}

	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := s.Connect(ctx, serverT); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("result has no text content: %+v", res)
	return ""
}

func TestToolsReflectConfiguration(t *testing.T) {
	session := connect(t, testConfig(fakeProm(t).URL), &bytes.Buffer{})

	list, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	want := "list_connectors,list_metric_names,query_metrics"
	if got := strings.Join(names, ","); got != want {
		t.Errorf("tools = %s, want %s", got, want)
	}

	for _, tool := range list.Tools {
		schema, ok := tool.InputSchema.(map[string]any)
		if !ok || schema["type"] != "object" {
			t.Errorf("tool %s inputSchema = %v, want object schema", tool.Name, tool.InputSchema)
		}
	}
}

func TestListConnectorsReportsNameTypeHealth(t *testing.T) {
	session := connect(t, testConfig(fakeProm(t).URL), &bytes.Buffer{})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_connectors"})
	if err != nil {
		t.Fatal(err)
	}

	var payload struct {
		Connectors []struct {
			Name    string `json:"name"`
			Type    string `json:"type"`
			Healthy bool   `json:"healthy"`
		} `json:"connectors"`
	}
	if err := json.Unmarshal([]byte(toolText(t, res)), &payload); err != nil {
		t.Fatalf("list_connectors payload is not JSON: %v", err)
	}
	if len(payload.Connectors) != 1 {
		t.Fatalf("got %d connectors", len(payload.Connectors))
	}
	c := payload.Connectors[0]
	if c.Name != "prom-a" || c.Type != "prometheus" || !c.Healthy {
		t.Errorf("connector = %+v", c)
	}
}

func TestUnreachableBackendShowsUnhealthyServerKeepsServing(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1")
	session := connect(t, cfg, &bytes.Buffer{})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_connectors"})
	if err != nil {
		t.Fatalf("server failed to serve with unreachable backend: %v", err)
	}
	text := toolText(t, res)
	if !strings.Contains(text, `"healthy":false`) {
		t.Errorf("list_connectors = %s, want healthy:false", text)
	}
}

func TestQueryMetricsFlowsThroughPipelineAndAudits(t *testing.T) {
	var audit bytes.Buffer
	session := connect(t, testConfig(fakeProm(t).URL), &audit)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "query_metrics",
		Arguments: map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(t, res))
	}
	if !strings.Contains(toolText(t, res), `"resultType":"vector"`) {
		t.Errorf("result = %s", toolText(t, res))
	}
	if !strings.Contains(audit.String(), `"outcome":"ok"`) {
		t.Errorf("audit log missing ok line: %s", audit.String())
	}
}

func TestUnknownArgumentRejectedAndAudited(t *testing.T) {
	var audit bytes.Buffer
	session := connect(t, testConfig(fakeProm(t).URL), &audit)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "query_metrics",
		Arguments: map[string]any{"query": "up", "timeout": "30s"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("call with unknown argument did not return a tool error")
	}
	if !strings.Contains(toolText(t, res), "timeout") {
		t.Errorf("error text %q does not name the bad argument", toolText(t, res))
	}
	if !strings.Contains(audit.String(), `"outcome":"rejected"`) {
		t.Errorf("audit log missing rejected line: %s", audit.String())
	}
}

func TestExplicitConnectorArgumentRoutes(t *testing.T) {
	session := connect(t, testConfig(fakeProm(t).URL), &bytes.Buffer{})

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "query_metrics",
		Arguments: map[string]any{"query": "up", "connector": "prom-a"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(t, res))
	}
}

func TestHTTPTransportRoundTrip(t *testing.T) {
	s, err := New(testConfig(fakeProm(t).URL), "test", WithAuditWriter(&bytes.Buffer{}))
	if err != nil {
		t.Fatal(err)
	}
	httpSrv := httptest.NewServer(s.HTTPHandler())
	t.Cleanup(httpSrv.Close)

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(context.Background(),
		&mcp.StreamableClientTransport{Endpoint: httpSrv.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "query_metrics",
		Arguments: map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", toolText(t, res))
	}
	if !strings.Contains(toolText(t, res), `"resultType":"vector"`) {
		t.Errorf("result = %s", toolText(t, res))
	}
}

func TestUnsupportedConnectorTypeFailsAtStartup(t *testing.T) {
	cfg := testConfig("http://localhost:9090")
	cfg.Connectors[0].Type = "influxdb"

	if _, err := New(cfg, "test"); err == nil {
		t.Error("New(unsupported connector type) = nil error")
	}
}
