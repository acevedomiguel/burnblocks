# Burn Blocks project instructions

Common engineering, Go, GitHub Actions, and Homebrew conventions are loaded from
the tagged central documents in `opencode.jsonc`. This file contains only
Burn Blocks-specific contracts.

Burn Blocks is the shared, headless Go module behind RouteBurn (OpenRouter),
KimiBurn (Kimi Code), and DashBurn (both), plus the DeepSeek balance provider.
The sibling applications keep their terminal UI and import these packages for
provider access, calculations, and persistence. No Charm, Bubble Tea, Lipgloss,
or other UI dependency belongs in this module.

## Hard rules (MUST)

- **DO NOT EXPOSE PRIVATE INFO.** Real account data, credentials, tokens, API
  keys, authorization-header values, raw provider responses, account emails or
  identifiers, balances, quotas, spend, and usage figures MUST NOT appear in
  code, logs, errors, JSON output, cache files, test output, tests, fixtures,
  screenshots, documentation, commit messages, issue text, or release
  artifacts.
- Only synthetic, hand-authored data MAY be committed to this repository. Real
  data MAY appear only in a user's own terminal, in their own local cache, or in
  their own `--json` output at runtime, and nowhere else.
- Redact by default. Route every user-visible error through the redaction helper
  before it is written, and treat any unredacted provider payload as a defect.
- If a change could surface private information anywhere it is committed or
  transmitted, it MUST NOT be made.

## Product and credential boundaries

- Burn Blocks is read-only. It never sends inference requests, purchases credit
  or quota, or creates, rotates, or revokes a credential.
- OpenRouter calls are limited to `GET /credits`, `GET /key`,
  `GET /analytics/meta`, and the read-only `POST /analytics/query`. Keep
  management and regular API keys separate; `OPENROUTER_API_KEY` is a fallback
  for the regular key only and must never satisfy the management slot.
- Kimi calls are limited to `GET /usages` on the explicitly selected Kimi Code
  host. Never call both hosts automatically, and never fall back to Moonshot
  Open Platform or OpenRouter.
- DeepSeek calls are limited to `GET /user/balance`. DeepSeek has no read-only
  keys: the same key can call models, so the docs must say so, but the engine
  still only ever reads. Never derive a DeepSeek runway from the readiness flag,
  and never total two currencies together or add a DeepSeek amount to another
  provider's dollars.
- The library packages never read the environment, keyring, configuration, or
  cache implicitly and never write to stdout or stderr. Callers supply
  credentials, HTTP clients, clocks, writers, and prior state explicitly.
- Credentials, authorization headers, raw API response bodies, and real account
  data are never exposed in logs, errors, JSON, cache, tests, fixtures, commits,
  or release artifacts. Route user-visible errors through a redaction helper.
- Tests are synthetic-only: hand-authored fixtures, fake keyrings, injected
  clocks, `httptest` servers, and temporary directories. They must not reach the
  real network, keyring, user configuration, cache, or a real credential, and
  must never commit a sanitized real response.

## Architecture boundaries

- Module `github.com/acevedomiguel/burnblocks`, binary built from
  `./cmd/burnblocks`.
- `openrouter`, `kimi`, and `deepseek` are the supported public packages.
  Provider-specific types, quota rules, runway formulas, and balance semantics
  stay inside their own package.
- Share generic utilities only when their semantics are identical across
  providers. Do not model OpenRouter and Kimi behind one generic provider
  abstraction that erases their differences.
- Preserve missing-versus-zero everywhere: optional values stay pointers or
  null, never a fabricated `0`. An unavailable forecast is not infinite.
- The module must never import RouteBurn, KimiBurn, or DashBurn; the dependency
  direction is one-way so no cycle forms.

## Provider contracts to preserve

- OpenRouter: required credits with optional analytics/key degradation to
  warnings, previous-snapshot reuse, TLS/loopback exceptions, 15-second timeout,
  4 MiB response cap, bounded retries, `Retry-After`, nullable limits, negative
  balances, half-open UTC intervals, and aggregate-only versioned cache
  snapshots that never present cached runway as live.
- Kimi: only the selected `GET /usages` host, token resolution limited to
  Burn Blocks-owned locations, missing-versus-zero semantics, conflict warnings,
  monthly membership and monthly Code allocation never summed, and
  process-lifetime runway history with injected clocks.
- DeepSeek: only `GET /user/balance`, credential resolution limited to
  Burn Blocks-owned locations, amounts parsed from strings with missing-versus-
  zero preserved, per-currency balances that are never totalled together, an
  independent schema-1 JSON with a trailing newline, and a runway that is always
  honestly unavailable.
- Preserve each provider's JSON schema version independently, including field
  names, nulls, warning strings, indentation, and trailing newlines.

## Baseline versions

- Go directive `1.26.0` in `go.mod`; CI also builds current stable Go.
- Commit messages are Conventional Commits; releases are derived from them, so
  subjects are lowercase and imperative.

## Git workflow

- ALWAYS work in a dedicated git worktree on a feature branch; never edit or
  commit directly on `main` in the primary checkout. Create worktrees with
  `git worktree add .worktrees/<branch> -b <branch>` and remove them after
  merging.
- Branch names follow Conventional Commit types, e.g. `feat/...`, `fix/...`,
  `docs/...`.
- Land changes through a squash-merged pull request; the PR title is the release
  commit subject, so keep it lowercase and imperative.

## Release automation

- `semantic-release` derives the version from Conventional Commits on `main`
  and drives `.goreleaser.yaml`. Do not create tags, edit the generated release
  sections of `CHANGELOG.md`, or hand-publish normal artefacts.
- Keep the unsuffixed Go module on `v1.x.y`. `scripts/check-module-version.mjs`
  runs in CI and semantic-release `verifyReleaseCmd`; it must reject a release
  major that does not match `go.mod`. Coordinate a `/v2` module-path migration
  before any future v2 tag.
- `.goreleaser.yaml` intentionally publishes a Homebrew **cask** to
  `acevedomiguel/homebrew-burnblocks`, not a formula. Keep generation under
  `homebrew_casks` and do not hand-edit the tap from this repository.
- The tap push is skipped when `HOMEBREW_TAP_TOKEN` is empty, so releases still
  succeed with only the default `GITHUB_TOKEN`.
- Preserve `CGO_ENABLED=0`, the declared target exclusions, version injection,
  SHA-256 checksums, and empty-token snapshot behavior.
- Preserve declarative `postflight_steps`, not deprecated `postflight`. The
  quarantine removal exists because binaries are ad-hoc signed; it is a
  documented Gatekeeper trade-off, with signing/notarization preferred.

## Verification

Run checks relevant to the changed area and report anything unavailable:

- Go changes: `make check`, plus `gofmt -l .` (no output), `go test -race ./...`,
  and `go build ./cmd/burnblocks` before a pull request.
- CLI/build changes: `make build` and `./burnblocks --version` without loading
  real credentials.
- GoReleaser changes: `goreleaser check` and `goreleaser release --snapshot
  --clean` with `HOMEBREW_TAP_TOKEN` defined-but-empty, followed by
  `git diff --exit-code` because its hooks run `go mod tidy`.
- Release tooling: `npm ci && npm run check:release-notes` and
  `npm run check:module-version`.
- Public library changes: exercise the package from a separate temporary module
  against synthetic fixtures, with no external network.
