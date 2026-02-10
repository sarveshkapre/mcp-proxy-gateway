# INCIDENTS

## 2026-02-09 - Latency Defer Evaluation Bug (Pre-merge)
- Summary: New latency metrics initially used `defer s.metrics.observeLatency(time.Since(start))`, which evaluates `time.Since` immediately and produced incorrect timings.
- Impact: Incorrect latency bucket accounting; detected before merge by `golangci-lint` (`govet` defers check).
- Root cause: Incorrect defer pattern for time-delta calculation in `handleSingle` and batch item handlers.
- Fix: Wrap latency observation in deferred closures:
  - `defer func() { s.metrics.observeLatency(time.Since(start)) }()`
  - `defer func() { s.metrics.observeLatency(time.Since(itemStart)) }()`
- Prevention rule: For deferred duration measurements, always defer a closure and compute `time.Since` inside the closure body.

## 2026-02-10 - CI Failure: Disabled /metrics Returned 405
- Summary: `GET /metrics` returned `405 Method Not Allowed` when Prometheus exposition was disabled, causing unit test failure on `main`.
- Impact: GitHub Actions `ci` job failed (run `21836360939`), blocking clean green builds on `main`.
- Root cause: Router checked HTTP method before path, so `GET /metrics` (and other unknown GETs) hit the generic “POST-only” guard instead of a `404`.
- Fix: Route by path first; treat `/metrics` as `404` unless explicitly enabled; keep `/rpc` requiring `POST`.
- Prevention rule: When adding new endpoints, route by path first and only apply method guards inside the matched endpoint.
