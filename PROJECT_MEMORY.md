# PROJECT_MEMORY

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
- Commit: `be511eb` (feature); follow-up commit updates smoke + trackers
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
