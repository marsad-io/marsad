package guardrails

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/marsad-io/marsad/internal/connector"
)

func okExecutor(res connector.ToolResult, err error) Executor {
	return func(context.Context, connector.ToolCall) (connector.ToolResult, string, error) {
		return res, "prom-a", err
	}
}

func auditLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("audit line %q is not JSON: %v", raw, err)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestReadOnlyToolPassesThrough(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf},
		okExecutor(connector.ToolResult{Content: "data"}, nil))

	res, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	})

	if err != nil {
		t.Fatalf("Execute = %v", err)
	}
	if res.Content != "data" {
		t.Errorf("result = %v", res.Content)
	}
}

func TestUnknownToolRejectedAsNotReadOnly(t *testing.T) {
	var buf bytes.Buffer
	called := false
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf},
		func(context.Context, connector.ToolCall) (connector.ToolResult, string, error) {
			called = true
			return connector.ToolResult{}, "", nil
		})

	_, err := p.Execute(context.Background(), connector.ToolCall{Tool: "delete_everything"})

	if err == nil {
		t.Fatal("Execute(unknown tool) = nil error")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error %q does not state Marsad is read-only", err)
	}
	if called {
		t.Error("executor was called for a rejected tool")
	}
	lines := auditLines(t, &buf)
	if len(lines) != 1 || lines[0]["outcome"] != "rejected" {
		t.Errorf("audit lines = %v, want one rejected line", lines)
	}
}

func TestOversizedTimeRangeRejected(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf},
		okExecutor(connector.ToolResult{}, nil))

	_, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{
			"query": "up",
			"start": "2026-07-03T00:00:00Z",
			"end":   "2026-07-03T02:00:00Z", // 2h > 1h limit
			"step":  "30s",
		},
	})

	if err == nil {
		t.Fatal("Execute(oversized range) = nil error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "1h") || !strings.Contains(msg, "2h") {
		t.Errorf("error %q does not name limit and requested range", err)
	}
}

func TestTimeRangeAcceptsUnixSeconds(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf},
		okExecutor(connector.ToolResult{}, nil))

	_, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{
			"query": "up",
			"start": "1751500000",
			"end":   "1751503000", // 50m < 1h
			"step":  "30s",
		},
	})
	if err != nil {
		t.Errorf("Execute(unix range within limit) = %v", err)
	}
}

func TestEveryCallEmitsOneAuditLine(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf},
		okExecutor(connector.ToolResult{Content: map[string]any{"answer": 42}}, nil))

	_, err := p.Execute(context.Background(), connector.ToolCall{
		Tool:      "query_metrics",
		Connector: "prom-a",
		Args:      map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatal(err)
	}

	lines := auditLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d audit lines, want 1", len(lines))
	}
	line := lines[0]
	for _, field := range []string{"ts", "tool", "connector", "args_hash", "duration_ms", "outcome", "bytes"} {
		if _, ok := line[field]; !ok {
			t.Errorf("audit line missing field %q: %v", field, line)
		}
	}
	if line["outcome"] != "ok" {
		t.Errorf("outcome = %v, want ok", line["outcome"])
	}
	if _, ok := line["args"]; ok {
		t.Error("audit line contains argument values by default")
	}
	if raw := buf.String(); strings.Contains(raw, "up") {
		t.Errorf("audit output leaks argument values: %s", raw)
	}
}

func TestFailedExecutionAuditedWithErrorOutcome(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf},
		okExecutor(connector.ToolResult{}, errors.New("backend exploded")))

	_, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	})
	if err == nil {
		t.Fatal("Execute = nil error, want propagated failure")
	}

	lines := auditLines(t, &buf)
	if len(lines) != 1 || lines[0]["outcome"] != "error" {
		t.Fatalf("audit lines = %v, want one error line", lines)
	}
	if lines[0]["error"] != "backend exploded" {
		t.Errorf("audit error = %v", lines[0]["error"])
	}
}

func TestIncludeArgumentsOptIn(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf, IncludeArguments: true},
		okExecutor(connector.ToolResult{}, nil))

	if _, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	}); err != nil {
		t.Fatal(err)
	}

	lines := auditLines(t, &buf)
	args, ok := lines[0]["args"].(map[string]any)
	if !ok || args["query"] != "up" {
		t.Errorf("audit line does not include opted-in args: %v", lines[0])
	}
}

func TestInvalidArgumentsRejectedBeforeExecution(t *testing.T) {
	var buf bytes.Buffer
	called := false
	p := NewPipeline(Options{MaxTimeRange: time.Hour, AuditSink: &buf},
		func(context.Context, connector.ToolCall) (connector.ToolResult, string, error) {
			called = true
			return connector.ToolResult{}, "", nil
		})

	_, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up", "timeout": "30s"},
	})

	if err == nil {
		t.Fatal("Execute(unknown argument) = nil error")
	}
	if called {
		t.Error("executor ran despite invalid arguments")
	}
	lines := auditLines(t, &buf)
	if len(lines) != 1 || lines[0]["outcome"] != "rejected" {
		t.Errorf("audit lines = %v, want one rejected line", lines)
	}
}

func TestOversizedResultTruncatedWithMarker(t *testing.T) {
	big := strings.Repeat("log line payload ", 100) // well over the budget below
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, MaxResultBytes: 128, AuditSink: &buf},
		okExecutor(connector.ToolResult{Content: map[string]any{"entries": big}}, nil))

	res, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatalf("Execute = %v, truncation must not fail the call", err)
	}

	content, ok := res.Content.(map[string]any)
	if !ok {
		t.Fatalf("truncated content = %T, want map with truncation marker", res.Content)
	}
	if content["truncated"] != true {
		t.Errorf("content[truncated] = %v, want true", content["truncated"])
	}
	total, _ := content["total_bytes"].(int)
	if total <= 128 {
		t.Errorf("content[total_bytes] = %v, want the full pre-truncation size", content["total_bytes"])
	}
	notice, _ := content["notice"].(string)
	if !strings.Contains(notice, "truncated") {
		t.Errorf("content[notice] = %q, want an explicit truncation marker", notice)
	}
	partial, _ := content["partial"].(string)
	if len(partial) == 0 || len(partial) > 128 {
		t.Errorf("partial length = %d, want 1..128 bytes", len(partial))
	}

	lines := auditLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d audit lines, want 1", len(lines))
	}
	line := lines[0]
	auditTotal, _ := line["total_bytes"].(float64)
	if int(auditTotal) != total {
		t.Errorf("audit total_bytes = %v, want %d", line["total_bytes"], total)
	}
	auditBytes, _ := line["bytes"].(float64)
	if auditBytes <= 0 || int(auditBytes) >= total {
		t.Errorf("audit bytes = %v, want truncated size smaller than total %d", line["bytes"], total)
	}
}

func TestResultWithinBudgetPassesUntouched(t *testing.T) {
	var buf bytes.Buffer
	p := NewPipeline(Options{MaxTimeRange: time.Hour, MaxResultBytes: 1 << 20, AuditSink: &buf},
		okExecutor(connector.ToolResult{Content: map[string]any{"entries": "small"}}, nil))

	res, err := p.Execute(context.Background(), connector.ToolCall{
		Tool: "query_metrics",
		Args: map[string]any{"query": "up"},
	})
	if err != nil {
		t.Fatal(err)
	}
	content := res.Content.(map[string]any)
	if _, ok := content["truncated"]; ok {
		t.Error("small result was wrapped in a truncation marker")
	}
	lines := auditLines(t, &buf)
	if _, ok := lines[0]["total_bytes"]; ok {
		t.Error("audit line carries total_bytes for an untruncated result")
	}
}
