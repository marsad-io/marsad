# guardrails Specification

## Purpose
TBD - created by archiving change add-gateway-core. Update Purpose after archive.
## Requirements
### Requirement: Read-only enforcement

The gateway SHALL execute only tool calls marked read-only in the unified schema; any other call is rejected before reaching a connector.

#### Scenario: Non-read-only call rejected

- **WHEN** a tool call not marked read-only enters the pipeline
- **THEN** it is rejected with an error stating Marsad is read-only, and the rejection is audit-logged

### Requirement: Query cost limits

The gateway SHALL enforce a configurable maximum time range per query.

#### Scenario: Oversized range rejected

- **WHEN** a `query_metrics` range query spans more than the configured maximum
- **THEN** the call is rejected with an error naming the limit and the requested range

### Requirement: Audit log for every tool call

The gateway SHALL emit one structured JSON audit line per tool call - success or failure - containing timestamp, tool, connector, argument hash, duration, outcome, and result size. Argument values SHALL be excluded unless explicitly enabled.

#### Scenario: Success audited

- **WHEN** a tool call succeeds
- **THEN** an audit line with outcome `ok` and all required fields is written to the configured sink

#### Scenario: Failure audited

- **WHEN** a tool call fails or is rejected by a guardrail
- **THEN** an audit line with the failure outcome is written; no call bypasses auditing

### Requirement: No outbound calls beyond configured backends

The gateway SHALL make no network connections other than to explicitly configured backend URLs - no telemetry, no update checks, no phone-home.

#### Scenario: Outbound allowlist enforced

- **WHEN** any component attempts an HTTP request to a host not in the configured backend set
- **THEN** the request is blocked by the shared transport layer and surfaced as an internal error in tests

### Requirement: Result size budget

The gateway SHALL enforce a configurable maximum result size per tool call; oversized results are truncated with an explicit truncation marker so agents know the result is partial.

#### Scenario: Oversized log result truncated

- **WHEN** a `search_logs` call produces a payload exceeding the configured byte budget
- **THEN** the result is truncated to the budget (UTF-8 safe), the tool result carries a truncation marker with total and returned byte sizes, and the audit line records both truncated and total sizes

