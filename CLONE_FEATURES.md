# Clone Feature Tracker

## Context Sources
- README and docs
- TODO/FIXME markers in code
- Test and build failures
- Gaps found during codebase exploration

## Candidate Features To Do
- [ ] BACKLOG P1: Add optional request correlation support (`X-Request-Id` generation + upstream forwarding + recorder correlation metadata).
  Scoring: impact=med effort=med fit=high diff=quality risk=low confidence=med
- [ ] BACKLOG P1: Add optional pprof/debug endpoints behind an explicit flag and localhost-only binding guard.
  Scoring: impact=med effort=low fit=med diff=quality risk=low confidence=med
- [ ] BACKLOG P1: Add per-tool validation reject counters in `/metricsz` and `/metrics` with bounded cardinality controls.
  Scoring: impact=med effort=med fit=med diff=quality risk=med confidence=med
- [ ] BACKLOG P1: Add upstream retry policy for transient network failures with strict idempotency-safe defaults.
  Scoring: impact=med effort=med fit=med diff=parity risk=med confidence=low
- [ ] BACKLOG P2: Extend replay verification tooling with a CLI command to detect stale/invalid NDJSON recordings.
  Scoring: impact=med effort=med fit=high diff=differentiator risk=low confidence=med
- [ ] BACKLOG P2: Add a dedicated integration test suite that boots gateway+stub upstream and verifies record/replay lifecycle end-to-end.
  Scoring: impact=med effort=med fit=high diff=quality risk=low confidence=med
- [ ] BACKLOG P2: Support configurable upstream URL allowlist/denylist constraints to reduce SSRF risk when automated launch wrappers are used.
  Scoring: impact=med effort=med fit=med diff=security risk=med confidence=low
- [ ] BACKLOG P2: Add graceful upstream circuit-breaker controls (open/half-open) as optional policy for noisy failure periods.
  Scoring: impact=med effort=high fit=med diff=parity risk=med confidence=low
- [ ] BACKLOG P2: Add optional OpenTelemetry traces/spans export for request flow visibility (kept opt-in/local-first).
  Scoring: impact=med effort=high fit=med diff=parity risk=med confidence=low
- [ ] BACKLOG P3: Add Prometheus labels for key dimensions (`replay_match`, `upstream_configured`) without cardinality explosion.
  Scoring: impact=low effort=med fit=med diff=nice risk=med confidence=low
- [ ] BACKLOG P3: Extend streaming support beyond SSE passthrough to Streamable HTTP/session semantics while preserving local-first defaults.
  Scoring: impact=high effort=high fit=high diff=parity risk=med confidence=low
- [ ] BACKLOG P3: Add a minimal web UI inspector for local replay records (read-only) for developer ergonomics.
  Scoring: impact=low effort=high fit=low diff=differentiator risk=med confidence=low

## Implemented
- [x] 2026-02-11: Add MCP endpoint compatibility by accepting `POST /mcp` as an alias to `POST /rpc`, with method-guard parity and smoke coverage.
  Evidence: `internal/proxy/proxy.go`, `internal/proxy/proxy_test.go` (`TestMCPEndpointPostAlias`, `TestMCPEndpointWrongMethodReturns405`), `scripts/smoke.sh`, `README.md`, `docs/PROJECT.md`, `cmd/mcp-proxy-gateway/main.go`.
- [x] 2026-02-11: Tighten SSE semantics so single-request SSE passthrough only occurs when client explicitly requests `Accept: text/event-stream`; otherwise return JSON-RPC upstream error.
  Evidence: `internal/proxy/proxy.go`, `internal/proxy/stream_test.go` (`TestUnexpectedSSEWithoutClientAcceptReturnsJSONRPCError`), `internal/proxy/proxy_test.go` (`TestSingleReplayHitWithSSEAcceptStillReturnsJSON`), `README.md`.
- [x] 2026-02-11: Add handler-level benchmarks for proxy batch replay-hit and upstream forwarding paths.
  Evidence: `internal/proxy/proxy_benchmark_test.go`, command `go test ./internal/proxy -run '^$' -bench 'BenchmarkServeHTTPBatch' -benchmem`.
