# connector-loki Specification

## Purpose
TBD - created by archiving change add-loki-connector. Update Purpose after archive.
## Requirements
### Requirement: Unified log tools over the Loki HTTP API

The Loki connector SHALL serve `search_logs` and `list_log_labels` by mapping unified arguments to the Loki HTTP API (`/loki/api/v1/query_range`, `/loki/api/v1/labels`, `/loki/api/v1/label/{name}/values`), returning logs in the backend-agnostic result shape (timestamp, labels, line).

#### Scenario: Log search by selector

- **WHEN** an agent calls `search_logs` with a label selector, time range, and limit
- **THEN** the connector issues `/loki/api/v1/query_range` and returns matching entries ordered per the requested direction, each with timestamp, labels, and line

#### Scenario: Label discovery

- **WHEN** an agent calls `list_log_labels`
- **THEN** the connector returns label names from `/loki/api/v1/labels`, and values for a specific label when one is named

#### Scenario: Unreachable backend

- **WHEN** the Loki URL does not respond
- **THEN** `Health` returns an error, `list_connectors` shows the connector unhealthy, and other connectors keep serving

### Requirement: Backend-agnostic search_logs contract

The `search_logs` tool schema SHALL contain no Loki-specific concepts in its required arguments, so future log backends (Elasticsearch/OpenSearch, ClickHouse) can implement the identical contract.

#### Scenario: Neutral required arguments

- **WHEN** the `search_logs` golden-file schema is inspected
- **THEN** required arguments are limited to backend-neutral fields (query/selector, time range) and any LogQL-specific capability is an optional argument

