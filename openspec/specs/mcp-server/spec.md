# mcp-server Specification

## Purpose
TBD - created by archiving change add-gateway-core. Update Purpose after archive.
## Requirements
### Requirement: MCP transports

The server SHALL expose MCP over stdio and streamable HTTP transports, selectable via `marsad serve --transport`.

#### Scenario: stdio round-trip

- **WHEN** an MCP client connects over stdio and calls `tools/list`
- **THEN** the server responds with the registered unified tools and valid JSON schemas

#### Scenario: HTTP round-trip

- **WHEN** the server runs with `--transport http --listen :8811` and a client calls a tool
- **THEN** the call succeeds over streamable HTTP with the same result as stdio

### Requirement: Tool registration from connector capabilities

The server SHALL register the union of all configured connectors' declared capabilities as MCP tools and route calls to the owning connector.

#### Scenario: Tools reflect configuration

- **WHEN** only a Prometheus connector is configured
- **THEN** `tools/list` includes `query_metrics`, `list_metric_names`, `list_connectors` and no log-search tools

#### Scenario: Connector listing

- **WHEN** an agent calls `list_connectors`
- **THEN** the response includes each configured connector's name, type, and current health status

