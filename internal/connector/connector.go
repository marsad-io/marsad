// Package connector defines the backend connector interface and the registry
// that routes unified tool calls to the connector serving them.
package connector

import (
	"context"
	"fmt"

	"github.com/marsad-io/marsad/internal/schema"
)

// ToolCall is one unified tool invocation flowing through the gateway.
type ToolCall struct {
	Tool      string
	Connector string // optional explicit target; empty means route by capability
	Args      map[string]any
}

// ToolResult carries a JSON-marshalable payload back to the MCP client.
type ToolResult struct {
	Content any
}

// Connector is implemented once per backend type (Prometheus, Loki, ...).
type Connector interface {
	Name() string
	Type() string
	Health(ctx context.Context) error
	Capabilities() []schema.ToolSpec
	Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}

// Registry holds configured connectors and routes calls to them.
type Registry struct {
	ordered []Connector
	byName  map[string]Connector
	byTool  map[string][]Connector
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		byName: map[string]Connector{},
		byTool: map[string][]Connector{},
	}
}

// Register adds a connector; names must be unique.
func (r *Registry) Register(c Connector) error {
	if _, exists := r.byName[c.Name()]; exists {
		return fmt.Errorf("connector %q already registered", c.Name())
	}
	r.byName[c.Name()] = c
	r.ordered = append(r.ordered, c)
	for _, spec := range c.Capabilities() {
		r.byTool[spec.Name] = append(r.byTool[spec.Name], c)
	}
	return nil
}

// Route resolves the connector that should execute a call: the explicitly
// named one if the call sets Connector, otherwise the single connector serving
// the tool. Ambiguity is an error until fan-out lands.
func (r *Registry) Route(call ToolCall) (Connector, error) {
	if call.Connector != "" {
		c, ok := r.byName[call.Connector]
		if !ok {
			return nil, fmt.Errorf("unknown connector %q", call.Connector)
		}
		for _, spec := range c.Capabilities() {
			if spec.Name == call.Tool {
				return c, nil
			}
		}
		return nil, fmt.Errorf("connector %q does not serve tool %q", call.Connector, call.Tool)
	}

	serving := r.byTool[call.Tool]
	switch len(serving) {
	case 0:
		return nil, fmt.Errorf("no configured connector serves tool %q", call.Tool)
	case 1:
		return serving[0], nil
	default:
		return nil, fmt.Errorf("tool %q is served by %d connectors; pass the connector argument to pick one", call.Tool, len(serving))
	}
}

// All returns connectors in registration order.
func (r *Registry) All() []Connector {
	out := make([]Connector, len(r.ordered))
	copy(out, r.ordered)
	return out
}
