# Usage and operations

This guide covers installation, configuration selection, every command, normal operating workflows, shell completion, exit behavior, and troubleshooting.

## Output style

Command output follows a shared, calm convention so a first-run operator can parse it at a glance:

- **Sectioned checks.** `doctor` prints one labeled line per check (`ok`/`fail`) with a fixed-width label column.
- **Tabular listings.** `status` and `models` use fixed-width or tab-separated columns, so output is easy to read and diff.
- **Compact summaries.** Every command prints only what changed or matters; nothing is decorated with progress bars or banners.
- **Clear next steps.** Failure output names the object and the reason, and `doctor` reports every problem in one pass so you can fix them together.
- **Machine-readable stays boring.** `--format json` is deterministic and color-free; human output stays stable even as JSON evolves.

llmm intentionally avoids color and interactive full-screen modes. Keep the terminal surface boring and dependable; put richness in the docs and examples instead.

## Requirements

Building llmm requires Go 1.22 or newer. Runtime requirements depend on the manifest:

- `systemd` entries need a user systemd session and `systemctl`.
- `docker` entries need the Docker CLI and access to the Docker daemon.
- Remote consumption needs an SSH server on the node and an authenticated SSH client.

llmm does not install any of these prerequisites.

## Installation

### Install with Go

```bash
go install github.com/magiodev/llmm/cmd/llmm@latest
```

Go normally installs the binary into `$(go env GOPATH)/bin`. Ensure that directory is on `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

### Build from source

```bash
git clone https://github.com/magiodev/llmm.git
cd llmm
go test ./...
go build -o llmm ./cmd/llmm
mkdir -p ~/.local/bin
install -m 0755 llmm ~/.local/bin/llmm
```

### Cross-compile for Linux ARM64

This is useful when building on an x86-64 workstation for a GB10 or another ARM64 node:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -o llmm-linux-arm64 ./cmd/llmm
scp llmm-linux-arm64 dgx:/tmp/llmm
ssh dgx 'install -m 0755 /tmp/llmm "$HOME/.local/bin/llmm" && rm /tmp/llmm'
```

The current code does not require CGO.

### Verify the binary

```bash
llmm --version
llmm --help
```

A binary built without an injected version may report `dev`. Release or deployment builds can set the main package version with `-ldflags`.

## Config selection

llmm chooses the manifest in this order:

1. `--config PATH` on the command line;
2. `LLMM_CONFIG` in the environment;
3. the operating system's user config directory plus `llmm/config.yaml`.

On Linux, the normal default is:

```text
~/.config/llmm/config.yaml
```

Examples:

```bash
llmm --config ./lab.yaml status
LLMM_CONFIG=./lab.yaml llmm doctor
llmm status
```

`--config` is a persistent flag, so it goes before or after a subcommand:

```bash
llmm --config ./lab.yaml status
llmm status --config ./lab.yaml
```

## Create a manifest

```bash
llmm config init
```

The command:

- creates parent directories with mode `0700`;
- writes the manifest with mode `0600`, including when `--force` replaces a file with broader permissions;
- writes through a same-directory temporary file and atomically installs it;
- rejects symlink destinations;
- validates the starter before writing it;
- refuses to replace an existing file.

To deliberately replace an existing manifest:

```bash
llmm config init --force
```

`--force` destroys the existing manifest at the selected path. Review `--config` and `LLMM_CONFIG` first.

The generated manifest is intentionally minimal. Edit it to describe services, containers, and model files that already exist.

## Validate configuration

```bash
llmm config validate
```

Validation checks:

- YAML syntax;
- unknown fields;
- schema version;
- presence of at least one runtime;
- supported runtime types;
- required `service` or `container` fields;
- model-to-runtime references;
- required model paths;
- one YAML document only;
- non-negative model limits and valid SHA-256 syntax;
- absolute credential-free endpoints;
- supervisor-specific fields and identifiers.

It does not inspect the host. Use `doctor` for that.

A successful result is:

```text
config: ok
```

Any validation problem produces a non-zero exit. Several semantic problems are sorted and returned together so they can be fixed in one pass.

## Export normalized configuration

YAML:

```bash
llmm config show
```

JSON:

```bash
llmm config show --format json
```

The command parses and validates before producing output. It therefore doubles as a safe consumer boundary: downstream clients never need to parse an unchecked source file.

Unsupported formats fail explicitly:

```bash
llmm config show --format toml
# unsupported format "toml" (use yaml or json)
```

