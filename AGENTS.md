# AGENTS.md — llmm conventions

Guidance for AI agents and contributors working in this repository.

## Commands

```bash
make fmt      # gofmt -w cmd internal
make test     # go test ./...
make vet      # go vet ./...
make build    # go build ./cmd/llmm
make cover    # go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

## Architecture

- `cmd/llmm/` — thin executable entrypoint; keep logic out of `main()`.
- `internal/app/` — Cobra commands and diagnostics.
- `internal/config/` — schema, strict parser, validation, serialization.
- `internal/runtime/` — systemd and Docker lifecycle adapters.
- `docs/` and `examples/` — user-facing guides and a portable manifest.

## Rules

- Two direct dependencies only: Cobra and yaml.v3. Prefer stdlib.
- Keep files under 500 lines.
- Validation, error handling, and security paths (0600 writes, symlink rejection, leading-dash supervisor args) are non-negotiable — never weaken them.
- CI enforces statement, block (branch), and line coverage; new logic must keep branch coverage at 100%.
- Commit immediately after a change; never leave work uncommitted.
