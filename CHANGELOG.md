# CHANGELOG

## Unreleased
- Enforce JSON-RPC notification semantics for single requests (`204 No Content` when `id` is omitted).
- Rewrite replayed JSON-RPC response IDs to match incoming request IDs (single + batch).
- Add regression tests for replay ID remapping and single-notification behavior.
- Extend smoke test to verify notification handling (`204` + empty body).
- Add `GET /healthz` endpoint for basic status checks.
- Add graceful shutdown (SIGINT/SIGTERM).
- Improve request/response size-limit handling (`--max-body`).
- Make replay loader handle large NDJSON lines.
- Propagate client request cancellation to upstream requests.
- Make `make lint` auto-install `golangci-lint` if missing.
- Add optional redaction for recordings via `policy.record.redact_keys` / `policy.record.redact_key_regex`.
- Add JSON-RPC batch support (sequential per-item processing).
- Add replay match modes (`signature`, `method`, `tool`) via `policy.replay.match`.
- Make smoke test wait for server readiness.

## 0.1.0 - 2026-02-01
- Initial scaffold.
- HTTP JSON-RPC proxy with `tools/call` validation and record/replay.
- Smoke test and signature helper.