Typical JSON queries:

```bash
llmm config show --format json | jq -r '.node'
llmm config show --format json | jq -r '.runtimes.example.endpoint'
llmm config show --format json | jq -r '.models | keys[]'
llmm config show --format json | jq '.models["example-model"] | {context, output}'
```

## Inspect runtime status

Show every runtime:

```bash
llmm status
```

Show one runtime:

```bash
llmm status example
```

Example:

```text
example              active
open-webui       running
```

Status meanings come from the native supervisor:

- systemd: output of `systemctl --user is-active`;
- Docker: `.State.Status` from `docker inspect`;
- supervisor errors or missing objects: command failure and a non-zero llmm exit.

`active` or `running` means the supervisor sees a live workload. It does not prove that a model has finished loading or that an HTTP endpoint is healthy.

### Machine-readable output

`status`, `models`, and `doctor` accept `--format text|json` (default `text`). JSON output is deterministic, color-free, and free of prose; it is meant for automation. Failures still produce a non-zero exit even when emitting JSON, so scripts can rely on the exit code rather than parsing text.

`status --format json` emits an array of runtime objects:

```json
[{"name":"example","type":"systemd","state":"active"}]
```

`state` is the supervisor's reported state, or `"error"` when the supervisor call fails (and llmm still exits non-zero).

## Control runtimes

```bash
llmm start example
llmm stop example
llmm restart example
```

For systemd entries, llmm executes:

```text
systemctl --user <action> <service>
```

For Docker entries, it executes:

```text
docker <action> <container>
```

The runtime name must exist in the manifest. Shell completion can suggest valid names.

Errors include the failed native command and its combined output. Supervisor commands time out after 30 seconds. llmm does not retry or hide supervisor failures.

### Readiness after start or restart

Model loading may take minutes. Check supervisor state first, then probe the actual API:

```bash
llmm restart example
llmm status example

until curl -fsS http://dgx:8001/v1/models >/dev/null; do
  sleep 5
done
```

Use the runtime's real readiness endpoint when it provides one. Do not route client traffic based only on `llmm status`.

## List models

```bash
llmm models
```

Output is tab-separated:

```text
example-model	example	/models/example-model.gguf
```

The columns are:

1. model ID;
2. configured runtime;
3. local artifact path.

Models are sorted by ID for stable output.

## Install a model

A declared model with a `source` URL can be fetched onto the node:

```bash
llmm install <model>
```

llmm downloads the artifact to the model's declared `path`, verifies the declared `size` and `sha256` when present, then atomically publishes it (temp file + rename) so a partial or failed download never leaves half-written state at the final path. On success it records the installed origin and integrity in a machine-managed `installed.yaml` next to your manifest. That state file is separate from the human-edited manifest and is additive and versioned.

`source` must be an absolute `http` or `https` URL without embedded credentials. Models without a `source` cannot be installed.

`models --format json` emits an array of model objects with the primary path, advertised limits, a `default` marker when the model ID equals `default_model`, and any declared artifacts:

```json
[{"name":"example-model","runtime":"example","path":"/models/example-model.gguf","context":262144,"output":8192,"default":true}]
```
## Run diagnostics

```bash
llmm doctor
```

The normal doctor checks:

- the manifest loaded and validated;
- each declared executable is a regular file with an execute bit;
- each systemd service is loaded;
- each declared Docker container is inspectable;
- each model path exists and is a regular file;
- declared model size matches the file size.

Example:

```text
ok    config                   /home/alice/.config/llmm/config.yaml
ok    runtime example              /opt/example/example-server
ok    service example              example.service
ok    docker open-webui        open-webui
ok    model example-model  /models/example-model.gguf (86720111488 bytes)
```

Failed checks use `fail` and produce a non-zero exit after all checks run.

### Deep integrity check

```bash
llmm doctor --deep
```

For each model with `sha256`, this reads the complete file and compares the digest. Large files can take time and consume substantial storage bandwidth; a single hash is capped at 30 minutes. The normal doctor checks size without hashing the whole artifact.

Deep mode does not fail models that omit `sha256`; it simply has no digest to verify.

`doctor --format json` emits an object with an overall `success` boolean and an explicit `checks` array, each with `ok`, `label`, and `detail`:

```json
{"success":false,"checks":[{"ok":true,"label":"config","detail":"/home/alice/.config/llmm/config.yaml"},{"ok":false,"label":"model example-model","detail":"/models/example-model.gguf"}]}
```

