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

## Next
- Add recording redaction rules (denylist keys/regex).
- Consider batch JSON-RPC support.
