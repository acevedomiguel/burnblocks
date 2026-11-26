# Burn Blocks

**Shared, read-only Go libraries and a CLI for Kimi Code and OpenRouter usage,
credits, and estimated runway.**

Burn Blocks is the single home for the provider logic behind
[RouteBurn](https://github.com/acevedomiguel/routeburn) (OpenRouter),
[KimiBurn](https://github.com/acevedomiguel/kimiburn) (Kimi Code), and
[DashBurn](https://github.com/acevedomiguel/dashburn) (both). Those applications
keep their terminal UI and import these packages for everything else.

> Burn Blocks only reads data. It sends no inference requests, makes no
> purchases, and never creates, rotates, or revokes credentials.

## Packages

| Package | Purpose |
| --- | --- |
| [`openrouter`](./openrouter) | OpenRouter credits, key limits, spend analytics, and credit runway. |
| [`kimi`](./kimi) | Kimi Code membership quotas and runway. |

Both are libraries only. Callers supply credentials, HTTP clients, clocks, and
any prior state explicitly; the packages never read the environment, keyring, or
a cache implicitly, and never write to stdout or stderr.

## Status

Both provider engines live here, together with the `burnblocks` command that
reports either one from the terminal. `kimi` exposes the usage source, snapshot
and runway state, schema-1 JSON, the caller-owned cache, configuration, and
credential helpers. `openrouter` exposes the read-only API boundary, snapshot
fetch, schema-2 JSON, the aggregate-only cache, configuration, and credential
helpers. Each carries its original synthetic test suite and an external consumer
check, and KimiBurn, RouteBurn, and DashBurn all consume these packages.

## Requirements

The Homebrew cask and the prebuilt archives need no toolchain. Building from
source, or embedding the libraries, needs:

- Go 1.26.0 or newer (`go.mod` declares the minimum).
- Node.js as pinned in `.nvmrc`, only for release tooling.

## Installation

### Homebrew (macOS and Linux)

```bash
brew tap acevedomiguel/burnblocks
brew install --cask burnblocks
burnblocks --version
```

Upgrades arrive through the same tap:

```bash
brew update
brew upgrade --cask burnblocks
```

The cask downloads a prebuilt binary rather than compiling from source. That
binary is not notarized by Apple, so the cask clears the macOS quarantine flag
after staging; without that step Gatekeeper would kill the binary on first run.
The tap is a third-party tap, so Homebrew asks you to trust it once:

```bash
brew trust --tap acevedomiguel/burnblocks
```

### Go

The command:

```bash
go install github.com/acevedomiguel/burnblocks/cmd/burnblocks@latest
```

The libraries, for embedding either engine in your own program:

```bash
go get github.com/acevedomiguel/burnblocks/kimi
go get github.com/acevedomiguel/burnblocks/openrouter
```

### Prebuilt binaries

Every release publishes Linux, macOS, and Windows archives plus a `checksums.txt`
file on the GitHub Releases page. Verify a download with:

```bash
sha256sum -c checksums.txt --ignore-missing
```

### From source

```bash
git clone https://github.com/acevedomiguel/burnblocks.git
cd burnblocks
make build
./burnblocks --version
```

## Development

```bash
make build     # build ./cmd/burnblocks into ./burnblocks
make test      # go test ./...
make check     # fmt + vet + test
make snapshot  # local GoReleaser snapshot
```

## Usage

```console
$ burnblocks kimi            # one Kimi Code status check
$ burnblocks openrouter      # one OpenRouter status check
$ burnblocks kimi --json     # the provider's own JSON document
```

Each subcommand performs one read-only check. By default it prints a concise
text report; `--json` prints the provider's versioned document instead — schema 1
for Kimi, schema 2 for OpenRouter — and neither starts a terminal UI.

| Flag | Effect |
| --- | --- |
| `--json` | Emit one provider JSON snapshot on stdout |
| `--config <path>` | Use an explicit configuration file |
| `--no-cache` | Do not read or write the local cache |

Exit codes are `0` success, `1` runtime or API failure, `2` usage error, and `3`
missing credentials. JSON goes to stdout and diagnostics go to stderr.

The command resolves the same configuration, credentials, and cache the sibling
applications use — `KIMIBURN_TOKEN` and the `kimiburn` keyring entry for Kimi,
`ROUTEBURN_MANAGEMENT_KEY`, `ROUTEBURN_API_KEY`, and the `routeburn` entries for
OpenRouter — so it adds no third credential store. Kimi reports ratios and
OpenRouter reports US dollars; the two are never totalled together.

## Security

Credentials are never logged, never written to the config file or a cache, and
never appear in `--json` output. See [SECURITY.md](./SECURITY.md).

## Release

Merging a Conventional Commit into `main` is the only release action;
automation computes the version, tags it, publishes binaries, and updates the
Homebrew tap. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT. See [LICENSE](./LICENSE).
