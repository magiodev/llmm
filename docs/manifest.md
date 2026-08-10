# Manifest reference

The llmm manifest is a versioned declaration of one node's LLM runtimes and model artifacts. It is local to the node, strict, and safe to export as operational metadata when it contains no secrets.

## Complete example

```yaml
version: 1
node: dgx

runtimes:
  example:
    type: systemd
    service: example.service
    executable: /opt/example/example-server
    endpoint: http://dgx:8001/v1

  vllm:
    type: systemd
    service: vllm-server.service
    executable: /opt/vllm/bin/vllm
    endpoint: http://dgx:9000/v1

  open-webui:
    type: docker
    container: open-webui
    endpoint: http://dgx:8080

models:
  example-model:
    runtime: example
    format: gguf
    path: /models/example-model.gguf
    source: owner/example-model-gguf
    sha256: ca22ae2f838e14077c22bc1c1417b71b45b5e5a3687bd96c2ac6e17fdb6261c0
    size: 86720111488
    context: 262144
    output: 8192
  vision-model:
    runtime: vllm
    format: safetensors
    path: /models/vision/model-00001-of-00002.safetensors
    context: 131072
    output: 8192
    artifacts:
      - path: /models/vision/model-00002-of-00002.safetensors
      - path: /models/vision/tokenizer.json
        sha256: ca22ae2f838e14077c22bc1c1417b71b45b5e5a3687bd96c2ac6e17fdb6261c0
      - path: /models/vision/mmproj.mmap
```

## Top-level fields

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `version` | integer | yes | Manifest schema version. The current binary accepts `1`. |
| `node` | string | no | Stable logical node identity, such as `dgx` or `dgx-2`. |
| `runtimes` | map | yes | Named runtime declarations. At least one entry is required. |
| `models` | map | no | Named model metadata. Omission or an empty map normalizes to an empty map. |

Unknown fields are rejected at every level.

## `version`

```yaml
version: 1
```

Versioning protects clients from silently interpreting a changed schema. llmm refuses any version other than the one compiled into the binary.

Do not increment this value to make an error disappear. Upgrade the binary or migrate the manifest according to the release that introduced the new version.

## `node`

```yaml
node: dgx
```

`node` is the logical identity exposed to clients. It should be:

- stable across operating system reinstalls;
- independent of a hardware SKU;
- unique within the fleet;
- suitable as an SSH alias and, when available, a Tailnet DNS name.

Good names:

```text
dgx
dgx-2
dgx-berlin
dgx-lab
```

Hardware descriptions such as `asus-ascent-gb10` or `dgx-spark` belong in inventory systems, not in the stable identity clients depend on.

The field is currently descriptive. llmm exports it but does not compare it with the operating system hostname.

## `runtimes`

`runtimes` is a map keyed by the operator-facing runtime name:

```yaml
runtimes:
  example:
    type: systemd
    service: example.service
    executable: /opt/example/example-server
    endpoint: http://dgx:8001/v1
```

Runtime names:

- are used by `start`, `stop`, `restart`, and `status`;
- are suggested by shell completion;
- are referenced by models;
- should remain stable even when an executable path changes.
- must not be empty.

### Runtime fields

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `type` | string | yes | Native supervisor type: `systemd` or `docker`. |
| `service` | string | for `systemd` | User systemd unit name; invalid for Docker and may not start with `-`. |
| `container` | string | for `docker` | Existing Docker container name or ID; invalid for systemd and may not start with `-`. |
| `executable` | string | no | Host executable expected by `doctor`. |
| `endpoint` | string | no | Advertised client endpoint. Exported but not probed. |

### systemd runtime

```yaml
runtimes:
  example:
    type: systemd
    service: example.service
    executable: /home/alice/src/example/example-server
    endpoint: http://dgx:8001/v1
```

Lifecycle maps to user services:

```text
systemctl --user start example.service
systemctl --user stop example.service
systemctl --user restart example.service
```

Doctor checks that:

- `executable`, when supplied, is a regular file with an execute bit;
- systemd reports the unit's `LoadState` as `loaded`.

Status uses `systemctl --user is-active`.

llmm does not create, enable, edit, or daemon-reload units. It also does not assume a system service when the manifest says `systemd`; all commands use `--user`.

### Docker runtime

