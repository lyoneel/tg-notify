---
version: 1.4.20260917
last-updated: 2026-09-17
title: Changelog
description: Release history for tg-notify
---

# Changelog

All notable changes to this project are documented here.

Releases use the date-build scheme:

- `vMAJOR.MINOR.YYYYMMDD-N`: prerelease; N is the build number within
  the calendar day.
- `vMAJOR.MINOR.YYYYMMDD`: stable release; no suffix, supersedes the
  prereleases of that date.

## v1.4.20260917

### Changed

- Repository renamed to `tg-notify` on both GitLab and GitHub; the
  module path is `gitlab.com/lyoneel/tg-notify` and the install line
  stays `go install gitlab.com/lyoneel/tg-notify/cmd/tg-notify@latest`.

### Fixed

- The v1.3.20260916 tag still declared the superseded `tgnotify`
  module path, so `go install ...@latest` failed with a module path
  mismatch; this release publishes the renamed path.

## v1.3.20260916

### Added

- Go library: the Bot API client is now the importable root package
  `tgnotify` (previously `internal/telegram`), with an options-style
  send API (`SendOptions`, `NewSendOptions` with `WithParseMode`,
  `WithReplyTo`, `WithSilent`, `WithCaption`) alongside the positional
  methods, and `Opts` variants for every send method
  (`SendMessageOpts`, `SendFileOpts`, `SendFileByURLOpts`,
  `SendFileByIDOpts`, `SendMediaGroupOpts`).
- Library retry policy (`RetryPolicy`, `SetRetryPolicy`): enabled by
  default with the CLI values (60 transient retries, 2s base wait,
  429 `retry_after` honored), progress routed through an injectable
  `Logger`, silent by default.
- `FromEnv` resolves `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`,
  `TELEGRAM_BASE_URL`, and `TELEGRAM_PROXY` into a ready bot and chat
  ID.
- Client-side payload limits (`MaxMessageRunes` 4096,
  `MaxCaptionRunes` 1024) and token scrubbing on transport errors.
- Godoc `Example` functions for the three send styles.

### Changed

- Repository and module renamed to the then-current `tgnotify` slug;
  the install line is now
  `go install gitlab.com/lyoneel/tg-notify/cmd/tg-notify@latest`.
- The CLI retry handling now runs through the library policy; flags
  `--no-retry`, `--retries`, and `--base-wait` keep their exact
  behavior and output.

## v1.1.20260825

### Added

- Spanish, Italian, and Portuguese README translations, with a language
  bar at the top of each README.
- Release download documentation in every README, including the Windows
  steps, covering the binaries published by the CI release jobs.
- End-to-end proxy tests: message, file, album, `--whoami`, and
  `--discover-chat-id` sends go through recording HTTP, HTTPS, and
  SOCKS5 proxies, and every request and response is asserted; a Docker
  Compose smoke stack (`make e2e-docker`) exercises the same flows
  through Squid and a third-party SOCKS5 proxy, with a manual runbook
  in `docs/manual-proxy-e2e.md`.
- Test coverage reporting on merge requests from the Go test Cobertura
  profile.

## v1.0.20260825

### Added

- `--silent`/`-S` delivers messages, files, URLs, file-ids, and albums
  with `disable_notification`, so notifications arrive without buzzing
  the recipient's phone.
- `--proxy`/`-P` (or `TELEGRAM_PROXY`) routes every request through an HTTP,
  HTTPS, SOCKS5, or SOCKS5h proxy, overriding the standard
  `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` settings.
- `--base-url`/`-U` (or `TELEGRAM_BASE_URL`) targets a self-hosted Bot API
  server, raising the local upload limit to 2000 MB for all types.
- `--completion`/`-A bash|zsh|fish` prints a shell completion script.
- `--dry-run`/`-D` prints the resolved request (method, chat ID, type, size,
  caption) without sending and without leaking the token; works with
  `--json`.
- Optional `./.env` loading at startup; file values only fill variables
  not already present in the environment, and flags still win.
- `--retries`/`-R` and `--base-wait`/`-B` flags override the transient
  retry count and the first backoff wait.
- Message text can now be read from stdin when neither `--message` nor
  a positional argument is given, enabling shell pipelines.
- `--reply-to` sets `reply_to_message_id` on messages and files.
- `--whoami` prints the bot's identity via `getMe`, validating the token.
- `--json` prints machine-readable JSON (`message_id` / `message_ids`).
- `--album`/`-a` sends a photo/video album (sendMediaGroup), 2-10 items
  from local paths or URLs; trailing positional arguments are treated
  as additional album items.
- `--reply-to` is now honoured by album sends.
- `--offset`/`-o` skips updates before the given ID in
  `--discover-chat-id`; `--json` also applies to `--discover-chat-id`.
- `--no-trim` preserves leading/trailing whitespace in a piped message.
- `animation` and `sticker` file types for GIFs and stickers.

### Changed

- Repository moved to GitLab as the primary host: module path is now
  `gitlab.com/lyoneel/tg-notify`, so the tool installs with
  `go install gitlab.com/lyoneel/tg-notify/cmd/tg-notify@latest`.
  `golang.org/x/net/proxy` is the first external dependency, added for
  SOCKS5 proxy support.
- The CLI now retries up to 60 times with exponential backoff and
  jitter on transient failures (network timeouts, connection errors,
  HTTP 5xx) in addition to the existing 429 retry-once; `--no-retry`
  disables both; `--discover-chat-id` shares the retry policy.

### Fixed

