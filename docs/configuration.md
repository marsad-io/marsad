# Configuration reference

Marsad reads a single YAML file, `marsad.yaml` by default. Point it elsewhere with `marsad serve --config /path/to/file.yaml`.

A test in `internal/config` asserts this document covers every config field, so it cannot silently drift from the code.

## Full example

```yaml
connectors:
  - name: prod-prometheus
    type: prometheus
    url: http://prometheus:9090
    bearer_token_env: PROM_TOKEN   # optional, name of an env var

guardrails:
  max_time_range: 24h

audit:
  sink: stderr
  include_arguments: false
```

## `connectors`

List of backends the gateway talks to. Marsad makes no network connection to any host outside this list - that is enforced in the HTTP transport layer, not by convention.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique name for this connector. Agents use it in the `connector` tool argument when more than one connector serves a tool. |
| `type` | yes | Backend type. Currently `prometheus` (Thanos and Mimir compatible). |
| `url` | yes | Base URL of the backend. |
| `bearer_token_env` | no | Name of an environment variable holding a bearer token for the backend. |

Credentials are never written inline. A field like `password:` or `token:` in the file fails validation with instructions to use an env reference instead.

## `guardrails`

| Field | Default | Description |
|-------|---------|-------------|
| `max_time_range` | `24h` | Maximum time span a range query may cover, as a Go duration string (`30m`, `6h`, `24h`). Oversized queries are rejected before reaching the backend. |

## `audit`

Every tool call - success, failure, or guardrail rejection - emits exactly one JSON line.

| Field | Default | Description |
|-------|---------|-------------|
| `sink` | `stderr` | Where audit lines go: `stderr` or a file path (append, `0600`). |
| `include_arguments` | `false` | Audit lines carry only a hash of the arguments by default. Set `true` to record full argument values. |

Audit line fields: `ts`, `tool`, `connector`, `args_hash`, `duration_ms`, `outcome` (`ok` / `error` / `rejected`), `bytes`, plus `error` on failures and `args` when opted in.

## Environment overrides

`MARSAD_`-prefixed variables take precedence over file values. The supported set is a deliberate allowlist:

| Variable | Overrides |
|----------|-----------|
| `MARSAD_AUDIT_SINK` | `audit.sink` |
| `MARSAD_GUARDRAILS_MAX_TIME_RANGE` | `guardrails.max_time_range` |

Secrets always come from the environment via `bearer_token_env` references, never from the file.
