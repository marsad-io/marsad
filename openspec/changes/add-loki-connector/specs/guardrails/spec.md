# guardrails Specification (delta)

## ADDED Requirements

### Requirement: Result size budget

The gateway SHALL enforce a configurable maximum result size per tool call; oversized results are truncated with an explicit truncation marker so agents know the result is partial.

#### Scenario: Oversized log result truncated

- **WHEN** a `search_logs` call produces a payload exceeding the configured byte budget
- **THEN** the result is truncated to the budget (UTF-8 safe), the tool result carries a truncation marker with total and returned byte sizes, and the audit line records both truncated and total sizes
