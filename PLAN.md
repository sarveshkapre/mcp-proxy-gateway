# PLAN

## Product pitch
Local-first HTTP JSON-RPC gateway for MCP that validates and gates `tools/call`, and records/replays calls for deterministic tests.

## Features
- Proxy `POST /rpc` to an upstream MCP server (JSON-RPC).
- Policy enforcement for `tools/call` (allow/deny lists + JSON Schema; modes: `enforce`/`audit`/`off`).
- Record requests/responses to NDJSON and replay without an upstream server.
- Smoke test for replay mode (`make smoke`).

## Top risks / unknowns
- Replay signature stability vs. JSON canonicalization differences.
- Recordings can accidentally capture secrets (redaction is out-of-scope today).
- MVP is single-request JSON-RPC only (no batch / streaming / SSE).

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
- Add redaction hooks for recordings (denylist keys / regexes). ✅ (see `policy.record`)
- Add batch JSON-RPC support (validate each request, preserve ordering).
- Expand replay matching options (method-only, tool-only, and strict sig).
