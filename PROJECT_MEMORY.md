# PROJECT_MEMORY

## Entry: 2026-02-09 - Upstream Header Forwarding Allowlist + Formatting Guardrails
- Decision: Add explicit `policy.http.forward_headers` allowlist for upstream request header forwarding (keep narrow defaults; `Authorization` always forwarded, `Accept` only forwarded for SSE requests).
- Why: Authenticated upstreams and tracing are common needs; making forwarding explicit reduces accidental secret propagation and prevents the gateway from becoming a generic HTTP proxy.
- Evidence:
  - Code: `internal/proxy/proxy.go`, `internal/config/config.go`, `cmd/mcp-proxy-gateway/main.go`
  - Tests: `internal/proxy/headers_test.go`, existing SSE header tests in `internal/proxy/stream_test.go`
  - Docs/examples: `README.md`, `policy.example.yaml`, `docs/PROJECT.md`, `CHANGELOG.md`, `UPDATE.md`
  - Verification:
    - `make check` (pass)
    - `make smoke` (pass)
- Commit: `e3e2817`
- Confidence: high
- Trust label: verified-local
- Follow-ups:
  - Add integration tests covering streaming + replay/strict and batch interactions (explicitly document unsupported combos).
  - Consider whether a future Streamable HTTP/session feature should have its own header-forwarding semantics (separate from JSON-RPC proxying).

## Entry: 2026-02-09 - SSE Passthrough + Origin Allowlist Hardening
- Decision: Support long-running upstream tool responses via SSE passthrough (stream when upstream responds `text/event-stream`), and add opt-in `policy.http.origin_allowlist` request hardening.
- Why: Streaming is now table-stakes for MCP gateways; origin allowlisting reduces CSRF-style browser risk when the gateway is bound beyond localhost.
- Evidence:
  - Code: `internal/proxy/proxy.go`, `internal/config/config.go`, `cmd/mcp-proxy-gateway/main.go`
  - Tests: `internal/proxy/stream_test.go`, `internal/proxy/origin_test.go`, `internal/proxy/proxy_test.go`
  - Docs/examples: `README.md`, `policy.example.yaml`, `CHANGELOG.md`, `PLAN.md`
  - Verification:
    - `make check` (pass)
    - `make smoke` (pass; includes upstream stub + origin rejection + SSE passthrough)
- Commit: `be511eb` (feature) + `f9a97f4` (smoke + trackers)
- Confidence: high
- Trust label: verified-local
- Follow-ups:
  - Expand the upstream header-forwarding allowlist (currently minimal) with explicit docs.
  - Decide on a safe stance for recording/replaying streaming responses (likely keep JSON-only).

## Entry: 2026-02-09 - Runtime Metrics Endpoint
- Decision: Add a lightweight `GET /metricsz` endpoint backed by in-process atomic counters inside `internal/proxy`.
- Why: Operators needed direct visibility into request volume, replay effectiveness, validation rejects, upstream failures, and latency distribution without external dependencies.
- Evidence:
  - Code: `internal/proxy/proxy.go`, `internal/proxy/proxy_test.go`, `scripts/smoke.sh`, `cmd/mcp-proxy-gateway/main.go`
  - Verification: `make check`, `make smoke`, manual local `/metricsz` request
- Commit: `14d378bdc545717ebea90b9bf35ff1a3bfc5d9ab`
- Confidence: high
- Trust label: verified-local
- Follow-ups:
  - Add recorder retention/rotation metrics once file lifecycle controls are implemented.
  - Consider Prometheus exposition format if deployment scope expands beyond local-first workflows.

## Entry: 2026-02-09 - Recorder Rotation/Retention Controls
- Decision: Add optional NDJSON recorder rotation/retention (`record.max_bytes` + `record.max_files`) with CLI overrides (`--record-max-bytes`, `--record-max-files`).
- Why: Long-lived local gateways can otherwise grow recordings without bound; rotation makes the default record path safe for repeated usage.
- Evidence:
  - Code: `internal/record/record.go`, `internal/config/config.go`, `cmd/mcp-proxy-gateway/main.go`
  - Tests: `internal/record/record_test.go`, `internal/proxy/proxy_test.go`
  - Docs/examples: `README.md`, `policy.example.yaml`, `CHANGELOG.md`, `PLAN.md`, `docs/PROJECT.md`
  - Verification:
    - `make check` (pass)
    - `make smoke` (pass)
- Commit: `a4fc153d20483e0e37f6b87ef5f7f96f47a4f9b2`
- Confidence: high
- Trust label: verified-local
- Follow-ups:
  - Consider adding a small smoke-path for recording with a stub upstream to exercise rotation end-to-end (optional).

## Entry: 2026-02-09 - Track Root AGENTS.md Contract
- Decision: Check in the repo-level `AGENTS.md` contract file (previously only `docs/AGENTS.md` was tracked).
- Why: Automation and maintenance loops expect a stable top-level contract; keeping it versioned prevents drift.
- Evidence:
  - Code: `AGENTS.md`
- Commit: `9a7273a65d19812b180708576e8d85e891ce6c11`
- Confidence: high
- Trust label: verified-local

