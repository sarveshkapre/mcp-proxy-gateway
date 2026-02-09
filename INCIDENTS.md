# INCIDENTS

## 2026-02-09 - Latency Defer Evaluation Bug (Pre-merge)
- Summary: New latency metrics initially used `defer s.metrics.observeLatency(time.Since(start))`, which evaluates `time.Since` immediately and produced incorrect timings.
- Impact: Incorrect latency bucket accounting; detected before merge by `golangci-lint` (`govet` defers check).
- Root cause: Incorrect defer pattern for time-delta calculation in `handleSingle` and batch item handlers.
- Fix: Wrap latency observation in deferred closures:
  - `defer func() { s.metrics.observeLatency(time.Since(start)) }()`
  - `defer func() { s.metrics.observeLatency(time.Since(itemStart)) }()`
- Prevention rule: For deferred duration measurements, always defer a closure and compute `time.Since` inside the closure body.
