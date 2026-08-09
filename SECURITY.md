# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| latest `main` | ✅ |

llmm is a small, security-sensitive CLI: it reads a user manifest, checks host files, and drives systemd-user and Docker supervisors. Treat it as trusted-operator software.

## Reporting a vulnerability

Do **not** open a public issue for a security problem. Report privately by email to the maintainers at the address listed in the repository metadata, or via a GitHub security advisory:

1. Describe the issue: affected command, manifest shape, reproduction steps, and impact.
2. Include any relevant logs or minimal repro manifests.
3. Expect a reply within 5 business days.

Public disclosure only after the maintainers confirm the fix and release it.

## Security model

The full model is in [README.md](README.md#security-model). Key rules:

- Keep the manifest mode `0600`.
- Never put API keys, tokens, passwords, or private connection strings in the manifest.
- Keep secrets in the runtime's native mechanism (systemd credentials, protected env files, a secret manager).
- Use a private network (Tailscale) for remote endpoints.
- Give each client its own SSH key.
- `config show` output is operational metadata — transport it over authenticated SSH.
- Anyone who can read the manifest and reach its systemd-user manager or Docker daemon can control those workloads through llmm. Review lifecycle access carefully.

## Scope

- The config write path protects against symlink replacement and restores private permissions.
- Validation rejects unknown fields, credential-bearing endpoints, and leading-dash supervisor arguments.
- `doctor --deep` verifies model SHA-256 integrity.

Out-of-scope: model-server API security, CUDA/driver hardening, and host-level access control. Those are the responsibility of the runtime and the machine operator.
