# INCIDENTS

## 2026-02-11 - SSE Negotiation Gap on Single Requests (Pre-merge)
- Summary: The single-request path would stream upstream SSE whenever upstream returned `text/event-stream`, even when the client did not request SSE.
- Impact: Non-stream JSON clients could receive streaming payloads unexpectedly, causing parse/UX failures and inconsistent transport semantics.
- Root cause: SSE passthrough gate depended only on upstream `Content-Type`, not client `Accept`.
- Fix: Require explicit client `Accept: text/event-stream` plus upstream SSE content type before passthrough; otherwise return JSON-RPC upstream error.
- Prevention rule: For any transport upgrade behavior (streaming, compression, protocol switch), enforce bilateral negotiation checks (client capability signal + upstream response).

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

### 2026-02-12T20:01:01Z | Codex execution failure
- Date: 2026-02-12T20:01:01Z
- Trigger: Codex execution failure
- Impact: Repo session did not complete cleanly
- Root Cause: codex exec returned a non-zero status
- Fix: Captured failure logs and kept repository in a recoverable state
- Prevention Rule: Re-run with same pass context and inspect pass log before retrying
- Evidence: pass_log=logs/20260212-101456-mcp-proxy-gateway-cycle-2.log
- Commit: pending
- Confidence: medium

### 2026-02-12T20:04:30Z | Codex execution failure
- Date: 2026-02-12T20:04:30Z
- Trigger: Codex execution failure
- Impact: Repo session did not complete cleanly
- Root Cause: codex exec returned a non-zero status
- Fix: Captured failure logs and kept repository in a recoverable state
- Prevention Rule: Re-run with same pass context and inspect pass log before retrying
- Evidence: pass_log=logs/20260212-101456-mcp-proxy-gateway-cycle-3.log
- Commit: pending
- Confidence: medium

### 2026-02-12T20:07:57Z | Codex execution failure
- Date: 2026-02-12T20:07:57Z
- Trigger: Codex execution failure
- Impact: Repo session did not complete cleanly
- Root Cause: codex exec returned a non-zero status
- Fix: Captured failure logs and kept repository in a recoverable state
- Prevention Rule: Re-run with same pass context and inspect pass log before retrying
- Evidence: pass_log=logs/20260212-101456-mcp-proxy-gateway-cycle-4.log
- Commit: pending
- Confidence: medium
