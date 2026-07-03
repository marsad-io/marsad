package connector

import (
	"context"
	"strings"
	"testing"

	"github.com/marsad-io/marsad/internal/schema"
)

type fake struct {
	name     string
	tools    []string
	lastCall ToolCall
}

func (f *fake) Name() string { return f.name }
func (f *fake) Type() string { return "fake" }
func (f *fake) Health(context.Context) error {
	return nil
}
func (f *fake) Capabilities() []schema.ToolSpec {
	var specs []schema.ToolSpec
	for _, tool := range f.tools {
		spec, ok := schema.Lookup(tool)
		if !ok {
			panic("unknown tool in fake: " + tool)
		}
		specs = append(specs, spec)
	}
	return specs
}
func (f *fake) Execute(_ context.Context, call ToolCall) (ToolResult, error) {
	f.lastCall = call
	return ToolResult{Content: "from " + f.name}, nil
}

func TestRegistryRoutesToOnlyServingConnector(t *testing.T) {
	reg := NewRegistry()
	f := &fake{name: "prom-a", tools: []string{"query_metrics", "list_metric_names"}}
	if err := reg.Register(f); err != nil {
		t.Fatal(err)
	}

	call := ToolCall{Tool: "query_metrics", Args: map[string]any{"query": "up"}}
	got, err := reg.Route(call)
	if err != nil {
		t.Fatalf("Route = %v", err)
	}
	if got.Name() != "prom-a" {
		t.Errorf("routed to %q, want prom-a", got.Name())
	}

	result, err := got.Execute(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content != "from prom-a" {
		t.Errorf("result = %v", result.Content)
	}
	if f.lastCall.Tool != "query_metrics" {
		t.Errorf("connector received call %+v", f.lastCall)
	}
}

func TestRegistryRoutesByExplicitConnectorArg(t *testing.T) {
	reg := NewRegistry()
	a := &fake{name: "prom-a", tools: []string{"query_metrics"}}
	b := &fake{name: "prom-b", tools: []string{"query_metrics"}}
	for _, f := range []*fake{a, b} {
		if err := reg.Register(f); err != nil {
			t.Fatal(err)
		}
	}

	got, err := reg.Route(ToolCall{Tool: "query_metrics", Connector: "prom-b"})
	if err != nil {
		t.Fatalf("Route = %v", err)
	}
	if got.Name() != "prom-b" {
		t.Errorf("routed to %q, want prom-b", got.Name())
	}
}

func TestRegistryAmbiguousRouteFailsWithoutConnectorArg(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{"prom-a", "prom-b"} {
		if err := reg.Register(&fake{name: name, tools: []string{"query_metrics"}}); err != nil {
			t.Fatal(err)
		}
	}

	_, err := reg.Route(ToolCall{Tool: "query_metrics"})
	if err == nil {
		t.Fatal("Route(ambiguous) = nil error")
	}
	if !strings.Contains(err.Error(), "connector") {
		t.Errorf("error %q does not tell the caller to pass a connector", err)
	}
}

func TestRegistryUnknownToolAndUnknownConnectorFail(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(&fake{name: "prom-a", tools: []string{"query_metrics"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := reg.Route(ToolCall{Tool: "search_logs"}); err == nil {
		t.Error("Route(unserved tool) = nil error")
	}
	if _, err := reg.Route(ToolCall{Tool: "query_metrics", Connector: "nope"}); err == nil {
		t.Error("Route(unknown connector) = nil error")
	}
}

func TestRegistryRejectsDuplicateNames(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(&fake{name: "prom-a", tools: []string{"query_metrics"}}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register(&fake{name: "prom-a", tools: []string{"query_metrics"}}); err == nil {
		t.Error("Register(duplicate) = nil error")
	}
}

func TestRegistryAllReturnsRegistrationOrder(t *testing.T) {
	reg := NewRegistry()
	for _, name := range []string{"b", "a", "c"} {
		if err := reg.Register(&fake{name: name, tools: []string{"query_metrics"}}); err != nil {
			t.Fatal(err)
		}
	}
	var names []string
	for _, c := range reg.All() {
		names = append(names, c.Name())
	}
	if strings.Join(names, ",") != "b,a,c" {
		t.Errorf("All() order = %v, want registration order", names)
	}
}
