# Change: Add Alertmanager connector and unified alert tools

## Why

An investigation usually starts from an alert, not from a metric. With metrics (Prometheus) and logs (Loki) served, alerts are the missing leg of the triad: an agent should be able to ask "what is firing right now" and pivot into the metrics and logs behind it through the same gateway. Alertmanager is the de facto self-hosted alert hub, and its read-only API is small, making this a compact change.

## What Changes

- New unified tools in `internal/schema` with golden files: `list_alerts` (filter by label matchers and state: active, silenced, inhibited, unprocessed) and `list_silences` (read-only visibility into what is muted and why)
- New connector `internal/connector/alertmanager`: maps unified arguments to the Alertmanager v2 API (`/api/v2/alerts`, `/api/v2/silences`), `Health` via `/-/ready`, bearer/basic auth via env references (same plumbing as Loki)
- Guardrails apply unchanged: read-only enforcement, result size budget, audit, outbound allowlist join
- Three-connector e2e: alert fires in a real Alertmanager, agent lists it, then queries metrics and logs in the same MCP session

## Impact

- Affected specs: `connector-alertmanager` (new)
- Affected code: `internal/schema` (two tool specs), `internal/connector/alertmanager` (new), `internal/config` (alertmanager connector type), docs (configuration reference, README table)

## Non-goals

- Creating or expiring silences, acknowledging alerts - Marsad stays read-only; write operations are a separate future capability with its own guardrail design
- Alert history (Alertmanager only holds current state; history needs a different backend)
- Alert correlation or dedup logic - that is agent territory, not gateway territory
