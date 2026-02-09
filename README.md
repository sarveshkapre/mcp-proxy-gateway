# MCP Proxy Gateway

Observe and gate MCP tool calls with schema validation, and record/replay for deterministic tests.

## What it does
- HTTP JSON-RPC proxy that forwards to an upstream MCP server
- Validates `tools/call` arguments with JSON Schema
- Records requests/responses to NDJSON
- Replays recorded calls without an upstream server
- Streams upstream SSE responses when the client requests it (`Accept: text/event-stream`)
- Health endpoint for status checks (`GET /healthz`)
- Metrics endpoint for local runtime counters (`GET /metricsz`)
- Supports JSON-RPC batch requests (handled sequentially per item)
- Implements JSON-RPC notification semantics (`204 No Content` when request omits `id`)
- Rewrites replayed response IDs to the incoming request ID for correlation safety

## Quickstart
```bash
make setup
make build
./bin/mcp-proxy-gateway \
  --listen :8080 \
  --upstream http://localhost:8090/rpc \
  --policy ./policy.example.yaml \
  --record ./records.ndjson
```

Send JSON-RPC requests to `http://localhost:8080/rpc`.
Check health at `http://localhost:8080/healthz`.
Check metrics at `http://localhost:8080/metricsz`.

## Demo (replay)
```bash
make build
./bin/mcp-proxy-gateway --listen :8080 --replay ./records.example.ndjson --replay-strict
curl -sS -X POST http://localhost:8080/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hello","max_results":3}}}'
```

## Replay mode
```bash
./bin/mcp-proxy-gateway \
  --listen :8080 \
  --replay ./records.example.ndjson \
  --replay-strict
```

Replay lookup matching is configurable in the policy:
```yaml
replay:
  match: signature # signature (default), method, or tool
```

## Streaming/SSE passthrough
If an upstream tool response is long-running and the upstream server supports SSE, clients can request it with:
```bash
curl -sS -N -X POST http://localhost:8080/rpc \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"tool":"web.search","arguments":{"query":"hello"}}}'
```

Notes:
- The gateway streams the upstream response bytes as-is when the upstream responds with `Content-Type: text/event-stream`.
- Streamed responses are not recorded (record/replay is JSON-only).

## Policy example
```yaml
version: 1
mode: enforce
allow_tools:
  - web.search
  - fs.read

http:
  # Optional CSRF-style hardening for browser-initiated requests: if a request
  # includes an `Origin` header not in this list, it is rejected (403). Requests
  # without an Origin header are allowed.
  origin_allowlist: ["http://localhost:3000"]

record:
  # Redaction is applied before writing NDJSON recordings.
  redact_keys: ["token", "access_token", "api_key", "authorization"]
  redact_key_regex: ["(?i)secret|password"]
  # Optional recorder lifecycle controls:
  # - max_bytes rotates the active file when the next append would exceed this size.
  # - max_files retains up to N rotated backups as `records.ndjson.1..N`.
  max_bytes: 10485760 # 10 MiB
  max_files: 3

tools:
  web.search:
    schema:
      type: object
      properties:
        query:
          type: string
        max_results:
          type: integer
          minimum: 1
          maximum: 10
      required: [query]
      additionalProperties: false
```

## Example files
- `policy.example.yaml`
- `records.example.ndjson`

## Smoke test
```bash
make smoke
```

## Docs
See `docs/` for architecture, commands, and contribution details.

## License
MIT
