# Tasks: add-loki-connector

TDD throughout: failing test first, then implement. Each numbered task is independently committable.

## 1. Unified log tool schema

- [x] 1.1 Test: golden files for `search_logs` and `list_log_labels`; required arguments are backend-neutral -> implement ToolSpecs in `internal/schema`
- [x] 1.2 Test: argument validation for time range and limit bounds -> extend validation helper

## 2. Result size budget guardrail

- [x] 2.1 Test: result exceeding configured byte budget is truncated with marker and total count; audit line records truncated + total sizes -> implement size-budget guardrail in the pipeline
- [x] 2.2 Test: budget configurable via `marsad.yaml` and `MARSAD_` env override, with a sane default -> wire config

## 3. Loki connector

- [ ] 3.1 Test (httptest fake Loki): `search_logs` maps selector/time range/limit/direction to `/loki/api/v1/query_range` and parses streams into the neutral result shape -> implement `internal/connector/loki`
- [x] 3.2 Test: `list_log_labels` maps to `/loki/api/v1/labels` and `/loki/api/v1/label/{name}/values` -> implement
- [x] 3.3 Test: `Health` via `/ready`; unreachable Loki reports unhealthy without affecting other connectors -> implement
- [ ] 3.4 Test: bearer/basic auth via env references only; inline secrets rejected by config -> implement auth plumbing
- [ ] 3.5 Integration test (testcontainers, real Loki image): seed logs, search end to end

## 4. Two-connector wiring

- [ ] 4.1 Test: config with prometheus + loki registers the tool union; `list_connectors` shows both with health -> extend server tests
- [ ] 4.2 E2e smoke: MCP client -> stdio server -> Prometheus + Loki containers -> metric query and log search in one session; audit verified

## 5. Docs

- [ ] 5.1 Quickstart gains Loki (docker compose + config); README connector table updated
- [ ] 5.2 `docs/configuration.md` covers loki connector type and size budget (docs-coverage test extended)