- [x] 2026-02-10: Fix routing semantics so disabled Prometheus exposition (`/metrics`) returns `404` (and unknown paths return `404`, not `405`).
  Evidence: `internal/proxy/proxy.go`, `internal/proxy/proxy_test.go`, GitHub Actions CI run fix.
- [x] 2026-02-10: Run `make smoke` in CI and harden `scripts/smoke.sh` to avoid port/tmp collisions; bump CI Go version to match tooling.
  Evidence: `.github/workflows/ci.yml`, `scripts/smoke.sh`, `internal/proxy/proxy_test.go`.
- [x] 2026-02-09: Policy-driven upstream header forwarding allowlist via `policy.http.forward_headers` (keeps narrow defaults to avoid becoming a generic HTTP proxy).
  Evidence: `internal/proxy/proxy.go` (allowlist copy), `internal/config/config.go` (policy field + validation), `internal/proxy/headers_test.go` (regression), `README.md` + `policy.example.yaml` (docs/examples).
- [x] 2026-02-09: Formatting guardrails added (`make fmt`, `make fmtcheck`) and `fmtcheck` enforced by `make check` (CI).
  Evidence: `Makefile`, `docs/PROJECT.md`, `CHANGELOG.md`.
- [x] 2026-02-09: Changelog hygiene: cut v0.2.0 (2026-02-09) entry and reset “Unreleased”.
  Evidence: `CHANGELOG.md`, `UPDATE.md`.
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
- Upstream header forwarding is intentionally explicit: `Authorization` is forwarded; additional headers require `policy.http.forward_headers` to avoid accidental secret propagation.
- `docs/PLAN.md` checklist had drifted out of sync with implementation; keeping this updated prevents false-positive backlog detection in automation loops.
- `govet` defer diagnostics caught an early latency-measurement bug; keeping `make check` mandatory before push prevented incorrect metrics shipping.
- Market scan (2026-02-10, untrusted): MCP gateways in the wild cluster into (1) “auth front-doors” for existing MCP servers, and (2) “gateways/registries” that aggregate many servers and add middleware. Transport bridging (stdio <-> HTTP/SSE/Streamable HTTP), session isolation, and observability (Prometheus/OTEL) show up frequently once the gateway is network-exposed.
  Sources: https://github.com/sigbit/mcp-auth-proxy, https://github.com/matthisholleville/mcp-gateway, https://github.com/dgellow/mcp-front, https://github.com/IBM/mcp-context-forge, https://model-context-protocol.com/servers/mcp-gateway-stdio-http-rest-api.
- Gap map (2026-02-10): Missing: Streamable HTTP/session semantics beyond SSE passthrough; Weak: CI runtime smoke coverage (now addressed) and endpoint routing semantics; Parity: SSE passthrough + opt-in Origin allowlist + Prometheus text exposition; Differentiator: schema gating + record/replay for deterministic tests.
- Market scan (2026-02-11, untrusted): Transport-facing MCP gateways commonly expose an `/mcp`-style endpoint and increasingly separate SSE negotiation from baseline JSON transport; explicit non-stream fallback behavior is expected for compatibility.
  Sources: https://github.com/matthisholleville/mcp-gateway, https://github.com/sigbit/mcp-auth-proxy, https://github.com/dgellow/mcp-front, https://github.com/IBM/mcp-context-forge, https://modelcontextprotocol.io/docs/concepts/transports.
- Gap map (2026-02-11): Missing: Streamable HTTP session semantics; Weak: explicit transport compatibility docs and single-request SSE negotiation guardrails (addressed in this session); Parity: `/mcp` endpoint compatibility plus SSE passthrough; Differentiator: deterministic record/replay with schema-gated `tools/call`.

## Notes
- This file is maintained by the autonomous clone loop.

### Checklist Sync (2026-02-09)
- `docs/PLAN.md` MVP checklist now reflects shipped status (`[x]` across core MVP items).
