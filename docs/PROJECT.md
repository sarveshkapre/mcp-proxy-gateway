# PROJECT

## Commands
```bash
make setup
make dev
make fmt
make fmtcheck
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
- Operational endpoints: `/healthz` and `/metricsz` (and optional Prometheus `GET /metrics` when enabled).
- Record/replay files are NDJSON (one request/response per line).
- Recorder rotation/retention is configurable via `policy.record.max_bytes` / `policy.record.max_files` (and can be overridden via CLI flags).
- JSON-RPC batch requests are supported; the gateway processes batch items sequentially.
- Replay matching can be configured via `policy.replay.match` (`signature`, `method`, `tool`).

## Next 3 improvements
1. Policy-driven upstream header-forwarding allowlist (beyond the current minimal passthrough) to support authenticated and traceable upstreams without becoming a generic HTTP proxy.
2. Integration tests for streaming + replay/strict interactions and batch behavior (and explicit docs for unsupported combos).
3. Prometheus exposition format for metrics (keep `/metricsz` JSON as the local-first default).
