// Package server wires config, connectors, guardrails, and the MCP SDK into
// the running gateway. It is the only package that imports the SDK, so an SDK
// change touches exactly one place.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/marsad-io/marsad/internal/config"
	"github.com/marsad-io/marsad/internal/connector"
	"github.com/marsad-io/marsad/internal/connector/prometheus"
	"github.com/marsad-io/marsad/internal/guardrails"
	"github.com/marsad-io/marsad/internal/schema"
)

// Server is the assembled gateway.
type Server struct {
	mcpServer *mcp.Server
	registry  *connector.Registry
	pipeline  *guardrails.Pipeline
}

// Option customizes construction, mainly for tests.
type Option func(*options)

type options struct {
	auditWriter io.Writer
}

// WithAuditWriter overrides the audit sink from config with a writer.
func WithAuditWriter(w io.Writer) Option {
	return func(o *options) { o.auditWriter = w }
}

// New builds the gateway from config: connectors behind transport-restricted
// HTTP clients, the guardrail pipeline, and MCP tool registration.
func New(cfg config.Config, version string, opts ...Option) (*Server, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	registry := connector.NewRegistry()
	for _, cc := range cfg.Connectors {
		c, err := buildConnector(cc)
		if err != nil {
			return nil, err
		}
		if err := registry.Register(c); err != nil {
			return nil, err
		}
	}

	auditWriter, err := resolveAuditSink(cfg.Audit.Sink, o.auditWriter)
	if err != nil {
		return nil, err
	}

	s := &Server{registry: registry}
	s.pipeline = guardrails.NewPipeline(guardrails.Options{
		MaxTimeRange:     cfg.Guardrails.MaxTimeRange,
		MaxResultBytes:   cfg.Guardrails.MaxResultBytes,
		AuditSink:        auditWriter,
		IncludeArguments: cfg.Audit.IncludeArguments,
	}, s.execute)

	s.mcpServer = mcp.NewServer(&mcp.Implementation{Name: "marsad", Version: version}, nil)
	if err := s.registerTools(); err != nil {
		return nil, err
	}
	return s, nil
}

func buildConnector(cc config.ConnectorConfig) (connector.Connector, error) {
	client := guardrails.RestrictedClient([]string{cc.URL})
	switch cc.Type {
	case "prometheus":
		return prometheus.New(cc.Name, cc.URL, cc.BearerToken, client)
	default:
		return nil, fmt.Errorf("connector %q: unsupported type %q", cc.Name, cc.Type)
	}
}

func resolveAuditSink(sink string, override io.Writer) (io.Writer, error) {
	if override != nil {
		return override, nil
	}
	if sink == "" || sink == "stderr" {
		return os.Stderr, nil
	}
	f, err := os.OpenFile(sink, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit sink %s: %w", sink, err)
	}
	return f, nil
}

// registerTools exposes the union of connector capabilities plus the built-in
// list_connectors tool, all defined by the unified schema.
func (s *Server) registerTools() error {
	tools := map[string]schema.ToolSpec{}
	for _, c := range s.registry.All() {
		for _, spec := range c.Capabilities() {
			tools[spec.Name] = spec
		}
	}
	if spec, ok := schema.Lookup("list_connectors"); ok {
		tools[spec.Name] = spec
	}

	for _, spec := range tools {
		inputSchema, err := json.Marshal(spec.Input)
		if err != nil {
			return err
		}
		s.mcpServer.AddTool(&mcp.Tool{
			Name:        spec.Name,
			Description: spec.Description,
			InputSchema: json.RawMessage(inputSchema),
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: spec.ReadOnly},
		}, s.handleTool)
	}
	return nil
}

// handleTool converts an MCP call into a pipeline execution.
func (s *Server) handleTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := map[string]any{}
	if len(req.Params.Arguments) > 0 {
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return toolError(fmt.Errorf("arguments are not a JSON object: %w", err)), nil
		}
	}

	call := connector.ToolCall{Tool: req.Params.Name, Args: args}
	if name, ok := args["connector"].(string); ok {
		call.Connector = name
	}

	res, err := s.pipeline.Execute(ctx, call)
	if err != nil {
		return toolError(err), nil
	}

	payload, err := json.Marshal(res.Content)
	if err != nil {
		return toolError(fmt.Errorf("marshaling result: %w", err)), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
	}, nil
}

// toolError reports a failure to the MCP client as a tool-level error rather
// than a protocol error, so agents can read and react to the message.
func toolError(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
	}
}

// execute is the pipeline's executor: route and run. Validation already
// happened in the guardrail chain.
func (s *Server) execute(ctx context.Context, call connector.ToolCall) (connector.ToolResult, string, error) {
	if call.Tool == "list_connectors" {
		return s.listConnectors(ctx), "", nil
	}

	target, err := s.registry.Route(call)
	if err != nil {
		return connector.ToolResult{}, "", err
	}
	res, err := target.Execute(ctx, call)
	return res, target.Name(), err
}

func (s *Server) listConnectors(ctx context.Context) connector.ToolResult {
	type entry struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Healthy bool   `json:"healthy"`
		Error   string `json:"error,omitempty"`
	}
	var entries []entry
	for _, c := range s.registry.All() {
		e := entry{Name: c.Name(), Type: c.Type(), Healthy: true}
		if err := c.Health(ctx); err != nil {
			e.Healthy = false
			e.Error = err.Error()
		}
		entries = append(entries, e)
	}
	return connector.ToolResult{Content: map[string]any{"connectors": entries}}
}

// Connect attaches the server to a transport; used by tests and RunStdio.
func (s *Server) Connect(ctx context.Context, t mcp.Transport) (*mcp.ServerSession, error) {
	return s.mcpServer.Connect(ctx, t, nil)
}

// RunStdio serves MCP over stdio until the context ends.
func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// HTTPHandler serves MCP over streamable HTTP.
func (s *Server) HTTPHandler() http.Handler {
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s.mcpServer }, nil)
}
