# AGENTS

## Guardrails
- Keep the gateway local-first and single-user; no auth.
- Preserve deterministic replay behavior.
- Do not add heavy dependencies unless required.

## Commands
- `make setup` — install tooling deps
- `make dev` — run gateway locally
- `make test` — run tests
- `make lint` — lint (golangci-lint)
- `make typecheck` — go vet
- `make build` — build binary
- `make check` — run all quality gates
