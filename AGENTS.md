# AGENTS.md — Ly CLI Telegram Notify

## What it is

Go CLI that sends Telegram messages and files via the Bot API using a
self-contained client (one external dependency: golang.org/x/net/proxy
for SOCKS5 proxy support).
Project type: application (CLI).

## Commands

| Action | Command |
|--------|---------|
| Help (all targets) | `make help` |
| Build | `make build` |
| Run | `make run ARGS="your message"` (builds, then executes) |
| Test | `make test` |
| Quality | `make audit` (fmt, vet, lint, mod verify, vuln when online) |

The Makefile (workspace app template) also provides `test-race`, `fmt`,
`vet`, `lint`, `vuln`, `bench`, `deadcode`, `generate`, `release`,
`release-all`, and `clean`; `make help` lists every target with a
description. `release` builds the common platforms (linux amd64,
darwin amd64/arm64, windows amd64, Raspberry Pi arm v6/v7) for local
testing; `release-all` cross-compiles every supported platform (linux,
darwin, windows, freebsd, openbsd, with arm and riscv64 variants). Both
stamp the version via `VERSION ?=`, e.g. `make release VERSION=v1.2.0`.

`make run` is the intended dev workflow for quick sends: it builds the
binary, then executes it. A bare positional argument is treated as
message text; `-f`/`--file`, `--url`, or `--file-id` switches to file
sending, in which case a positional text argument becomes the file
caption; `--discover-chat-id` prints the latest chat ID.

## Configuration

All config is environment variables, optionally seeded from a `./.env`
file loaded at startup; real environment variables and flags override
the file:

| Variable | Flag override | Description |
|----------|---------------|-------------|
| `TELEGRAM_BOT_TOKEN` | `--token` | Telegram bot token |
| `TELEGRAM_CHAT_ID` | `--chat-id` | Chat ID, passed as a string (large/negative group IDs work) |
| `TELEGRAM_BASE_URL` | `--base-url` | Bot API base URL for a self-hosted server |
| `TELEGRAM_PROXY` | `--proxy` | Proxy URL (`http`, `https`, `socks5`, `socks5h`) |

Message flags: `--message`/`-m`, `--parse-mode` (`MarkdownV2`, `HTML`,
or empty for plain text, which is the default), `--no-retry`,
`--retries`/`-R`, `--base-wait`/`-B`, `--reply-to`, `--json`, `--silent`/`-S`,
`--proxy`/`-P`, `--base-url`/`-U`, `--dry-run`/`-D`. File
flags: `--file`/`-f`, `--url`, `--file-id` (exactly one), `--type`
(required with `--file-id`; supports `photo`, `document`, `audio`,
`video`, `voice`, `animation`, `sticker`), `--caption`/`-c`,
`--parse-mode`, `--no-retry`, `--retries`/`-R`, `--base-wait`/`-B`,
`--reply-to`, `--json`, `--silent`/`-S`, `--dry-run`/`-D`. `--album`/`-a` sends a photo/video album
(sendMediaGroup), 2-10 items from local paths, URLs, or trailing
positional arguments; `--reply-to` is honoured on albums. `--offset`/`-o`
filters `--discover-chat-id`. `--whoami`
prints the bot identity via `getMe`. `--completion bash|zsh|fish`
(`-A`) prints a completion script. `--dry-run`/`-D` prints the resolved request
without sending. When neither `--message` nor a
positional argument is present, message text is read from stdin
(trimmed unless `--no-trim`).
`-v`/`--version` prints the version: the Makefile injects
`git describe --tags --always --dirty` via ldflags, and plain
`go build`/`go install` binaries fall back to the module version or
VCS revision read from the embedded build info.

## Structure

