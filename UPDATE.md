# UPDATE

## 2026-02-01
- Establish root `PLAN.md` / `CHANGELOG.md` / `UPDATE.md` for repo-level memory and release notes.
- Ship reliability + ergonomics pass:
  - `/healthz` endpoint
  - graceful shutdown on SIGINT/SIGTERM
  - safer request/response size limiting
  - replay loader supports large lines
  - upstream requests honor client cancellation
  - `make lint` auto-installs `golangci-lint`
- Add optional recording redaction config (`policy.record`) to reduce risk of persisting secrets in NDJSON.
- Add JSON-RPC batch support (sequential per-item processing).
- Add configurable replay match modes (`policy.replay.match`).

## 2026-02-09
- Correct JSON-RPC notification handling for single requests: gateway now returns `204 No Content` when `id` is omitted.
- Correct replay correlation by rewriting replayed response `id` to the current request `id` for both single and batch flows.
- Add proxy regression tests for:
  - single notification passthrough behavior
  - replay hit notification suppression
  - replay response ID remapping (single and batch)
- Extend smoke test to assert notification path returns `204` with an empty body.
- Refresh stale docs checklist state in `docs/PLAN.md`.

## Next
- Streaming/SSE proxy support.
- Recorder file rotation or max-size safeguards.
- Lightweight request metrics endpoint.
