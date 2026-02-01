# PROJECT

## Commands
```bash
make setup
make dev
make test
make lint
make typecheck
make build
make check
make smoke
make release
```

## Development notes
- The HTTP gateway listens on `/rpc`.
- Record/replay files are NDJSON (one request/response per line).
- JSON-RPC batch requests are supported; the gateway processes batch items sequentially.
- Replay matching can be configured via `policy.replay.match` (`signature`, `method`, `tool`).

## Next 3 improvements
1. Batch JSON-RPC support with per-request validation.
2. Streaming/SSE proxy support.
3. Configurable redaction for record files. ✅ (see `policy.record`)
