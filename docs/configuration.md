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
  - name: prod-loki
    type: loki
    url: http://loki:3100
    basic_auth_user_env: LOKI_USER          # optional, pairs with the password ref
    basic_auth_password_env: LOKI_PASSWORD

guardrails:
  max_time_range: 24h
  max_result_bytes: 1048576

audit:
  sink: stderr
  include_arguments: false
```

## `connectors`

List of backends the gateway talks to. Marsad makes no network connection to any host outside this list - that is enforced in the HTTP transport layer, not by convention.

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Unique name for this connector. Agents use it in the `connector` tool argument when more than one connector serves a tool. |
| `type` | yes | Backend type: `prometheus` (Thanos and Mimir compatible) serving `query_metrics` and `list_metric_names`, or `loki` serving `search_logs` and `list_log_labels`. |
| `url` | yes | Base URL of the backend. |
| `bearer_token_env` | no | Name of an environment variable holding a bearer token for the backend. |
| `basic_auth_user_env` | no | Name of an environment variable holding the basic auth username. Use together with `basic_auth_password_env`; mutually exclusive with `bearer_token_env`. |
| `basic_auth_password_env` | no | Name of an environment variable holding the basic auth password. |

Credentials are never written inline. A field like `password:` or `token:` in the file fails validation with instructions to use an env reference instead.

## `guardrails`

| Field | Default | Description |
|-------|---------|-------------|
| `max_time_range` | `24h` | Maximum time span a range query may cover, as a Go duration string (`30m`, `6h`, `24h`). Oversized queries are rejected before reaching the backend. |
| `max_result_bytes` | `1048576` (1 MiB) | Byte budget per tool result. A result whose JSON serialization exceeds it is truncated: the tool result becomes a wrapper with `truncated: true`, `total_bytes`, `returned_bytes`, a human-readable `notice`, and the leading `partial` bytes, so agents know the result is partial. Must be positive. |

## `audit`

Every tool call - success, failure, or guardrail rejection - emits exactly one JSON line.

| Field | Default | Description |
|-------|---------|-------------|
| `sink` | `stderr` | Where audit lines go: `stderr` or a file path (append, `0600`). |
| `include_arguments` | `false` | Audit lines carry only a hash of the arguments by default. Set `true` to record full argument values. |

Audit line fields: `ts`, `tool`, `connector`, `args_hash`, `duration_ms`, `outcome` (`ok` / `error` / `rejected`), `bytes`, plus `error` on failures, `args` when opted in, and `total_bytes` (the pre-truncation size) when a result was truncated by `max_result_bytes`.

## Environment overrides

`MARSAD_`-prefixed variables take precedence over file values. The supported set is a deliberate allowlist:

| Variable | Overrides |
|----------|-----------|
| `MARSAD_AUDIT_SINK` | `audit.sink` |
| `MARSAD_GUARDRAILS_MAX_TIME_RANGE` | `guardrails.max_time_range` |
| `MARSAD_GUARDRAILS_MAX_RESULT_BYTES` | `guardrails.max_result_bytes` |

Secrets always come from the environment via `bearer_token_env` or the `basic_auth_*_env` references, never from the file.
