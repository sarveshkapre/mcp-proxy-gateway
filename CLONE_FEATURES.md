# Clone Feature Tracker

## Context Sources
- README and docs
- TODO/FIXME markers in code
- Test and build failures
- Gaps found during codebase exploration

## Candidate Features To Do
- [ ] BACKLOG P1: Expand upstream request header forwarding allowlist (beyond current minimal `Authorization` passthrough; add `Accept`, `Traceparent`, etc) with explicit docs to avoid accidental secret propagation.
  Scoring: impact=high effort=low fit=high diff=parity risk=med confidence=med
- [ ] BACKLOG P2: Add integration tests for streaming + replay/strict interactions and batch behavior (explicitly document unsupported combos).
  Scoring: impact=med effort=med fit=med diff=parity risk=med confidence=med
- [ ] BACKLOG P2: Add benchmark coverage for proxy batch throughput (handler-level) and replay lookup hot paths (beyond micro-benchmarks).
  Scoring: impact=low effort=med fit=med diff=parity risk=low confidence=med
- [ ] BACKLOG P3: Add Prometheus exposition format (`/metrics`) behind a flag/policy while keeping `/metricsz` JSON as the local-first default.
  Scoring: impact=low effort=med fit=med diff=nice risk=low confidence=med
- [ ] BACKLOG P3: Extend streaming support beyond SSE passthrough (Streamable HTTP/session semantics) while preserving local-first defaults.
  Scoring: impact=med effort=high fit=med diff=parity risk=med confidence=low

## Implemented
- [x] 2026-02-09: P0 SSE passthrough for long-running upstream responses when the upstream responds with `Content-Type: text/event-stream` (client requests with `Accept: text/event-stream`).
  Evidence: `internal/proxy/proxy.go` (SSE detection + streaming copy), `internal/proxy/stream_test.go` (SSE passthrough + skip record), `README.md` (usage notes).
- [x] 2026-02-09: P1 optional `policy.http.origin_allowlist` to reject unexpected browser-originated requests (403 when an `Origin` header is present but not allowlisted).
  Evidence: `internal/config/config.go` (policy struct), `cmd/mcp-proxy-gateway/main.go` (plumbing), `internal/proxy/proxy.go` (enforcement), `internal/proxy/origin_test.go` (unit tests), `policy.example.yaml` + `README.md` (docs/examples).
- [x] 2026-02-09: P2 smoke coverage expanded to include a real upstream stub (non-replay), origin allowlist rejection, and SSE passthrough verification.
  Evidence: `scripts/smoke.sh`.
- [x] 2026-02-09: P0 recorder rotation/retention controls for NDJSON recordings (max-bytes + max-files backups), with policy + CLI configuration.
  Evidence: `cmd/mcp-proxy-gateway/main.go` (CLI overrides), `internal/config/config.go` (policy fields + validation), `internal/record/record.go` (rotation), `internal/record/record_test.go` (rotation tests), `README.md` + `policy.example.yaml` (docs/examples).
- [x] 2026-02-09: P1 proxy-layer regression coverage for replay match modes (`method`/`tool`), including notification edge cases and ID remapping.
  Evidence: `internal/proxy/proxy_test.go` (`TestReplayMatchByMethodAtProxyLayerRemapsID`, `TestReplayMatchByToolAtProxyLayerRemapsID`, `TestReplayMatchByMethodNotificationReturns204`).
- [x] 2026-02-09: P2 replay lookup micro-benchmarks (signature/method/tool).
  Evidence: `internal/record/replay_benchmark_test.go`.
- [x] 2026-02-09: P0 track root `AGENTS.md` contract in git.
  Evidence: `AGENTS.md`.
- [x] 2026-02-09: P0 replay ID remapping implemented for replay hits in single and batch flows.
  Evidence: `internal/proxy/proxy.go` (`withResponseID`, replay branches in `handleSingle` and `handleBatch`), `internal/proxy/proxy_test.go` (`TestSingleReplayResponseIDIsRewritten`, `TestBatchReplayResponseIDIsRewritten`).