## Entry: 2026-02-09 - Bounded Market Scan (MCP Gateways/Proxies)
- Decision: Track baseline expectations for MCP gateways and keep scope aligned with local-first goals (no auth by default).
- Why: Streaming transport support and basic security hardening expectations are evolving quickly; capturing the baseline helps prioritize parity gaps.
- Evidence (external, untrusted):
  - Streamable HTTP proxy patterns (stdio to Streamable HTTP): https://github.com/atrawog/mcp-streamablehttp-proxy
  - Auth/rate-limiting style proxy examples (out of scope for this repo’s local-first guardrails): https://github.com/sigbit/mcp-auth-proxy
  - “Gateway as middleware” patterns: https://github.com/microsoft/mcp-gateway, https://github.com/matthisholleville/mcp-gateway
  - OAuth gateway example (out of scope): https://github.com/atrawog/mcp-oauth-gateway
- Confidence: medium
- Trust label: untrusted-web
- Follow-ups:
  - Prioritize Streamable HTTP/SSE support as the next parity feature.
  - Add optional `Origin` allowlist hardening to reduce browser-initiated request risk when bound beyond localhost.

## Entry: 2026-02-10 - CI Fixes: /metrics Routing + CI Smoke
- Decision: Route by path first so disabled Prometheus exposition (`GET /metrics`) returns `404` (instead of `405`), and treat unknown endpoints as `404` while keeping `GET /rpc` as `405`.
- Why: CI was failing because `/metrics` was falling through the generic “POST-only” gate; the resulting behavior was confusing (and incorrectly suggested `/metrics` existed but disallowed methods). Path-first routing is the expected HTTP shape for this gateway.
- Evidence:
  - Code: `internal/proxy/proxy.go`
  - Tests: `internal/proxy/proxy_test.go` (`TestMetricsPromDisabledReturns404`, `TestUnknownPathReturns404`, `TestRPCEndpointWrongMethodReturns405`)
  - CI: GitHub Actions run `21836360939` failure resolved by `384189c`
  - Workflow hardening: run `make smoke` in CI; bump CI Go version to avoid implicit toolchain downloads for `golangci-lint`; make `scripts/smoke.sh` port- and tmp-safe.
  - Verification:
    - `go test ./...` (pass)
    - `make check` (pass)
    - `make smoke` (pass)
- Commit: `384189c` (routing fix) + `8d3bf04` (CI smoke + smoke hardening + CI Go bump + routing tests)
- Confidence: high
- Trust label: verified-local
- Follow-ups:
  - Add docs note for `/metrics` explicitly being “404 unless enabled” if user confusion recurs.

## Recent Decisions (2026-02-11)
- 2026-02-11 | Add `POST /mcp` compatibility alias with the same method guard and handler semantics as `POST /rpc` | Many MCP gateway clients/tools expect an `/mcp` endpoint shape; aliasing improves transport compatibility without changing core JSON-RPC behavior | `internal/proxy/proxy.go`, `internal/proxy/proxy_test.go`, `scripts/smoke.sh`, `README.md`, `docs/PROJECT.md` | `36b90fc` | high | trusted
- 2026-02-11 | Enforce strict SSE negotiation for single requests (`Accept: text/event-stream` required before SSE passthrough) | Prevents non-stream clients from unexpectedly receiving SSE payloads while preserving explicit streaming behavior | `internal/proxy/proxy.go`, `internal/proxy/stream_test.go`, `internal/proxy/proxy_test.go`, `README.md` | `36b90fc` | high | trusted
- 2026-02-11 | Add handler-level benchmark coverage for batch replay-hit and batch upstream proxy paths | Provides local performance baselines at the HTTP handler layer (beyond replay-store micro-benchmarks) | `internal/proxy/proxy_benchmark_test.go`, benchmark output below | `36b90fc` | high | trusted
- 2026-02-11 | Refresh bounded market expectations for MCP gateways (transport endpoint compatibility, explicit streaming negotiation, streamable HTTP trajectory) | Aligns near-term backlog with external baseline patterns while retaining local-first scope | Sources: https://github.com/matthisholleville/mcp-gateway, https://github.com/sigbit/mcp-auth-proxy, https://github.com/dgellow/mcp-front, https://github.com/IBM/mcp-context-forge, https://modelcontextprotocol.io/docs/concepts/transports | n/a | medium | untrusted

## Mistakes And Fixes (2026-02-11)
- Root cause: Single-request proxy logic streamed upstream SSE whenever upstream returned `Content-Type: text/event-stream`, even when client did not request streaming.
  Fix: Require client `Accept: text/event-stream` before SSE passthrough; otherwise return JSON-RPC upstream error.
  Prevention rule: Treat streaming as an explicit capability negotiation (client signal + upstream signal), not upstream-content-type-only.

## Verification Evidence (2026-02-11)
- `go test ./...` | pass
- `go test ./internal/proxy -run '^$' -bench 'BenchmarkServeHTTPBatch' -benchmem` | pass
- `make check` | pass
- `make smoke` | pass
- `gh issue list --limit 50 --json number,title,author,state,labels,updatedAt` | pass (no open issues)
- `gh run view 21836360939 --json ...` | pass (historical failure confirmed; now superseded by green runs)
