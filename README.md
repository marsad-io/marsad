# Marsad

**Marsad** (Arabic: مرصد, "observatory") is a vendor-neutral MCP gateway for observability. One binary, one consistent tool schema, every self-hosted backend your AI agents need.

Point any MCP-capable agent (Claude, IDE assistants, SRE agents) at Marsad and it can query metrics, search logs, inspect alerts, and read Kubernetes state - across Prometheus, Loki, Elasticsearch/OpenSearch, ClickHouse, Alertmanager, and the Kubernetes API - through unified tools like `query_metrics` and `search_logs`. Swap a backend and your agent prompts do not change.

## Why Marsad

- **Vendor-neutral.** Not tied to any observability vendor's platform. Direct-to-backend connectors for the open source stack you already run.
- **Sovereign by design.** Single static binary, fully air-gapped operation, zero telemetry, no phone-home - verifiable in source. Your observability data never leaves your network.
- **Guardrailed for agents.** Read-only by default, per-connector scoping, query cost limits, and a full audit log of every tool call. You always know what an agent read and how much it cost.
- **One schema.** Agents see consistent tools, not per-vendor APIs. The same prompt works whether logs live in Loki or Elasticsearch.

## Status

Early development. The first milestone (MCP server core, Prometheus connector, guardrails baseline) is specified in [`openspec/changes/`](openspec/changes/) and under active build.

## Planned v1 connectors

| Backend | Tools |
|---------|-------|
| Prometheus (Thanos/Mimir compatible) | `query_metrics`, `list_metric_names` |
| Loki | `search_logs` |
| Elasticsearch / OpenSearch | `search_logs` |
| ClickHouse | `query_metrics`, `search_logs` |
| Alertmanager | `list_alerts` |
| Kubernetes API (read-only) | `get_k8s_events`, `get_pod_status`, `get_pod_logs` |

## License

[Apache-2.0](LICENSE)

## Sponsors

Founded and maintained with the support of [NomadX](https://nomadx.ae) - AI agents consultancy, Dubai.
