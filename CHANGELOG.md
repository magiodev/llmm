# Changelog

All notable changes are documented here. This project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

- First-class model artifacts: multi-file layouts via a model-local `artifacts` list; `doctor` and `doctor --deep` now validate every declared artifact (path, size, SHA-256).
- Docs and discoverability: README overhaul, contributor/security/conduct guides, Makefile targets.
- CI: coverage gate enforcing statement, block (branch), and line thresholds.
- Tests: hardened branch coverage across `app`, `config`, and `runtime`.

## 0.1.0

- Initial `llmm` CLI: strict YAML manifest, `config init|validate|show`, `doctor` (+`--deep`), `status`, `start|stop|restart`, `models`, shell completion.
- Native systemd-user and Docker lifecycle adapters.
- Remote-client manifest export over SSH (YAML/JSON).
