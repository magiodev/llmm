# Phase 1 contract

This document freezes the rules the first implementation wave must follow. It is the Milestone 0 contract for the single-node LLM management CLI. Phase-1 issues are implemented without reopening the product boundary question.

## Product boundary

`llmm` is the authoritative CLI for **one** LLM node:

- declaring the node
- stocking the node with model artifacts
- verifying artifact integrity
- operating runtimes
- inspecting health and state
- exporting stable machine-readable state to trusted clients

Out of scope for the expanded product (non-goals):

- daemon or background reconciler
- cluster registry or fleet manager
- API proxy/gateway
- quantization/conversion/training pipelines
- discovery marketplace or browse/search product
- client onboarding logic inside `llmm`

## Artifact representation

Settled by #4. One logical served model is keyed once under `models`, with one runtime.

- `path` is the **primary served file** and is required.
- `artifacts` is an optional model-local list of additional files in a multi-file layout (shards, tokenizers, mmproj companions).
- Each artifact carries `path` (required), optional `size`, and optional `sha256`.
- The common single-file case keeps `artifacts` empty.
- `doctor` validates every declared artifact: regular-file existence, declared size, and (under `--deep`) SHA-256.

This representation is the foundation for future fetch/install/verify work. It does not itself download or install anything.

## Provenance and integrity

Settled by #20. `source` on a model records the provenance origin (e.g. an `owner/repo` model reference); it is additive, optional, and exported unchanged in `config show`.

- When `source` is present, `config validate` rejects whitespace, a leading `-`, and embedded credentials (`@`).
- A model may declare `size` and `sha256` to integrity-pin its artifact; `verify` and installed-state reporting consume these pins so drift between the declared digest and the on-disk artifact is detected and reported.

## Schema and versioning strategy

- **Additive changes** stay in version `1`: new optional fields that old binaries and old clients safely ignore (for example, `artifacts` and `default_model`).
- **Breaking changes** bump `version`, and old binaries reject the new version clearly instead of silently stretching version 1.
- Do not invent a fake aggregate path for sharded models; represent the real files.

## Machine vs human output contract

Settled by #6. `status`, `models`, and `doctor` accept `--format text|json` (default `text`).

- JSON output is deterministic (sorted names, fixed check order), color-free, and prose-free.
- **Failures exit non-zero even when emitting JSON**, so scripts rely on the exit code, not text parsing.
- Human text output stays stable even as JSON evolves; machine output is the contract.

### JSON shapes

`status --format json`:

```json
[{"name":"example","type":"systemd","state":"active"}]
```

`state` is `"error"` on supervisor failure (still non-zero exit).

`models --format json`:

```json
[{"name":"example-model","runtime":"example","path":"/models/example-model.gguf","context":262144,"output":8192,"default":true}]
```

`default` is present only when the model ID equals `default_model`.

`doctor --format json`:

```json
{"success":false,"checks":[{"ok":true,"label":"config","detail":"/path"},{"ok":false,"label":"model example-model","detail":"/path"}]}
```

## `config show --format json` compatibility

Trusted clients consume exported truth over SSH; they do not scrape local files.

- Exported config is **additive**: new fields are optional and ignorable by old clients.
- Clients read `.default_model` instead of hardcoding an alphabetical fallback.
- Clients may ignore artifact-install internals unless they become explicitly useful.
- `llm-client` remains a consumer of `config show --format json`; it must not become a second control plane.

## Default-model semantics

Settled by #5. `default_model` names the operator default by model ID and must reference an existing model.

- When set, clients and node-local workflows treat it as authoritative.
- When omitted, no default is declared; clients fall back to their own deterministic rule. llmm does not invent a default on the operator's behalf.

## Milestone 0 definition of done

Phase-1 issues can be implemented without reopening the product boundary question. This document records the settled rules; it is the source of truth for the implementation wave.
