//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marsad-io/marsad/internal/connector"
	"github.com/marsad-io/marsad/internal/connector/loki"
)

// buildMarsad compiles the real binary the way an operator would run it.
func buildMarsad(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "marsad")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/marsad-io/marsad/cmd/marsad")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd)) // test/integration -> repo root
}

// End-to-end smoke test: real MCP client -> marsad binary over stdio ->
// real Prometheus and Loki containers, metric query and log search in one
// session, with audit lines verified on disk.
func TestEndToEndStdioSmoke(t *testing.T) {
	promURL := startPrometheus(t)
	lokiURL := startLoki(t)
	bin := buildMarsad(t)

	// Seed Loki and wait until the entries are searchable, so the single
	// search_logs call through the session is deterministic.
	seedLokiLogs(t, lokiURL,
		map[string]string{"app": "marsad-e2e"},
		[]string{"e2e log line one", "e2e log line two"})
	waitForLokiEntries(t, lokiURL, `{app="marsad-e2e"}`, 2)

	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.jsonl")
	cfgPath := filepath.Join(dir, "marsad.yaml")
	cfg := fmt.Sprintf(`
connectors:
  - name: prom-e2e
    type: prometheus
    url: %s
  - name: loki-e2e
    type: loki
    url: %s
guardrails:
  max_time_range: 24h
audit:
  sink: %s
`, promURL, lokiURL, auditPath)
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := mcp.NewClient(&mcp.Implementation{Name: "e2e-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{
		Command: exec.Command(bin, "serve", "--config", cfgPath, "--transport", "stdio"),
	}, nil)
	if err != nil {
		t.Fatalf("connecting over stdio: %v", err)
	}
	defer session.Close()

	list, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != 5 {
		t.Errorf("got %d tools, want the union of metric, log, and built-in tools (5)", len(list.Tools))
	}

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "query_metrics",
		Arguments: map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("query_metrics returned tool error: %+v", res.Content)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_logs",
		Arguments: map[string]any{
			"query": `{app="marsad-e2e"}`,
			"start": time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
			"end":   time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("search_logs returned tool error: %+v", res.Content)
	}
	if text := textContent(t, res); !strings.Contains(text, "e2e log line one") {
		t.Errorf("search_logs result %s missing seeded line", text)
	}

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_connectors"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("list_connectors returned tool error: %+v", res.Content)
	}
	if text := textContent(t, res); !strings.Contains(text, `"loki-e2e"`) || !strings.Contains(text, `"prom-e2e"`) {
		t.Errorf("list_connectors result %s missing a connector", text)
	}

	session.Close()

	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("reading audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(audit)), "\n")
	if len(lines) != 3 {
		t.Errorf("got %d audit lines, want 3:\n%s", len(lines), audit)
	}
	for _, line := range lines {
		if !strings.Contains(line, `"outcome":"ok"`) {
			t.Errorf("audit line without ok outcome: %s", line)
		}
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, `"connector":"prom-e2e"`) || !strings.Contains(joined, `"connector":"loki-e2e"`) {
		t.Errorf("audit lines do not attribute calls to both connectors:\n%s", joined)
	}
}

// textContent returns the first text payload of a tool result.
func textContent(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("result has no text content: %+v", res)
	return ""
}

// waitForLokiEntries polls the query API until the selector returns want
// entries, so later assertions do not race ingestion.
func waitForLokiEntries(t *testing.T, baseURL, selector string, want int) {
	t.Helper()
	c, err := loki.New("loki-e2e-wait", baseURL, loki.Auth{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	args := map[string]any{
		"query": selector,
		"start": time.Now().Add(-5 * time.Minute).UTC().Format(time.RFC3339),
		"end":   time.Now().Add(5 * time.Minute).UTC().Format(time.RFC3339),
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		res, err := c.Execute(context.Background(), connector.ToolCall{Tool: "search_logs", Args: args})
		if err != nil {
			t.Fatalf("polling loki: %v", err)
		}
		b, err := json.Marshal(res.Content)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), fmt.Sprintf(`"count":%d`, want)) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("seeded entries not searchable after 30s; last result: %s", b)
		}
		time.Sleep(2 * time.Second)
	}
}
