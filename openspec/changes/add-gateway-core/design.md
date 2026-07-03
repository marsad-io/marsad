# Design: gateway core

## Context

Marsad's differentiation is not "an MCP server for X" (Grafana and every vendor ship those) but a neutral gateway: many backends, one tool schema, guardrails as first-class citizens. The core must make adding a connector cheap and make guardrails unavoidable - every tool call flows through the same pipeline.

## Goals / Non-Goals

Goals: walking skeleton with real value (Prometheus), connector interface stable enough for five more connectors, guardrails in the request path from day one.
Non-goals: multi-tenancy, write operations, agent logic.

## Decisions

### 1. Request pipeline

Every MCP tool call passes through: transport -> guardrail chain (read-only check, time-range cap, future: rate limit, redaction) -> connector dispatch -> audit log (always, success or failure). Guardrails are middleware over a single `ToolCall` type, so new guardrails are one function each.

### 2. Connector interface

```go
type Connector interface {
    Name() string
    Health(ctx context.Context) error
    Capabilities() []ToolSpec          // which unified tools this connector serves
    Execute(ctx context.Context, call ToolCall) (ToolResult, error)
}
```

Connectors declare capabilities; the server registers the union as MCP tools and routes calls by tool name + optional `connector` argument. Two connectors serving `search_logs` coexist; the agent picks or Marsad fans out (fan-out is a later change).

### 3. Unified tool schema

Tool names and JSON schemas live in `internal/schema` as the single source of truth, with golden-file tests (`testdata/tools/*.json`). Connectors map unified arguments to backend queries; they never invent tool names. This is the contract that makes Marsad "one schema, any backend".

### 4. Config

One YAML file (`marsad.yaml`), env var overrides (`MARSAD_` prefix). Secrets referenced via env only, never inline. Config loading is pure (no I/O beyond reading the file) for testability.

### 5. Audit log

JSON lines to a configurable sink (default stderr): timestamp, tool, connector, arguments hash, duration, outcome, bytes returned. No argument values by default (privacy), full values opt-in. This ships in the skeleton because retrofitting auditing never happens.

## Risks / Trade-offs

- Official MCP Go SDK is young; pin the version, wrap it behind `internal/server` so a SDK change touches one package.
- Unified schema may leak lowest-common-denominator semantics; mitigation: `raw_query` escape hatch per connector (still guardrailed, still audited).

## Open Questions

- None blocking. Fan-out semantics deferred to the multi-connector change.
