# PLAN

## Summary
Build a local-first MCP Proxy Gateway that sits between MCP clients and servers, validates tool-call inputs against JSON Schema, and records/replays calls for deterministic tests.

## Goals
- HTTP JSON-RPC gateway that forwards requests to an upstream MCP server.
- Schema validation for `tools/call` arguments with enforce/audit/off modes.
- Record tool calls and responses to NDJSON for deterministic replay.
- Deterministic replay mode without upstream dependency.

## Non-goals
- Full MCP spec coverage (batch, streaming, SSE) in MVP.
- Authn/authz, multi-tenant routing, or persistence beyond flat files.
- UI or dashboard.

## Target users
- Agent/tool developers who want deterministic tool-call tests.
- Security engineers who want gating/validation for tool calls.

## MVP scope
- Single JSON-RPC request per HTTP POST (no batch).
- `tools/call` validation with allow/deny lists and JSON Schema.
- `tools/list` and other methods passthrough.
- Record (append) and replay (map by request signature) modes.

## Architecture
- `cmd/mcp-proxy-gateway`: CLI entrypoint
- `internal/jsonrpc`: minimal JSON-RPC types and errors
- `internal/config`: policy config loader (YAML/JSON)
- `internal/validate`: schema validation + allow/deny rules
- `internal/record`: NDJSON record/replay store
- `internal/proxy`: HTTP handler + upstream forwarder

### Data flow
1. HTTP POST `/rpc` with JSON-RPC request
2. Parse + validate JSON-RPC
3. If replay enabled: serve recorded response (if match)
4. If `tools/call`: validate tool name + arguments
5. Forward to upstream (if not replayed) and return response
6. Optionally append record line

## Security considerations
- Request size limit and timeouts
- Explicit allow/deny lists for tool names
- JSON Schema validation to reduce injection risk
- No secret logging; record files are opt-in

## Risks
- JSON canonicalization mismatch affects replay hits
- Schemas may be incomplete and cause false rejects
- Upstream latency propagation

## Milestones
1. Scaffold repo + docs + CI + Makefile
2. Implement HTTP gateway + config + validation
3. Implement record/replay + tests
4. Dockerfile + docs polish + v0.1.0

## MVP checklist
- [ ] CLI parses flags, starts HTTP server
- [ ] JSON-RPC request validation
- [ ] Schema validation for `tools/call`
- [ ] Record/replay NDJSON support
- [ ] Unit tests for validation and replay
- [ ] `make check` passes locally
