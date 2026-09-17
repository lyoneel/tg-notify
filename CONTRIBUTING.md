# Contributing

Thanks for your interest in tg-notify. This document covers how to
set up a development environment, the conventions the project follows,
and the workflow for getting changes merged.

## Development environment

Requirements:

- Go 1.24 or newer
- `make` (optional; all targets have plain `go` equivalents)

Clone and build:

```bash
git clone https://gitlab.com/lyoneel/tg-notify.git
cd tg-notify
make build
```

The binary is written to `./build/tg-notify`. `make help` lists every
available target.

## Code style

- Standard Go formatting (`gofmt`); the Makefile `fmt` target applies it.
- Follow the workspace Go conventions in `docs/references/`:
  `go-common-guidelines.md` (module basics, naming, documentation) and
  `go-app-guidelines.md` (application layout).
- The project is a zero-dependency application: the Bot API client in
  `internal/telegram` uses only the standard library plus
  `golang.org/x/net` for proxy support. Do not add external
  dependencies without a strong reason.
- Exported symbols need doc comments; keep them concise and factual.

## Testing

```bash
make test        # go test ./...
make test-race   # go test -race ./...
make audit       # fmt + vet + lint + mod verify (+ vuln when online)
```

Tests live next to the code they cover (`*_test.go`). The Bot API
client is tested against a local `httptest` server; no network or real
bot token is required.

Proxy behaviour is covered by hermetic end-to-end tests that run
recording HTTP, TLS, and SOCKS5 proxies (`internal/testproxy`) in
front of a fake Bot API, plus a thin exec-the-binary smoke layer that
covers env resolution, `.env` loading, exit codes, and token
scrubbing. Everything binds loopback on dynamic ports; no external
proxy or real token is needed. Proxy failure-path tests always pass
`--no-retry` because connection errors are transient-retried by
default. `make e2e-docker` runs an optional Docker Compose smoke stack
against real third-party proxies (squid, go-socks5-proxy); it skips
cleanly when docker is not installed and never blocks `make test`.

## Git workflow

- Branch from `master` with a short, descriptive name.
- Commit messages follow Conventional Commits
  (`feat:`, `fix:`, `docs:`, `refactor:`, `ci:`, `chore:`, `build:`).
- Keep commits focused: one logical change per commit.

## Pull request process

1. Open a PR against `master`.
2. Ensure `make audit` passes and all tests are green.
3. Describe what the change does and why, including any user-facing
   behaviour changes.
4. A maintainer will review; address feedback before merge.

## Code review

Reviewers check for:

- Correctness and edge cases (especially around flag parsing, retry
  behaviour, and file type detection).
- No secrets or credentials committed (see the release workflow).
- Documentation updated where flags or behaviour change.

## Onboarding

A good first contribution is improving test coverage or documentation.
Run `make help` to see the available targets, and read `AGENTS.md` for
an overview of the codebase structure and conventions.

## License

All contributions are submitted under the MIT license.
