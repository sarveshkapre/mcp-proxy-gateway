# PROJECT_MEMORY

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
