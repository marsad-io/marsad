# connector-prometheus Specification

## Purpose
TBD - created by archiving change add-gateway-core. Update Purpose after archive.
## Requirements
### Requirement: Unified metrics tools over the Prometheus HTTP API

The Prometheus connector SHALL serve `query_metrics` (instant and range) and `list_metric_names` by mapping unified arguments to the Prometheus HTTP API (`/api/v1/query`, `/api/v1/query_range`, `/api/v1/label/__name__/values`), compatible with Thanos and Mimir endpoints.

#### Scenario: Instant query

- **WHEN** an agent calls `query_metrics` with a PromQL expression and no range
- **THEN** the connector issues `/api/v1/query` and returns parsed samples with metric labels, values, and timestamps

#### Scenario: Range query

- **WHEN** an agent calls `query_metrics` with `start`, `end`, and `step`
- **THEN** the connector issues `/api/v1/query_range` and returns the series matrix

#### Scenario: Metric discovery

- **WHEN** an agent calls `list_metric_names`
- **THEN** the connector returns the metric name list from `/api/v1/label/__name__/values`

### Requirement: Health reporting

The connector SHALL report backend health without crashing the gateway when the backend is unreachable.

#### Scenario: Unreachable backend

- **WHEN** the Prometheus URL does not respond
- **THEN** `Health` returns an error, `list_connectors` shows the connector unhealthy, and the server keeps serving other tools

