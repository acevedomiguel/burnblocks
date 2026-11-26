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

## Status

Both provider engines live here now. `kimi` exposes the usage source, snapshot
and runway state, schema-1 JSON, the caller-owned cache, configuration, and
credential helpers. `openrouter` exposes the read-only API boundary, snapshot
fetch, schema-2 JSON, the aggregate-only cache, configuration, and credential
helpers. Each carries its original synthetic test suite and an external consumer
check. The `burnblocks kimi` and `burnblocks openrouter` subcommands and the
consumer migrations are follow-up work.

## Requirements

- Go 1.26.0 or newer (`go.mod` declares the minimum).
- Node.js as pinned in `.nvmrc`, only for release tooling.

## Development

```bash
make build     # build ./cmd/burnblocks into ./burnblocks
make test      # go test ./...
make check     # fmt + vet + test
make snapshot  # local GoReleaser snapshot
```

## Usage

Provider subcommands are not available yet; `burnblocks --version` reports the
version. The command line grows `burnblocks kimi` and `burnblocks openrouter`
next.

## Security

Credentials are never logged, never written to the config file or a cache, and
never appear in `--json` output. See [SECURITY.md](./SECURITY.md).

## Release

Merging a Conventional Commit into `main` is the only release action;
automation computes the version, tags it, publishes binaries, and updates the
Homebrew tap. See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

MIT. See [LICENSE](./LICENSE).
