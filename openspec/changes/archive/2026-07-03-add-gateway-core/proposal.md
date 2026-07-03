# Change: Add gateway core (MCP server, config, Prometheus connector, guardrails baseline)

## Why

Marsad needs a walking skeleton that proves the whole value proposition end to end: an MCP server an agent can connect to, a real backend behind a unified tool schema, and the guardrails that differentiate Marsad from every vendor MCP server. Prometheus is the first connector because it is the most widely deployed self-hosted backend and exercises the full connector interface.

## What Changes

- New Go module `github.com/marsad-io/marsad` with CLI entrypoint `marsad serve`
- MCP server supporting stdio and streamable HTTP transports, built on the official MCP Go SDK
- YAML configuration (single file + env var overrides) defining connectors and guardrail settings
- Connector interface (`Query`, `Health`, `Capabilities`) and its first implementation: Prometheus (instant + range queries, metric name listing; Thanos/Mimir compatible)
- Unified tools registered from connector capabilities: `query_metrics`, `list_metric_names`, `list_connectors`
- Guardrails baseline: read-only enforcement, per-call audit log (JSON lines), configurable max time range per query, zero outbound calls other than configured backends

## Impact

- Affected specs: `mcp-server`, `config`, `connector-prometheus`, `guardrails` (all new)
- Affected code: entire initial codebase (`cmd/marsad`, `internal/server`, `internal/config`, `internal/connector`, `internal/connector/prometheus`, `internal/guardrails`)
- New CI: GitHub Actions running unit + integration tests (testcontainers with a Prometheus image)

## Non-goals

- Loki, Elasticsearch/OpenSearch, ClickHouse, Alertmanager, Kubernetes connectors (each is its own follow-up change)
- PII redaction, rate limiting, per-connector RBAC (follow-up guardrails changes)
- Helm chart, goreleaser packaging, container images (follow-up release-engineering change)
- Any agent/investigation logic - Marsad phase 2 lives in a separate project cycle
