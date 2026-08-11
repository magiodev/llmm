# Changelog

All notable changes are documented here. This project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

- `llmm install <model>`: fetch a declared model from its `source`, verify size/SHA-256, atomic publish, and record machine-managed installed state (`installed.yaml`). Downloads resume from `.part` on retry.
- Phase 1 contract freeze: `docs/contract.md` settles artifact representation, schema/versioning strategy, machine-vs-human output rules, `config show --format json` compatibility, default-model semantics, and explicit non-goals.
- First-class model artifacts: multi-file layouts via a model-local `artifacts` list; `doctor` and `doctor --deep` now validate every declared artifact (path, size, SHA-256).
- Explicit `default_model` in exported config: operator-declared default model ID, validated against `models`, emitted in YAML and JSON.
- Machine-readable output: `--format text|json` on `status`, `models`, and `doctor`; JSON is deterministic, color-free, and preserves non-zero exit on failures.
- First-run UX: README restructured around declare → stock → validate → export, a terminal demo sequence, and example manifests for common node shapes (single systemd, multi-systemd, Docker UI, multi-file models).
- Docs and discoverability: README overhaul, contributor/security/conduct guides, Makefile targets.
- CI: coverage gate enforcing statement, block (branch), and line thresholds.
- Tests: hardened branch coverage across `app`, `config`, and `runtime`.

## 0.1.0

- Initial `llmm` CLI: strict YAML manifest, `config init|validate|show`, `doctor` (+`--deep`), `status`, `start|stop|restart`, `models`, shell completion.
- Native systemd-user and Docker lifecycle adapters.
- Remote-client manifest export over SSH (YAML/JSON).
