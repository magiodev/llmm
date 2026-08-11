# Changelog

All notable changes are documented here. This project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

- `llmm install <model>`: fetch a declared model from its `source`, verify size/SHA-256, atomic publish, and record machine-managed installed state (`installed.yaml`).
- Docs and discoverability: README overhaul, contributor/security/conduct guides, Makefile targets.
- CI: coverage gate enforcing statement, block (branch), and line thresholds.
- Tests: hardened branch coverage across `app`, `config`, and `runtime`.

## 0.1.0

- Initial `llmm` CLI: strict YAML manifest, `config init|validate|show`, `doctor` (+`--deep`), `status`, `start|stop|restart`, `models`, shell completion.
- Native systemd-user and Docker lifecycle adapters.
- Remote-client manifest export over SSH (YAML/JSON).