- [x] 2026-02-09: P0 single-request notification semantics enforced (`204 No Content` when `id` omitted).
  Evidence: `internal/proxy/proxy.go` (`isNotification` handling across replay, validation, upstream, and no-upstream paths), `internal/proxy/proxy_test.go` (`TestSingleNotificationReturns204AndForwards`, `TestSingleNotificationReplayHitReturns204`).
- [x] 2026-02-09: P1 regression coverage and smoke validation expanded for notification path.
  Evidence: `internal/proxy/proxy_test.go` (new notification/replay regression tests), `scripts/smoke.sh` (notification request asserts `204` + empty body).
- [x] 2026-02-09: P2 stale planning/docs state refreshed to match shipped behavior.
  Evidence: `docs/PLAN.md`, `docs/PROJECT.md`, `README.md`, `CHANGELOG.md`, `UPDATE.md`.
- [x] 2026-02-09: P2 CI maintenance hardening by upgrading CodeQL action to supported major version.
  Evidence: `.github/workflows/codeql.yml` (`github/codeql-action/init@v4`, `github/codeql-action/analyze@v4`).
- [x] 2026-02-09: P0 runtime metrics endpoint shipped with in-process counters.
  Evidence: `internal/proxy/proxy.go` (`GET /metricsz`, metrics accounting in single + batch handlers), `cmd/mcp-proxy-gateway/main.go` (startup endpoint log).
- [x] 2026-02-09: P0/P1 metrics regression coverage and smoke validation added.
  Evidence: `internal/proxy/proxy_test.go` (`TestMetricsz`, replay/validation/upstream/batch metrics tests), `scripts/smoke.sh` (`/metricsz` assertions).
- [x] 2026-02-09: P1 docs and memory synchronized for metrics release.
  Evidence: `README.md`, `docs/PROJECT.md`, `PLAN.md`, `docs/PLAN.md`, `docs/ROADMAP.md`, `CHANGELOG.md`, `UPDATE.md`, `PROJECT_MEMORY.md`, `INCIDENTS.md`.

## Insights
- JSON-RPC notification handling was already correct in batch mode but inconsistent in single mode; aligning both paths removed a client-visible protocol mismatch.
- Replay signatures intentionally ignore request IDs, so remapping replayed response IDs is required for safe client correlation.
- Recorder rotation defaults are intentionally conservative: rotation is off unless `max_bytes` (or `--record-max-bytes`) is set, and backups are retained unless explicitly configured to `0`.
- Streamed SSE responses are passed through as bytes and are not recorded/replayed (record/replay is JSON-only).
- `Origin` allowlisting is intentionally opt-in and only affects requests that include an `Origin` header; non-browser clients typically do not send one.
- `docs/PLAN.md` checklist had drifted out of sync with implementation; keeping this updated prevents false-positive backlog detection in automation loops.
- `govet` defer diagnostics caught an early latency-measurement bug; keeping `make check` mandatory before push prevented incorrect metrics shipping.
- Market scan (2026-02-09, untrusted): MCP gateways commonly emphasize Streamable HTTP/SSE support and session management for web clients, plus optional auth/rate limiting/observability when exposed beyond localhost.
  Sources: https://github.com/atrawog/mcp-streamablehttp-proxy, https://github.com/sigbit/mcp-auth-proxy, https://github.com/microsoft/mcp-gateway, https://github.com/matthisholleville/mcp-gateway, https://github.com/docker/mcp-gateway, https://github.com/lasso-security/mcp-gateway, https://github.com/Kuadrant/mcp-gateway.
- Gap map (2026-02-09): Missing: Streamable HTTP/session semantics beyond SSE passthrough; Weak: explicit upstream header-forwarding policy/docs; Parity: SSE passthrough + opt-in Origin allowlist; Differentiator: schema gating + record/replay for deterministic tests.

## Notes
- This file is maintained by the autonomous clone loop.

### Checklist Sync (2026-02-09)
- `docs/PLAN.md` MVP checklist now reflects shipped status (`[x]` across core MVP items).
