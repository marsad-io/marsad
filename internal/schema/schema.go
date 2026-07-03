// Package schema is the single source of truth for Marsad's unified tool
// schema: the tool names and JSON schemas every connector maps to. Golden
// files in testdata/tools pin the public contract.
package schema

// ToolSpec describes one unified tool exposed over MCP.
type ToolSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	ReadOnly    bool        `json:"readOnly"`
	Input       InputSchema `json:"inputSchema"`
}

// InputSchema is the JSON Schema (draft 2020-12 subset) for a tool's arguments.
type InputSchema struct {
	Type                 string              `json:"type"`
	Properties           map[string]Property `json:"properties"`
	Required             []string            `json:"required,omitempty"`
	AdditionalProperties bool                `json:"additionalProperties"`
}

// Property is a single argument definition.
type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

const connectorArgDescription = "Name of the configured connector to query. Optional when exactly one connector serves this tool."

var all = []ToolSpec{
	{
		Name:        "query_metrics",
		Description: "Run a metrics query (instant or range) against a configured metrics backend such as Prometheus, Thanos, or Mimir.",
		ReadOnly:    true,
		Input: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"query":     {Type: "string", Description: "Query expression in the backend's language, e.g. PromQL."},
				"start":     {Type: "string", Description: "Range start as RFC 3339 timestamp or Unix seconds. Omit for an instant query."},
				"end":       {Type: "string", Description: "Range end as RFC 3339 timestamp or Unix seconds. Requires start and step."},
				"step":      {Type: "string", Description: "Range resolution as a duration string, e.g. 30s or 5m."},
				"connector": {Type: "string", Description: connectorArgDescription},
			},
			Required: []string{"query"},
		},
	},
	{
		Name:        "list_metric_names",
		Description: "List metric names known to a configured metrics backend.",
		ReadOnly:    true,
		Input: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"connector": {Type: "string", Description: connectorArgDescription},
			},
		},
	},
	{
		Name:        "list_connectors",
		Description: "List the configured connectors with their type and current health status.",
		ReadOnly:    true,
		Input: InputSchema{
			Type:       "object",
			Properties: map[string]Property{},
		},
	},
}

// All returns every unified tool spec in a stable order.
func All() []ToolSpec {
	out := make([]ToolSpec, len(all))
	copy(out, all)
	return out
}

// Lookup returns the spec for a tool name.
func Lookup(name string) (ToolSpec, bool) {
	for _, s := range all {
		if s.Name == name {
			return s, true
		}
	}
	return ToolSpec{}, false
}
