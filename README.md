# Marsad

**Marsad** (Arabic: مرصد, "observatory") is a vendor-neutral MCP gateway for observability. One binary, one consistent tool schema, every self-hosted backend your AI agents need.

Point any MCP-capable agent (Claude, IDE assistants, SRE agents) at Marsad and it can query metrics, search logs, inspect alerts, and read Kubernetes state - across Prometheus, Loki, Elasticsearch/OpenSearch, ClickHouse, Alertmanager, and the Kubernetes API - through unified tools like `query_metrics` and `search_logs`. Swap a backend and your agent prompts do not change.

## Why Marsad

- **Vendor-neutral.** Not tied to any observability vendor's platform. Direct-to-backend connectors for the open source stack you already run.
- **Sovereign by design.** Single static binary, fully air-gapped operation, zero telemetry, no phone-home - verifiable in source. Your observability data never leaves your network.
- **Guardrailed for agents.** Read-only by default, per-connector scoping, query cost limits, and a full audit log of every tool call. You always know what an agent read and how much it cost.
- **One schema.** Agents see consistent tools, not per-vendor APIs. The same prompt works whether logs live in Loki or Elasticsearch.

## Quickstart

You need Go 1.24+ and Docker. Start a Prometheus with real data (it scrapes itself) and a Loki, then run Marsad against both:

```bash
# 1. Start the backends
docker compose -f examples/quickstart/docker-compose.yml up -d

# 2. Give Loki something to find
curl -s -X POST http://localhost:3100/loki/api/v1/push \
  -H 'Content-Type: application/json' \
  -d "{\"streams\":[{\"stream\":{\"app\":\"quickstart\"},\"values\":[[\"$(date +%s)000000000\",\"hello from the quickstart\"]]}]}"

# 3. Run the gateway over HTTP
go run ./cmd/marsad serve --config examples/quickstart/marsad.yaml --transport http --listen :8811
```

Connect Claude Code to it:

```bash
claude mcp add --transport http marsad http://localhost:8811
```

Or over stdio, no HTTP port at all:

```bash
claude mcp add marsad -- go run ./cmd/marsad serve --config examples/quickstart/marsad.yaml
```

Then ask your agent something like "what metrics does this Prometheus have?" or "search the logs for app quickstart" - it will call `list_metric_names`, `query_metrics`, and `search_logs` through Marsad, and every call lands in the audit log (stderr by default). Configuration is documented in [docs/configuration.md](docs/configuration.md).

## Status

Early development. Implemented so far: the MCP server core with stdio and streamable HTTP transports, the Prometheus and Loki connectors, and the guardrails baseline (read-only enforcement, query time-range caps, result size budget, per-call audit log, outbound allowlist); see [`openspec/`](openspec/) for the specs.

## v1 connectors

| Backend | Tools | Status |
|---------|-------|--------|
| Prometheus (Thanos/Mimir compatible) | `query_metrics`, `list_metric_names` | available |
| Loki | `search_logs`, `list_log_labels` | available |
| Elasticsearch / OpenSearch | `search_logs` | planned |
| ClickHouse | `query_metrics`, `search_logs` | planned |
| Alertmanager | `list_alerts` | planned |
| Kubernetes API (read-only) | `get_k8s_events`, `get_pod_status`, `get_pod_logs` | planned |

## License

[Apache-2.0](LICENSE)

## Sponsors

Founded and maintained with the support of [NomadX](https://nomadx.ae) - AI agents consultancy, Dubai.
