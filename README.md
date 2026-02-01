# MCP Proxy Gateway

Observe and gate MCP tool calls with schema validation, and record/replay for deterministic tests.

## What it does
- HTTP JSON-RPC proxy that forwards to an upstream MCP server
- Validates `tools/call` arguments with JSON Schema
- Records requests/responses to NDJSON
- Replays recorded calls without an upstream server
- Health endpoint for status checks (`GET /healthz`)

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

## Policy example
```yaml
version: 1
mode: enforce
allow_tools:
  - web.search
  - fs.read

record:
  # Redaction is applied before writing NDJSON recordings.
  redact_keys: ["token", "access_token", "api_key", "authorization"]
  redact_key_regex: ["(?i)secret|password"]

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
