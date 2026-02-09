# PLAN

## Product pitch
Local-first HTTP JSON-RPC gateway for MCP that validates and gates `tools/call`, and records/replays calls for deterministic tests.

## Features
- Proxy `POST /rpc` to an upstream MCP server (JSON-RPC).
- Operational endpoints: `GET /healthz` and `GET /metricsz` (request/replay/validation/upstream/latency counters).
- Policy enforcement for `tools/call` (allow/deny lists + JSON Schema; modes: `enforce`/`audit`/`off`).
- Record requests/responses to NDJSON and replay without an upstream server.
- Smoke test for replay mode (`make smoke`).

## Top risks / unknowns
- Replay signature stability vs. JSON canonicalization differences.
- Recordings can accidentally capture secrets if redaction policy is not configured.
- Streaming/SSE transport still unsupported for long-running responses.

## Commands
See `docs/PROJECT.md` for the full list. Common ones:
```bash
make setup
make check
make smoke
make build
```

## Shipped
- 2026-02-01: v0.1.0 scaffold (proxy + validation + record/replay + smoke test).
- 2026-02-01: Reliability/ops polish (graceful shutdown, `/healthz`, size-limit handling, replay load supports large lines, lint auto-installs).

## Next to ship
- Add lightweight metrics endpoint for runtime counters. ✅ (`GET /metricsz`)
- Add recorder rotation/retention controls (max size and optional rollover). ✅ (`policy.record.max_bytes` / `policy.record.max_files`)
- Add streaming/SSE passthrough support for long-running MCP tool responses.
