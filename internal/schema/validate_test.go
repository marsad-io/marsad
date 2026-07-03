package schema

import (
	"strings"
	"testing"
)

func mustLookup(t *testing.T, name string) ToolSpec {
	t.Helper()
	spec, ok := Lookup(name)
	if !ok {
		t.Fatalf("Lookup(%s) not found", name)
	}
	return spec
}

func TestValidateAcceptsKnownArguments(t *testing.T) {
	spec := mustLookup(t, "query_metrics")

	err := Validate(spec, map[string]any{
		"query": "up",
		"start": "2026-07-03T10:00:00Z",
		"end":   "2026-07-03T11:00:00Z",
		"step":  "30s",
	})

	if err != nil {
		t.Errorf("Validate(valid args) = %v, want nil", err)
	}
}

func TestValidateRejectsUnknownArgument(t *testing.T) {
	spec := mustLookup(t, "query_metrics")

	err := Validate(spec, map[string]any{
		"query":   "up",
		"timeout": "30s",
	})

	if err == nil {
		t.Fatal("Validate(unknown arg) = nil, want error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error %q does not name the unknown argument", err)
	}
}

func TestValidateRejectsMissingRequiredArgument(t *testing.T) {
	spec := mustLookup(t, "query_metrics")

	err := Validate(spec, map[string]any{"step": "30s"})

	if err == nil {
		t.Fatal("Validate(missing required) = nil, want error")
	}
	if !strings.Contains(err.Error(), "query") {
		t.Errorf("error %q does not name the missing argument", err)
	}
}

func TestValidateRejectsWrongType(t *testing.T) {
	spec := mustLookup(t, "query_metrics")

	err := Validate(spec, map[string]any{"query": 42})

	if err == nil {
		t.Fatal("Validate(wrong type) = nil, want error")
	}
	if !strings.Contains(err.Error(), "query") || !strings.Contains(err.Error(), "string") {
		t.Errorf("error %q does not explain the type mismatch", err)
	}
}

func validLogArgs(over map[string]any) map[string]any {
	args := map[string]any{
		"query": `{app="api"}`,
		"start": "2026-07-03T10:00:00Z",
		"end":   "2026-07-03T11:00:00Z",
	}
	for k, v := range over {
		args[k] = v
	}
	return args
}

func TestValidateAcceptsSearchLogsArguments(t *testing.T) {
	spec := mustLookup(t, "search_logs")

	err := Validate(spec, validLogArgs(map[string]any{
		"limit":     float64(100),
		"direction": "forward",
	}))
	if err != nil {
		t.Errorf("Validate(valid search_logs args) = %v, want nil", err)
	}
}

func TestValidateRejectsLimitOutOfBounds(t *testing.T) {
	spec := mustLookup(t, "search_logs")

	for _, limit := range []float64{0, -5, 5001} {
		err := Validate(spec, validLogArgs(map[string]any{"limit": limit}))
		if err == nil {
			t.Errorf("Validate(limit=%v) = nil, want bounds error", limit)
			continue
		}
		if !strings.Contains(err.Error(), "limit") {
			t.Errorf("error %q does not name the limit argument", err)
		}
	}
}

func TestValidateRejectsUnknownDirection(t *testing.T) {
	spec := mustLookup(t, "search_logs")

	err := Validate(spec, validLogArgs(map[string]any{"direction": "sideways"}))
	if err == nil {
		t.Fatal("Validate(direction=sideways) = nil, want enum error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "direction") || !strings.Contains(msg, "forward") {
		t.Errorf("error %q does not explain the allowed directions", err)
	}
}

func TestValidateRequiresTimeRangeForSearchLogs(t *testing.T) {
	spec := mustLookup(t, "search_logs")

	err := Validate(spec, map[string]any{"query": `{app="api"}`})
	if err == nil {
		t.Fatal("Validate(search_logs without time range) = nil, want error")
	}
	if !strings.Contains(err.Error(), "start") {
		t.Errorf("error %q does not name the missing argument", err)
	}
}
