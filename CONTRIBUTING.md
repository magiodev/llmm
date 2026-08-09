# Contributing to llmm

Thanks for helping make llmm the mainstream way to manage LLM machines. This file describes the contribution workflow and the quality bar.

## Workflow

1. Open an issue or discussion to propose a change before large work.
2. Fork the repo and create a branch: `feat/<thing>`, `fix/<thing>`, `chore/<thing>`, `docs/<thing>`.
3. Make small, reviewable changes. Prefer the smallest change that works.
4. Commit with a conventional message (e.g. `feat:`, `fix:`, `docs:`, `ci:`, `test:`).
5. Push and open a pull request against `main`. Reference any related issue.

## Coding standards

- Keep every file under 500 lines.
- Use the Go standard library over new dependencies. The project intentionally has only two direct dependencies (Cobra and yaml.v3); do not add more without discussion.
- No unnecessary abstractions, boilerplate, or commented-out code.
- Never weaken validation, error handling, or security paths (config writes, permissions, symlink handling, endpoint checks).
- Preserve strict input: unknown YAML fields must keep failing validation.

## Testing requirements

Coverage is enforced in CI at the branch level, not just lines and functions. Before opening a PR:

```bash
make test
make cover   # runs the branch/line/statement threshold gate
```

New logic must keep branch (block) coverage at 100%. If a branch is genuinely unreachable, say so in the PR and add a test that documents the invariant.

## Security

Do not add secrets, tokens, or credentials to the manifest or code. Report vulnerabilities privately via [SECURITY.md](SECURITY.md).

## Docs

Keep `docs/` short and current. Update docs when behavior, commands, or assumptions change.
