//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
// real Prometheus container, with audit lines verified on disk.
func TestEndToEndStdioSmoke(t *testing.T) {
	promURL := startPrometheus(t)
	bin := buildMarsad(t)

	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.jsonl")
	cfgPath := filepath.Join(dir, "marsad.yaml")
	cfg := fmt.Sprintf(`
connectors:
  - name: prom-e2e
    type: prometheus
    url: %s
guardrails:
  max_time_range: 24h
audit:
  sink: %s
`, promURL, auditPath)
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
	if len(list.Tools) != 3 {
		t.Errorf("got %d tools, want 3", len(list.Tools))
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

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_connectors"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("list_connectors returned tool error: %+v", res.Content)
	}

	session.Close()

	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("reading audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(audit)), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d audit lines, want 2:\n%s", len(lines), audit)
	}
	for _, line := range lines {
		if !strings.Contains(line, `"outcome":"ok"`) {
			t.Errorf("audit line without ok outcome: %s", line)
		}
	}
}