```
cmd/tg-notify/main.go     # flat flag parsing, mode dispatch, token/chat-id/proxy/base-url resolution, 429 retry-once, transient backoff
cmd/tg-notify/message.go  # message send (sendMessage) and shared bot/config resolution
cmd/tg-notify/file.go     # file send (sendPhoto/sendDocument/sendAudio/sendVideo/sendVoice/sendAnimation/sendSticker)
cmd/tg-notify/album.go    # album send (sendMediaGroup)
cmd/tg-notify/whoami.go   # whoami mode (getMe)
cmd/tg-notify/discover.go # discover-chat-id mode (getUpdates)
cmd/tg-notify/completion.go # shell completion (bash, zsh, fish)
cmd/tg-notify/dotenv.go   # optional ./.env loading
internal/telegram # Bot API client: client.go, files.go, filetype.go + tests
Makefile          # build/run/quality targets (see make help)
docs/             # Go guidelines references, plans
CHANGELOG.md      # release history (date-build scheme)
.editorconfig     # editor settings (workspace standard)
.gitignore        # Go, build output, OS patterns
.github/workflows/ci.yml  # GitHub Actions CI (build + test)
.github/workflows/release.yml  # GitHub Actions release on tags
.gitlab-ci.yml            # GitLab CI (build/test, security, release)
pkg/              # empty skeleton
config/           # empty skeleton
```

## Gotchas

- **Binary path**: Build output goes to `./build/tg-notify` (not `./`);
  `make run` executes it.
- **Retry policy**: on HTTP 429 the CLI sleeps `retry_after` seconds
  (default 5) and retries exactly once; transient failures (network
  timeouts, connection errors, HTTP 5xx) retry up to 60 times by
  default with exponential backoff (2s base, doubling, 60s per-wait
  cap, ±25% jitter, no total-time cap), overridable via
  `--retries`/`-R` and `--base-wait`/`-B`; `--no-retry` disables all
  retries.
- **Parse mode default**: empty (plain text). Formatting requires
  `--parse-mode MarkdownV2` or `HTML`.
- **Type detection order**: the extension table is consulted before the
  MIME table; unknown types fall back to `document`.
- **Flag parsing**: arguments are reordered before parsing so flags work
  anywhere on the command line, including after the positional text;
  multi-word positional text is rejected with a quoting hint.
- **Positional caption**: with a file flag present, one positional text
  argument is used as the caption; it conflicts with `--caption` and
  `--message`, which are rejected with an error.
- **Stdin message**: with no message/positional text, text is read from
  stdin (trimmed unless `--no-trim`); on a terminal stdin this falls
  back to usage instead of hanging.
- **Mode validation**: flags that do not apply to the selected mode are
  rejected (e.g. a message with `--whoami`, `--offset` without
  `--discover-chat-id`); `--reply-to` and `--offset` must be non-negative.
- **Token scrub**: the `/bot…/` token segment of Bot API URLs is masked
  in error output, so network errors cannot leak the token.
- **Flag misuse**: the `flag` package exits with code 2 on unknown
  flags; application errors exit 1.
- **No request logging**: the client never logs request/response
  bodies, so the bot token cannot leak through logs.
- **Silent delivery**: `--silent`/`-S` sets `disable_notification` on
  message, file, URL, file-id, and album sends.
- **Self-hosted base URL**: `--base-url`/`-U` (or `TELEGRAM_BASE_URL`) targets a
  self-hosted Bot API server; the local upload limit relaxes to 2000 MB
  for all types (official API caps stay 10 MB photo / 50 MB other).
- **Proxy**: `--proxy`/`-P` (or `TELEGRAM_PROXY`) supports `http`, `https`,
  `socks5`, and `socks5h`; a shared transport is set on all three HTTP
  clients.
- **Dry run**: `--dry-run`/`-D` prints the resolved request (method, chat ID,
  type, size, caption) without sending and without the token; it works
  with `--json`.
- **.env loading**: a `./.env` file (KEY=VALUE lines) is read at
  startup; it only fills variables not already in the environment.

## References

- `docs/references/go-common-guidelines.md`: conventions for all Go projects (module basics, naming, documentation, Makefile standard, baseline).
- `docs/references/go-app-guidelines.md`: community conventions for non-library Go projects (applications, CLIs, servers).

## Loading Rules

1. Determine the project type before Go work:
   - Library: a reusable Go module with an exported API intended to be imported by other modules.
   - Application: a `main` package that builds a binary (CLI, server, daemon, tool).
2. Load `docs/references/go-common-guidelines.md` for every Go project.
3. Load `docs/references/go-app-guidelines.md` additionally for applications.
