# Clone Feature Tracker

## Context Sources
- README and docs
- TODO/FIXME markers in code
- Test and build failures
- Gaps found during codebase exploration

## Candidate Features To Do
- [ ] BACKLOG P1: Add recorder rotation/retention controls (max size and optional count-based rollover) to prevent unbounded NDJSON growth.
- [ ] BACKLOG P1: Add streaming/SSE passthrough mode to support long-running MCP tool responses.
- [ ] BACKLOG P2: Add integration tests for replay match modes (`signature`, `method`, `tool`) with notification edge cases.
- [ ] BACKLOG P2: Add benchmark coverage for batch throughput and replay lookup hot paths.

## Implemented
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
- `docs/PLAN.md` checklist had drifted out of sync with implementation; keeping this updated prevents false-positive backlog detection in automation loops.
- `govet` defer diagnostics caught an early latency-measurement bug; keeping `make check` mandatory before push prevented incorrect metrics shipping.

## Notes
- This file is maintained by the autonomous clone loop.

### Checklist Sync (2026-02-09)
- `docs/PLAN.md` MVP checklist now reflects shipped status (`[x]` across core MVP items).