```yaml
runtimes:
  open-webui:
    type: docker
    container: open-webui
    endpoint: http://dgx:8080
```

Lifecycle maps to:

```text
docker start open-webui
docker stop open-webui
docker restart open-webui
```

Doctor verifies the configured container with `docker inspect`. Status reports `.State.Status`. Both fail when the CLI, daemon, permissions, or container are unavailable.

llmm does not create containers, run Compose, pull images, or edit container configuration.

### `executable`

```yaml
executable: /opt/example/example-server
```

This optional field gives doctor a concrete host prerequisite to verify. Use an absolute path. For an interpreter-based runtime, point to the executable the service actually launches.

The field does not change how lifecycle commands work. systemd or Docker remains authoritative for launch arguments.

### `endpoint`

```yaml
endpoint: http://dgx:8001/v1
```

`endpoint` tells clients where the runtime is intended to be reached. When present, it must be an absolute URL without embedded credentials. llmm serializes it in YAML and JSON but does not:

- send a health request;
- add authentication;
- open a firewall;
- create a Tailscale Serve mapping;
- proxy traffic.

For a remotely consumed manifest, advertise a private address clients can resolve. A Tailnet DNS name is usually better than `127.0.0.1`. Keep loopback endpoints only when all consumers run on the same node or deliberately create their own SSH tunnel.

Do not embed credentials in the URL.

## `models`

`models` is a map keyed by the stable served model ID. Model IDs must not be empty:

```yaml
models:
  example-model:
    runtime: example
    format: gguf
    path: /models/example-model.gguf
    source: owner/example-model-gguf
    sha256: ca22ae2f838e14077c22bc1c1417b71b45b5e5a3687bd96c2ac6e17fdb6261c0
    size: 86720111488
    context: 262144
    output: 8192
```

### Model fields

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `runtime` | string | yes | Key under `runtimes` that serves the model. |
| `format` | string | no | Artifact format such as `gguf` or `safetensors`. |
| `path` | string | yes | Local artifact path checked by doctor. |
| `artifacts` | list of artifacts | no | Additional files that must exist and be valid (companions, shards). |
| `source` | string | no | Human-readable source repository or provenance. |
| `sha256` | string | no | Expected 64-character hexadecimal digest used by `doctor --deep`. |
| `size` | integer | no | Expected bytes used by normal doctor; may not be negative. |
| `context` | integer | no | Advertised context-window limit; may not be negative. |
| `output` | integer | no | Advertised output-token limit; may not be negative. |
| `reasoning` | list of strings | no | Advertised reasoning levels (e.g. `[none, high]`). Entries must be non-empty strings. |

### `runtime`

```yaml
runtime: example
```

This value must exactly match a runtime key. Validation fails when it does not.

The reference is metadata; llmm does not inject model paths into services or switch a service's active model.

### `format`

```yaml
format: gguf
```

`format` is descriptive. The current schema accepts any string and does not enforce format-specific rules.

### `path`

```yaml
path: /models/example-model.gguf
```

Doctor requires this path to exist and be a regular file. FIFOs, sockets, devices, and directories fail. Prefer absolute paths so behavior does not depend on the working directory.

For a single-file model, `path` is the entire layout. For a multi-file model, `path` is the primary served file and `artifacts` lists the rest of the set. Do not invent a fake aggregate path.

### `artifacts`

```yaml
artifacts:
  - path: /models/vision/model-00002-of-00002.safetensors
  - path: /models/vision/tokenizer.json
    sha256: ca22ae2f838e14077c22bc1c1417b71b45b5e5a3687bd96c2ac6e17fdb6261c0
  - path: /models/vision/mmproj.mmap
```

`artifacts` is an optional list of additional files that belong to a multi-file model layout, such as sharded GGUF parts, safetensors companions, tokenizers, or an mmproj companion. It is empty for the common single-file case.

Each artifact carries:

| Field | Type | Required | Meaning |
|---|---:|---|
| `path` | string | yes | Local path checked by doctor; must not be empty. |
| `sha256` | string | no | Expected 64-character hexadecimal digest used by `doctor --deep`. |
| `size` | integer | no | Expected bytes used by normal doctor; may not be negative. |

Doctor checks that every artifact exists as a regular file, verifies its declared size, and under `--deep` hashes and compares its SHA-256. An artifact is independent of the model's own `path`, `sha256`, and `size` fields, which describe the primary served file.

