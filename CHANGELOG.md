# CHANGELOG

## Unreleased
- Add optional Prometheus text exposition endpoint at `GET /metrics` (enable via `--prometheus-metrics` or `policy.http.prometheus_metrics`).
- Extend `/metricsz` JSON payload with `latency_count` and `latency_sum_ms`.

## 0.2.0 - 2026-02-09
- Add SSE passthrough for long-running upstream responses when the upstream responds with `Content-Type: text/event-stream` (client requests with `Accept: text/event-stream`).
- Add optional `policy.http.origin_allowlist` hardening to reject unexpected browser-originated requests (403 when an `Origin` header is present but not allowlisted).
- Add explicit `policy.http.forward_headers` allowlist for upstream request header forwarding (beyond the minimal defaults).
- Forward `Authorization` header to upstream requests (and forward `Accept` only for SSE requests).
- Add `GET /metricsz` endpoint exposing runtime counters:
  - `requests_total`
  - `batch_items_total`
  - `replay_hits_total` / `replay_misses_total`
  - `validation_rejects_total`
  - `upstream_errors_total`
  - `latency_buckets_ms`
- Add recorder rotation/retention controls for NDJSON recordings (`policy.record.max_bytes`, `policy.record.max_files`, plus CLI overrides).
- Add proxy metrics accounting hooks for single + batch paths and regression tests.
- Extend smoke script to verify metrics endpoint availability and non-zero request count.
- Upgrade CodeQL GitHub Action from `v3` to `v4` to avoid announced deprecation.
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
- Add gofmt guardrails: `make fmt` and `make fmtcheck`, and include `fmtcheck` in `make check`.

## 0.1.0 - 2026-02-01
- Initial scaffold.
- HTTP JSON-RPC proxy with `tools/call` validation and record/replay.
- Smoke test and signature helper.
