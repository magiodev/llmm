# Manifest reference

The llmm manifest is a versioned declaration of one node's LLM runtimes and model artifacts. It is local to the node, strict, and safe to export as operational metadata when it contains no secrets.

## Complete example

```yaml
version: 1
node: dgx

runtimes:
  ds4:
    type: systemd
    service: ds4-server.service
    executable: /opt/ds4/ds4-server
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
  deepseek-v4-flash:
    runtime: ds4
    format: gguf
    path: /models/deepseek-v4-flash.gguf
    source: antirez/deepseek-v4-gguf
    sha256: ca22ae2f838e14077c22bc1c1417b71b45b5e5a3687bd96c2ac6e17fdb6261c0
    size: 86720111488
    context: 262144
    output: 8192
```

## Top-level fields

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `version` | integer | yes | Manifest schema version. The current binary accepts `1`. |
| `node` | string | no | Stable logical node identity, such as `dgx` or `dgx-2`. |
| `runtimes` | map | yes | Named runtime declarations. At least one entry is required. |
| `models` | map | yes | Named model metadata. An empty map is valid. |

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
  ds4:
    type: systemd
    service: ds4-server.service
    executable: /opt/ds4/ds4-server
    endpoint: http://dgx:8001/v1
```

Runtime names:

- are used by `start`, `stop`, `restart`, and `status`;
- are suggested by shell completion;
- are referenced by models;
- should remain stable even when an executable path changes.

### Runtime fields

| Field | Type | Required | Meaning |
|---|---|---:|---|
| `type` | string | yes | Native supervisor type: `systemd` or `docker`. |
| `service` | string | for `systemd` | User systemd unit name. |
| `container` | string | for `docker` | Existing Docker container name or ID. |
| `executable` | string | no | Host executable expected by `doctor`. |
| `endpoint` | string | no | Advertised client endpoint. Exported but not probed. |

### systemd runtime

```yaml
runtimes:
  ds4:
    type: systemd
    service: ds4-server.service
    executable: /home/alice/src/ds4/ds4-server
    endpoint: http://dgx:8001/v1
```

Lifecycle maps to user services:

```text
systemctl --user start ds4-server.service
systemctl --user stop ds4-server.service
systemctl --user restart ds4-server.service
```

Doctor checks that:

- `executable`, when supplied, exists and is not a directory;
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

Doctor checks that the Docker CLI is available. Status checks the configured container with `docker inspect` and reports `.State.Status`.

llmm does not create containers, run Compose, pull images, or edit container configuration.

### `executable`

```yaml
executable: /opt/ds4/ds4-server
```

This optional field gives doctor a concrete host prerequisite to verify. Use an absolute path. For an interpreter-based runtime, point to the executable the service actually launches.

The field does not change how lifecycle commands work. systemd or Docker remains authoritative for launch arguments.

### `endpoint`

```yaml
endpoint: http://dgx:8001/v1
```

`endpoint` tells clients where the runtime is intended to be reached. llmm serializes it in YAML and JSON but does not:

- validate the URL;
- send a health request;
- add authentication;
- open a firewall;
- create a Tailscale Serve mapping;
- proxy traffic.

For a remotely consumed manifest, advertise a private address clients can resolve. A Tailnet DNS name is usually better than `127.0.0.1`. Keep loopback endpoints only when all consumers run on the same node or deliberately create their own SSH tunnel.

Do not embed credentials in the URL.

## `models`

`models` is a map keyed by the stable served model ID:

```yaml
models:
  deepseek-v4-flash:
    runtime: ds4
    format: gguf
    path: /models/deepseek-v4-flash.gguf
    source: antirez/deepseek-v4-gguf
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
| `source` | string | no | Human-readable source repository or provenance. |
| `sha256` | string | no | Expected digest used by `doctor --deep`. |
| `size` | integer | no | Expected bytes used by normal doctor. |
| `context` | integer | no | Advertised context-window limit. |
| `output` | integer | no | Advertised output-token limit. |

### `runtime`

```yaml
runtime: ds4
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
path: /models/deepseek-v4-flash.gguf
```

Doctor requires this path to exist and be a regular file. Prefer absolute paths so behavior does not depend on the working directory.

For sharded models, version 1 expects one path per model entry and has no shard-set abstraction. Do not invent a fake aggregate path. Either point to a concrete artifact used by your runtime or omit that model until the schema supports the real representation.

### `source`

```yaml
source: antirez/deepseek-v4-gguf
```

Use this for provenance that helps an operator identify the artifact. It may be a repository ID, project name, or release reference. It is not used to download anything.

### `sha256`

```yaml
sha256: ca22ae2f838e14077c22bc1c1417b71b45b5e5a3687bd96c2ac6e17fdb6261c0
```

`doctor --deep` hashes the complete file and compares a lowercase digest. Normal doctor does not hash model files.

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
stat -c %s /models/deepseek-v4-flash.gguf   # Linux
stat -f %z /models/deepseek-v4-flash.gguf   # macOS
```

### `context` and `output`

```yaml
context: 262144
output: 8192
```

These are advertised token limits for clients. llmm exports them but does not configure or verify the runtime's launch flags.

Keep them aligned with the live server. A client that trusts an inflated context limit may submit a request the runtime cannot serve.

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

Use runtime-native secret handling. The manifest is mode `0600` because paths and topology may still be private even without credentials.

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
  ds4:
    type: process
```

Result includes:

```text
runtime "ds4" has unsupported type "process"
```

Broken model reference:

```yaml
runtimes:
  ds4:
    type: systemd
    service: ds4.service
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