This representation is the foundation for future node-local fetch, install, and prune workflows. It does not itself download or install anything.

### `source`

```yaml
source: owner/example-model-gguf
```

Use this for provenance that helps an operator identify the artifact. It may be a repository ID, project name, or release reference. It is not used to download anything.

### `sha256`

```yaml
sha256: ca22ae2f838e14077c22bc1c1417b71b45b5e5a3687bd96c2ac6e17fdb6261c0
```

Validation requires exactly 64 hexadecimal characters. `doctor --deep` hashes the complete file and compares a lowercase digest. Normal doctor does not hash model files.

A checksum is useful when:

- model filenames are not globally unique;
- artifacts are moved between disks;
- clients need evidence that a node serves the intended build;
- a large download may have been interrupted or corrupted.

Never copy a digest from an untrusted source without verifying provenance.

### `size`

```yaml
size: 86720111488
```

Size is bytes, not a human-readable string. Normal doctor compares this value with the regular file's actual size. It is a cheap integrity signal, not a replacement for a checksum.

Set it with a trusted tool after the artifact is complete:

```bash
stat -c %s /models/example-model.gguf   # Linux
stat -f %z /models/example-model.gguf   # macOS
```

### `context` and `output`

```yaml
context: 262144
output: 8192
```

These are advertised token limits for clients. llmm exports them but does not configure or verify the runtime's launch flags.

Keep them aligned with the live server. A client that trusts an inflated context limit may submit a request the runtime cannot serve.

### `reasoning`

```yaml
reasoning: [none, high]
```

Advertised reasoning levels for clients, passed through verbatim. Clients use them to present or cycle reasoning modes; llmm does not interpret the values. Entries must be non-empty strings.

## Minimal valid manifest

```yaml
version: 1
runtimes:
  example:
    type: systemd
    service: example.service
models: {}
```

`node` is optional and `models` may be empty, but `runtimes` must contain at least one entry.

## Multiple models on one runtime

```yaml
version: 1
node: dgx
runtimes:
  vllm:
    type: systemd
    service: vllm-server.service
    endpoint: http://dgx:9000/v1
models:
  model-a:
    runtime: vllm
    format: safetensors
    path: /models/model-a/model.safetensors
    context: 32768
    output: 4096
  model-b:
    runtime: vllm
    format: safetensors
    path: /models/model-b/model.safetensors
    context: 65536
    output: 8192
```

This declares that the runtime is associated with both models. It does not prove that both are concurrently served, and it does not switch between them. The native service configuration remains authoritative.

## Separate nodes

Each node owns a separate manifest:

```yaml
# node one
version: 1
node: dgx
runtimes: { ... }
models: { ... }
```

```yaml
# node two
version: 1
node: dgx-2
runtimes: { ... }
models: { ... }
```

Do not combine host-local paths from several machines into one manifest. Clients can fetch and merge normalized JSON when they need a fleet view.

## Security rules

The manifest should contain operational metadata, not secrets.

Do not add:

- API keys;
- bearer tokens;
- passwords;
- private keys;
- Hugging Face credentials;
- secret-bearing URLs;
- protected environment values.

Use runtime-native secret handling. The manifest should be mode `0600` because paths and topology may still be private even without credentials. `llmm config init` enforces that mode for new and force-replaced manifests.

## Validation examples

Unknown top-level field:

```yaml
version: 1
cluster: lab
```

Result:

```text
field cluster not found in type config.Config
```

Unsupported runtime:

```yaml
runtimes:
  example:
    type: process
```

Result includes:

```text
runtime "example" has unsupported type "process"
```

Broken model reference:

```yaml
runtimes:
  example:
    type: systemd
    service: example.service
models:
  flash:
    runtime: missing
    path: /models/flash.gguf
```

Result includes:

```text
model "flash" references unknown runtime "missing"
```

## Normalization

`llmm config show` decodes, validates, and re-encodes the manifest. Comments and original formatting are not preserved in the output. Map ordering follows the serializer and should not be treated as an API contract.

Programs should consume JSON fields by key:

```bash
llmm config show --format json | jq -r '.models | keys[]'
```

Do not parse human-facing command tables when normalized JSON contains the same facts.
