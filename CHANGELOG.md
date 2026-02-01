# CHANGELOG

## Unreleased
- Add `GET /healthz` endpoint for basic status checks.
- Add graceful shutdown (SIGINT/SIGTERM).
- Improve request/response size-limit handling (`--max-body`).
- Make replay loader handle large NDJSON lines.
- Propagate client request cancellation to upstream requests.
- Make `make lint` auto-install `golangci-lint` if missing.
- Add optional redaction for recordings via `policy.record.redact_keys` / `policy.record.redact_key_regex`.
- Add JSON-RPC batch support (sequential per-item processing).

## 0.1.0 - 2026-02-01
- Initial scaffold.
- HTTP JSON-RPC proxy with `tools/call` validation and record/replay.
- Smoke test and signature helper.
