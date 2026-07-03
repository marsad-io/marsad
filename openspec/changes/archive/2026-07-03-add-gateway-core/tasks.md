# Tasks: add-gateway-core

Every task follows TDD: write the failing test first, see it fail, implement, see it pass, refactor. Each numbered task is an independently committable chunk.

## 1. Project skeleton

- [x] 1.1 `go mod init github.com/marsad-io/marsad`; add MCP Go SDK dependency (pinned)
- [x] 1.2 Test: `marsad version` prints version string -> implement minimal CLI (cobra or stdlib flag)
- [x] 1.3 CI workflow: `go vet`, `go test ./...` on push/PR

## 2. Unified tool schema

- [x] 2.1 Test: golden files for `query_metrics`, `list_metric_names`, `list_connectors` JSON schemas -> implement `internal/schema` with typed ToolSpec definitions
- [x] 2.2 Test: schema validation rejects unknown arguments -> implement argument validation helper

## 3. Config

- [x] 3.1 Test: load `marsad.yaml` with one prometheus connector into typed Config; missing file and malformed YAML produce clear errors -> implement `internal/config`
- [x] 3.2 Test: `MARSAD_` env vars override file values; secrets only via env -> implement override layer

## 4. Connector interface + Prometheus

- [x] 4.1 Test: fake connector registers capabilities and receives routed calls -> implement `Connector` interface + registry in `internal/connector`
- [x] 4.2 Test (httptest fake Prometheus): `query_metrics` instant and range queries map to `/api/v1/query[_range]` and parse results -> implement `internal/connector/prometheus`
- [x] 4.3 Test: `list_metric_names` maps to `/api/v1/label/__name__/values` -> implement
- [x] 4.4 Test: `Health` maps to `/-/ready`; unreachable backend reports unhealthy without crashing -> implement
- [x] 4.5 Integration test (testcontainers, real Prometheus image): end-to-end query against seeded metrics

## 5. Guardrails baseline

- [x] 5.1 Test: pipeline rejects any tool call not marked read-only -> implement guardrail chain + read-only guardrail
- [x] 5.2 Test: range query exceeding configured max time range is rejected with an explanatory error -> implement time-range cap
- [x] 5.3 Test: every call (success and failure) emits one audit JSON line with required fields; argument values absent by default -> implement audit sink
- [x] 5.4 Test: no outbound HTTP to hosts outside configured backends (assert via custom transport) -> enforce transport allowlist

## 6. MCP server

- [x] 6.1 Test: server registers union of connector capabilities as MCP tools; `list_connectors` returns configured connectors with health -> implement `internal/server`
- [x] 6.2 Test: stdio transport round-trip with an in-process MCP client -> wire `marsad serve --transport stdio`
- [x] 6.3 Test: streamable HTTP transport round-trip -> wire `marsad serve --transport http --listen :8811`
- [x] 6.4 End-to-end smoke test: real MCP client -> stdio server -> testcontainers Prometheus -> result; audit lines verified

## 7. Docs

- [x] 7.1 Quickstart in README: run Prometheus + Marsad via docker compose, connect Claude Code as MCP client, first query
- [x] 7.2 `docs/configuration.md` generated-from-code config reference (test asserts docs cover every config field)
