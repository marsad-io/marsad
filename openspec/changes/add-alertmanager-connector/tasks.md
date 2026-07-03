# Tasks: add-alertmanager-connector

TDD throughout: failing test first, then implement. Each numbered task is independently committable.

## 1. Unified alert tool schema

- [ ] 1.1 Test: golden files for `list_alerts` (label matchers, state filter, optional connector) and `list_silences`; all backend-neutral -> implement ToolSpecs in `internal/schema`

## 2. Alertmanager connector

- [ ] 2.1 Test (httptest fake Alertmanager): `list_alerts` maps matchers/state to `/api/v2/alerts` query parameters and parses the v2 shape -> implement `internal/connector/alertmanager`
- [ ] 2.2 Test: `list_silences` maps to `/api/v2/silences` -> implement
- [ ] 2.3 Test: `Health` via `/-/ready`; unreachable backend unhealthy without affecting others; only read-only tools registered -> implement
- [ ] 2.4 Test: bearer/basic auth via env references (shared plumbing); config accepts alertmanager type -> wire config
- [ ] 2.5 Integration test (testcontainers, real Alertmanager image): post an alert via API, list it end to end

## 3. Triad wiring

- [ ] 3.1 Test: prometheus + loki + alertmanager config registers full tool union; `list_connectors` shows all three -> extend server tests
- [ ] 3.2 E2e smoke extension: alert visible, metrics queried, logs searched in one MCP session; audit verified

## 4. Docs

- [ ] 4.1 README table + quickstart gain Alertmanager; `docs/configuration.md` covers the connector type (docs-coverage test extended)
