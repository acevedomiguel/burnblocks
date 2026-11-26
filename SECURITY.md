# Security Policy

## Reporting a vulnerability

Do not open a public issue for a security problem. Use GitHub's private
vulnerability reporting on this repository (Security tab, "Report a
vulnerability"). If that is unavailable, contact the maintainer listed in
`go.mod` / `LICENSE`.

Include:

- Affected version (`burnblocks --version`) and platform, or the consuming
  application and version.
- Steps to reproduce, and whether the issue is remotely triggerable.
- Any proof of concept, with real tokens replaced by placeholders.

Expected timeline: acknowledgement within five business days, an assessment
within ten, and a coordinated disclosure date once a fix is ready. Credit is
given in the release notes unless you ask to stay anonymous.

## Scope

In scope:

- Credential handling: leakage into config, cache, JSON output, logs, error
  messages, or crash output.
- Unsafe fallback-file permissions or acceptance of group/world-readable files.
- Redaction failures in the shared redaction helper.
- Transport problems: accepting non-TLS base URLs, following untrusted
  redirects, unbounded response reads.
- Release supply chain: workflow permissions, action pinning, artifact or
  checksum substitution.

Out of scope:

- Vulnerabilities in OpenRouter's or Kimi's own APIs or consoles.
- Issues that require an attacker who already controls your user account or
  shell, or already holds your credential.
- Rate-limit or billing disputes handled by a provider's support team.
- Missing hardening that has no demonstrated impact (for example absent
  certificate pinning).

## Security design

Burn Blocks is deliberately narrow.

- **Read-only.** OpenRouter access is limited to `GET /credits`, `GET /key`,
  `GET /analytics/meta`, and the read-only `POST /analytics/query`. Kimi access
  is limited to `GET /usages`. It never purchases credits, creates or rotates
  keys, sends inference requests, or changes quota.
- **Explicit configuration.** The library packages never read the environment,
  the keyring, or a cache implicitly; the caller supplies credentials and
  configuration, so an embedding application controls every source.
- **No secrets at rest unless you ask.** Where a store is used, the OS keyring
  is the default. A fallback token file is created `0600` inside a `0700`
  directory and is refused if its permissions are loosened. Secrets are never
  written to a config file or a cache.
- **No secrets in output.** Only the `Authorization` header carries a
  credential, that header is never logged, error bodies are replaced with their
  typed meaning, and every user-visible error passes through a redaction helper.
  JSON output contains aggregates and the documented fields only.
- **Bounded I/O.** Providers keep their documented request timeouts, response
  caps, bounded retries, and `Retry-After` handling.
- **Explicit offline behaviour.** A cache hit is always labelled with its age; a
  cached runway estimate is never presented as live.

Deleting a credential affects only Burn Blocks-owned local entries; it does not
revoke the credential at the provider, so revoke there if it may have been
exposed.

## Hardening checklist for operators

- Prefer credentials from a secret manager over stored files.
- If you use a fallback file, keep the parent directory at `0700`.
- Treat a leaked management key as an account-level incident and rotate it in
  the provider console.
- Enable two-factor authentication on your provider accounts.

## Supported versions

Security fixes land on the latest released minor version. Older versions may be
asked to upgrade before a fix is backported.
