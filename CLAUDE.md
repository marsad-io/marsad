# Marsad - Claude Code Configuration

Vendor-neutral observability MCP gateway. Go, Apache-2.0, public repo.

## Workflow (non-negotiable)

1. **OpenSpec flow** - every feature starts as a change proposal in `openspec/changes/<id>/` (proposal.md, tasks.md, specs deltas). No implementation without an approved proposal. Archive changes when deployed.
2. **TDD** - write the failing test first, watch it fail, implement, watch it pass, refactor. No production code without a test that demanded it.

## Git rules

- Identity: `Aizhan Azhybaeva <aizhandxb@gmail.com>` (already set repo-local; never commit as any other identity)
- Conventional commits (`feat:`, `fix:`, `test:`, `docs:`, `chore:`)
- Never mention personal names in code, docs, or commit messages
- Never force-push

## Code conventions

- Go 1.24+, module `github.com/marsad-io/marsad`
- Official MCP Go SDK: `github.com/modelcontextprotocol/go-sdk`
- Connectors implement the `Connector` interface (Query, Health, Capabilities); one package per backend under `internal/connector/`
- Unit tests alongside code (`_test.go`); integration tests in `test/integration/` using testcontainers-go; golden files for tool schemas in `testdata/`
- Zero telemetry, no mandatory outbound calls, read-only by default - these are product invariants, not preferences

## Prose style (README, docs, comments)

- Plain hyphens only, never em dashes or double hyphens
- Conversational and direct, no corporate filler