- Album files are pre-validated, and filesystem errors no longer count
  as transient, so a missing album file fails fast instead of retrying.
- A bare `tg-notify` on a terminal now prints usage instead of waiting
  forever for piped input; piped/redirected input still works.
- `--reply-to` and `--offset` reject negative values.
- Flags that do not apply to the selected mode (e.g. a message with
  `--whoami`, or `--offset` without `--discover-chat-id`) are rejected
  with an error instead of being silently ignored.

## v1.0.20260820

### Added

- Shorthand flags for every option: `-u` (`--url`), `-t` (`--type`),
  `-F` (`--file-id`), `-p` (`--parse-mode`), `-T` (`--token`),
  `-C` (`--chat-id`), `-n` (`--no-retry`), `-d`
  (`--discover-chat-id`), and `-v` (`--version`).
- Typo aliases accepted but not shown in help: `--fileid`, `--chatid`,
  `--parsemode`, `--noretry`, `--discoverchatid`.

### Changed

- Repository and module renamed from `tg-notify-cli` to `tg-notify`:
  the module path is now `gitlab.com/lyoneel/tg-notify` and the repo is
  `lyoneel/tg-notify`.
- `--help`/`-h` prints usage to stdout and exits 0 instead of failing
  with `flag: help requested`.
- Help output lists each flag on its own line with a one-line
  description.
- `-version` replaced by `-v`/`--version`; the old `-version` spelling
  is rejected.

## v0.2.20260817-6

### Changed

- Positional text with a file flag now becomes the file caption:
  `tg-notify "caption" -f shot.png`. It conflicts with `--caption` and
  `--message`, which are rejected with an error.

## v0.2.20260817-5

### Added

- Automatic version stamping: `make build` injects
  `git describe --tags --always --dirty`, and `-version` falls back to
  the module version or VCS revision from the embedded build info, so
  `go install` and plain `go build` binaries report a version without
  any manual ldflags literal.

## v0.2.20260817-4

### Changed

- Module path renamed from `tg-notify-cli` to
  `gitlab.com/lyoneel/tg-notify-cli` so the tool installs directly with
  `go install gitlab.com/lyoneel/tg-notify-cli/cmd/tg-notify@latest`; no
  clone needed.

## v0.2.20260817-3

### Changed

- Moved the binary package to `cmd/tg-notify/` so the tool installs
  with `go install ./cmd/tg-notify` from a repository clone; the
  Makefile (`MAIN_PKG`) and both CI pipelines follow.
- README: rewritten opening paragraph describing what the tool is, and
  a new Installation section with `go install` first.

## v0.2.20260817-2

### Changed

- Flat command line: the `message`, `file`, and `discover-chat-id`
  subcommands are gone. Mode is now chosen by flags: `-f`/`--file`,
  `--url`, or `--file-id` sends a file; `--discover-chat-id` prints the
  chat ID; otherwise the positional text or `-m`/`--message` is sent as
  a message.
- Added `-f` shorthand for `--file`; `-m` and `-c` shorthands kept.
- Flags parse anywhere on the command line, including after positional
  arguments.
- Multi-word positional text is rejected with a hint to quote it,
  instead of silently sending only the first word.

### Fixed

- Error output scrubs the bot token out of Bot API URLs, so a network
  error can no longer print the token.

## v0.2.20260817-1

### Added

- Subcommands `message`, `file`, and `discover-chat-id`, with a bare
  positional argument still treated as message text for backwards
  compatibility.
- File sending: local uploads (multipart), remote URLs, and resends by
  Telegram `file_id`; auto-detection of photo/document/audio/video/
  voice from extension and MIME type with client-side size limits
  (10 MB photos, 50 MB others).
- Parse modes `MarkdownV2` and `HTML` via `--parse-mode` (default:
  plain text).
- Auto-retry once on 429 rate limits using the API's `retry_after`
  value; disable with `--no-retry`.
- Real error handling: failures print `Failed: <reason>` to stderr and
  exit 1; success prints `Sent (message_id: <id>)`.
- `-version` flag backed by an ldflags-injectable version variable.
- Unit tests for the Telegram client (request encoding, success and
  error parsing, 429 retry metadata, multipart uploads, type
  detection, size limits).

### Changed

- Chat ID is handled as a string, so large and negative group IDs
  (e.g. `-1001234567890`) work.

### Removed

- The `github.com/mymmrac/telego` dependency and all of its transitive
  dependencies; the Telegram Bot API client now lives in
  `internal/telegram` and uses only the Go standard library. The module
  has no `require` block and no `go.sum` anymore.

### Fixed

- Missing message argument no longer panics; it exits 1 with a clear
  error.
- Delivery failures are no longer silent: the API response is checked
  and reported.
- The always-on debug logger that could leak the bot token is gone; no
  request/response logging remains.

## v0.1.20260815-1

### Added

- Makefile from the workspace app template: build, run, test, quality,
  and clean targets.
- docs/ with Go guidelines references and Makefile templates.
- .editorconfig and .gitignore from the workspace template.
- Workspace AGENTS.md references and loading rules.

### Changed

- Renamed repository, module, and binary to tg-notify-cli / tg-notify.
- `make run` builds the binary and executes it with the passed message,
  replacing the shell wrappers.
- `make clean` removes build artifacts while keeping the build/
  directory and its .gitkeep.
- Environment variables renamed to match the tg-notify skill:
  `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID`.

### Removed

- send-msg.sh and send-msg.bat (replaced by the Makefile).
- build/ly-bot-notify.exe stale committed binary.
