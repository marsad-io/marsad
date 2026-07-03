# Change: Add Loki connector and unified log-search tools

## Why

Metrics without logs answers half an investigation. Loki is the most common self-hosted log store next to Prometheus (same operators, same Grafana stack), so it is the highest-leverage second connector and it forces the unified schema to prove its core promise: one `search_logs` tool regardless of backend, so Elasticsearch/OpenSearch and ClickHouse can implement the same contract next.

## What Changes

- New unified tools in `internal/schema` with golden files: `search_logs` (LogQL or label-filter based selection, time range, limit, direction) and `list_log_labels` (label names and values for discovery)
- New connector `internal/connector/loki`: maps unified arguments to the Loki HTTP API (`/loki/api/v1/query_range`, `/loki/api/v1/labels`, `/loki/api/v1/label/{name}/values`), parses streams into a backend-agnostic log result shape, `Health` via `/ready`, optional bearer/basic auth via env references
- Guardrails apply unchanged: time-range cap on log queries, result size participates in the audit line, Loki host joins the outbound allowlist
- New `max_result_bytes` guardrail: log payloads dwarf metric payloads, so responses over a configurable byte budget are truncated with an explicit truncation marker in the tool result
- Integration test (testcontainers, real Loki image) and e2e coverage extended to a two-connector configuration proving tool union and routing

## Impact

- Affected specs: `connector-loki` (new), `guardrails` (delta: result-size budget)
- Affected code: `internal/schema` (new tool specs), `internal/connector/loki` (new), `internal/config` (loki connector type), `internal/guardrails` (size budget), docs (configuration reference, README quickstart gains Loki)

## Non-goals

- Log tail/streaming (follow-up; MCP result model fits request/response first)
- Elasticsearch/OpenSearch and ClickHouse log backends (next changes; they implement the `search_logs` contract this change defines)
- Cross-connector fan-out of `search_logs` (deferred until two log backends exist)
- LogQL metric queries (`rate(...)` over logs) - `query_metrics` stays Prometheus-family territory for now
