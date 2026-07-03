# connector-alertmanager Specification (delta)

## ADDED Requirements

### Requirement: Unified alert tools over the Alertmanager v2 API

The Alertmanager connector SHALL serve `list_alerts` and `list_silences` by mapping unified arguments to the Alertmanager v2 API (`/api/v2/alerts`, `/api/v2/silences`), returning backend-agnostic shapes (alert: labels, annotations, state, starts_at, ends_at, generator_url; silence: id, matchers, comment, created_by, starts_at, ends_at, status).

#### Scenario: List firing alerts

- **WHEN** an agent calls `list_alerts` with no filters
- **THEN** the connector returns active alerts from `/api/v2/alerts` with labels, annotations, state, and timing fields

#### Scenario: Filter by label and state

- **WHEN** an agent calls `list_alerts` with label matchers and a state filter (e.g. silenced)
- **THEN** matchers map to the `filter` query parameter and state flags map to the corresponding query parameters, returning only matching alerts

#### Scenario: List silences

- **WHEN** an agent calls `list_silences`
- **THEN** the connector returns silences from `/api/v2/silences` including matchers, comment, creator, and expiry

#### Scenario: Unreachable backend

- **WHEN** the Alertmanager URL does not respond
- **THEN** `Health` returns an error, `list_connectors` shows the connector unhealthy, and other connectors keep serving

### Requirement: Read-only alert surface

The connector SHALL expose no tool that creates, modifies, or expires alerts or silences.

#### Scenario: No write tools registered

- **WHEN** the Alertmanager connector registers its capabilities
- **THEN** every registered tool is marked read-only and no silence-creation or alert-mutation tool exists