A failing doctor still exits non-zero even in JSON mode.

### What doctor does not check

Doctor does not:

- send HTTP requests to `endpoint`;
- assert that services are active;
- inspect GPU or unified memory;
- validate backend-specific model compatibility;
- install missing commands;
- inspect independent tools such as Model Shelf;
- check credentials.

Pair doctor with `status` and a real API probe when validating a deployment.

## Quiet mode

The global `--quiet` or `-q` flag suppresses confirmation output for commands that explicitly support it, including successful config initialization, validation, and lifecycle actions. Errors still print and retain non-zero exits.

Commands whose main purpose is output, such as `status`, `models`, `doctor`, and `config show`, continue to print their results.

Examples:

```bash
llmm -q config validate
llmm -q restart example
```

## Shell completion

Cobra provides completion for Bash, Zsh, Fish, and PowerShell:

```bash
source <(llmm completion bash)
llmm completion zsh > ~/.zfunc/_llmm
llmm completion fish > ~/.config/fish/completions/llmm.fish
```

Create the destination directory first for persistent files. Runtime action completion reads the selected manifest and suggests configured runtime names.

## Operational workflows

### Validate after editing

```bash
llmm config validate
llmm doctor
llmm status
```

Then probe every endpoint that matters.

### Add an existing runtime

1. Install and verify the runtime independently.
2. Create its systemd user service or Docker container.
3. Add the runtime to `runtimes`.
4. Add any model metadata to `models`.
5. Run `config validate` and `doctor`.
6. Start or restart through llmm.
7. Probe the API.

### Move a model file

1. Stop or otherwise make the runtime safe to reconfigure.
2. Move the artifact.
3. Update the runtime's native configuration.
4. Update `models.<id>.path` and, if needed, `size` and `sha256`.
5. Run `llmm doctor --deep`.
6. Start the runtime and probe readiness.

llmm does not modify native service files for you.

### Use a temporary manifest

```bash
cp ~/.config/llmm/config.yaml /tmp/llmm-test.yaml
$EDITOR /tmp/llmm-test.yaml
llmm --config /tmp/llmm-test.yaml config validate
llmm --config /tmp/llmm-test.yaml doctor
```

Do not run lifecycle commands against a temporary manifest until you have verified every target service or container.

## Troubleshooting

### `read config: no such file or directory`

Check the selected path:

```bash
printf '%s\n' "${LLMM_CONFIG:-$HOME/.config/llmm/config.yaml}"
llmm config init
```

Also check whether a global `--config` flag is pointing elsewhere.

### `field ... not found in type config.Config`

The parser is strict. Remove the unknown field or use the schema version that supports it. This usually catches a typo or documentation drift.

### `version must be 1`

The file targets a schema version unsupported by this binary. Upgrade the binary or migrate the manifest deliberately. Do not change the number without checking field compatibility.

### `runtime ... requires service`

A `systemd` runtime needs `service`. A `docker` runtime needs `container`.

### `model ... references unknown runtime`

The model's `runtime` value must exactly match a key under `runtimes`.

### Status says `inactive` but the service exists

Run the native supervisor command for detail:

```bash
systemctl --user status example.service
docker inspect open-webui
```

llmm keeps status output compact but returns native supervisor failures instead of presenting them as normal inactivity.

### Service is active but the API fails

The process may still be loading its model, may have bound a different address, or may have failed after systemd considered it started. Inspect logs and probe the declared endpoint:

```bash
journalctl --user -u example.service -n 100 --no-pager
curl -fsS http://dgx:8001/v1/models
```

### Docker doctor fails

Doctor inspects the named container. Use the native command to distinguish a missing CLI, daemon failure, permission problem, or missing container:

```bash
llmm status open-webui
docker inspect open-webui
```

### Deep checksum fails

Verify that the configured digest belongs to the exact artifact:

```bash
sha256sum /models/example-model.gguf
```

A size match is not a checksum match. Update the manifest only after confirming that the new artifact is intentional.

## Exit behavior for scripts

Use command exit status rather than parsing human text when deciding success:

```bash
if llmm config validate >/dev/null; then
  echo valid
else
  echo invalid >&2
  exit 1
fi
```

`config show --format json` is the preferred input for programs. Human-oriented `status`, `models`, and `doctor` output may evolve.

For remote consumption and multi-node examples, continue with [remote-clients.md](remote-clients.md). For every manifest field, see [manifest.md](manifest.md).
