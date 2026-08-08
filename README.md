# llmm

`llmm` is a small, declarative manager for local LLM runtimes. A single YAML file describes runtimes and models; the CLI validates the machine and controls existing systemd user services or Docker containers.

It deliberately does **not** install CUDA, models, runtimes, systemd units, Docker, or Tailscale. Those are host concerns. `llmm` makes their state inspectable and repeatable without becoming another daemon.

## Install

```bash
go install github.com/magiodev/llmm/cmd/llmm@latest
```

Go 1.22 or newer is required to build from source.

## Quick start

```bash
llmm config init
$EDITOR ~/.config/llmm/config.yaml
llmm config validate
llmm doctor
llmm status
```

Use another manifest with `--config PATH` or `LLMM_CONFIG`.

## Commands

```text
llmm config init [--force]   create a minimal manifest
llmm config validate         validate syntax and references
llmm doctor [--deep]         check services, binaries and model files
llmm models                  list configured models
llmm status [runtime]        show runtime state
llmm start <runtime>
llmm stop <runtime>
llmm restart <runtime>
```

`doctor --deep` hashes model files and can take several minutes for large models. Normal `doctor` checks the configured size without rereading the entire file.

## Manifest

See [`examples/config.yaml`](examples/config.yaml). Runtime types:

- `systemd`: requires `service`; controlled with `systemctl --user`.
- `docker`: requires `container`; controlled with Docker.

Model entries are metadata and integrity checks. `llmm` does not invent backend-specific launch flags or duplicate a model downloader.

Configuration is written with mode `0600` because manifests may eventually reference private paths. Do not put credentials in the file; use the runtime's normal secret mechanism.

## Design

- One process per command; no daemon.
- YAML is the source of truth.
- Runtime lifecycle delegates to native supervisors.
- Strict parsing rejects misspelled fields.
- Minimal dependencies: Cobra and `yaml.v3`.

## Development

```bash
go test ./...
go vet ./...
go build ./cmd/llmm
```

## License

MIT
