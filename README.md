---
version: 1.1.20260825
last-updated: 2026-08-25
title: Ly's CLI Telegram Notify
description: Go CLI for sending Telegram messages and files via the Bot API
---

# Ly's CLI Telegram Notify

**English** | [Español](README.es.md) | [Italiano](README.it.md) | [Português](README.pt.md)

> **Main repository**: https://gitlab.com/lyoneel/tgnotify.
> If you are reading this on any other host, it is a mirror. Please
> open issues and merge requests on GitLab.

Send Telegram notifications with a single, simple command. Designed to
stay out of your way: no config files, just environment variables or
flags. One command delivers everything:

- text messages
- photos, videos, documents, and files
- files sent by URL
- resending an existing file already on Telegram
- photo/video albums (up to 10 items)
- GIF animations and stickers
- replying to a specific message

Simple to install, simple to use, simple to configure. The binary is
self-contained with built-in retry handling for rate limits and
transient failures.

## Agent skills

This project has two agent skills in the public
[ly-agent-skills](https://gitlab.com/lyoneel/ly-agent-skills) repository
for driving Telegram notifications from an agent:

- [tg-notify](https://gitlab.com/lyoneel/ly-agent-skills/-/tree/master/tg-notify)
  wraps this CLI: messages, files, albums, replies, chat-ID discovery,
  and identity checks.
- [tg-notipy](https://gitlab.com/lyoneel/ly-agent-skills/-/tree/master/tg-notipy)
  is the Python sibling. It talks to the Bot API directly from a
  self-contained stdlib-only script and does not use this CLI. Where
  features overlap the two are interchangeable; only the CLI adds shell
  completion and socks5/socks5h proxy support.

## Examples

Send a message:

```bash
tg-notify "Deploy finished"
```

Send a file (photo, PDF, or video; type is auto-detected):

```bash
tg-notify -f screenshot.png
tg-notify "Monthly report" -f report.pdf
tg-notify -f clip.mp4
```

Send a GIF animation or sticker:

```bash
tg-notify -f logo.gif -t animation
tg-notify -f sticker.webp -t sticker
```

Send a photo/video album:

```bash
tg-notify --album a.jpg b.jpg c.jpg
```

Send a message from stdin (pipelined):

```bash
echo "Build failed" | tg-notify
```

Reply to a specific message:

```bash
tg-notify --reply-to 42 "Acknowledged"
```

Print your bot's identity (validates the token):

```bash
tg-notify --whoami
```

When a file flag (`-f`, `--url`, or `--file-id`) is present, a leading
text argument becomes the file's caption instead of a message.

## Installation

### Download a release

Grab the binary for your platform from the
[GitLab releases page](https://gitlab.com/lyoneel/tgnotify/-/releases)
or the
[GitHub releases page](https://github.com/lyoneel/tgnotify/releases).
Asset names follow the `tg-notify-<os>-<arch>` pattern, e.g.
`tg-notify-windows-amd64.exe`, `tg-notify-linux-amd64`, or
`tg-notify-darwin-arm64`.

**Windows**

Download the `.exe` for your architecture, rename it to
`tg-notify.exe`, and move it into a directory already on `PATH`; or
keep it in its own folder and add that folder to `PATH`. In PowerShell:

```powershell
$bin = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force $bin | Out-Null
Move-Item .\tg-notify-windows-amd64.exe "$bin\tg-notify.exe"
[Environment]::SetEnvironmentVariable("Path",
  [Environment]::GetEnvironmentVariable("Path", "User") + ";$bin",
  "User")
```

Alternatively, add the folder through Settings > System > About >
Advanced system settings > Environment Variables.

**Linux/macOS**

Make the binary executable and either add its directory to `PATH` or
move it into a directory already on `PATH`:

```bash
chmod +x tg-notify-linux-amd64
mkdir -p ~/bin && mv tg-notify-linux-amd64 ~/bin/tg-notify
```

Make sure `~/bin` (or whichever directory you choose) is on `PATH`.

### go install

```bash
go install gitlab.com/lyoneel/tgnotify/cmd/tg-notify@latest
```

The binary lands in `$(go env GOPATH)/bin` (make sure that directory
is on `PATH`).

### Build from source

```bash
git clone https://gitlab.com/lyoneel/tgnotify.git
cd tgnotify
make build          # or: go build -o ./build/tg-notify ./cmd/tg-notify
```

The Makefile provides build, run, test, and quality targets; `make help`
lists them. The version stamps itself: `make build` injects
`git describe --tags --always --dirty`, and plain `go build` or
`go install` binaries report their module version or VCS revision via
`-v`. Override with:

```bash
make build VERSION=v1.1.20260825
```

### Configure

Set these environment variables before running:

| Variable             | Required | Description                 |
| -------------------- | -------- | --------------------------- |
| `TELEGRAM_BOT_TOKEN` | yes      | Telegram bot token          |
| `TELEGRAM_CHAT_ID`   | yes      | Chat ID to send messages to |

Both can be overridden per invocation with the `--token` and
`--chat-id` flags. Chat IDs are passed as strings, so large or negative
group IDs (e.g. `-1001234567890`) work. No config file: set these once
in your shell profile (`~/.bashrc`, `~/.zshrc`) or pass them as flags
per invocation.

## Advanced

### Usage

```bash
tg-notify "message text"                       # positional text sends a message
tg-notify -m "message text" -p HTML            # message with a flag and formatting
echo "text from stdin" | tg-notify             # message read from stdin when no text is given
tg-notify -f path/to/file.png                  # send a local file (auto-detected type)
tg-notify "caption" -f path/to/file.png        # local file with positional caption
tg-notify -u https://example.com/a.pdf -t document
tg-notify -F AgAC... -t photo                  # resend an existing file
tg-notify --album a.jpg b.jpg c.jpg            # send a photo/video album (2-10 items)
tg-notify --reply-to 42 "done"                 # reply to message 42
tg-notify --json "hello"                       # machine-readable JSON output
tg-notify -S -f report.pdf                     # send without a phone notification
tg-notify -D "hello"                           # preview the request without sending
tg-notify -P socks5://127.0.0.1:1080 "x"       # route through a proxy
tg-notify -U http://localhost:8081 -f big.mp4  # self-hosted Bot API
tg-notify -A bash                              # print a bash completion script
tg-notify --whoami                             # print the bot's identity
tg-notify -d                                   # print the latest chat ID
tg-notify -d -o 123                            # print chat ID from updates after ID 123
tg-notify -v                                   # print the version
```

Via make in this repo: `make run ARGS="\"caption\" -f path/to/file.png"`.

### Flags

| Flag | Short | Description | Required |
| ---- | ----- | ----------- | -------- |
| `--message` | `-m` | Message text (1-4096 chars) | No; read from stdin if absent |
| `--file` | `-f` | Local file path to upload | One of `-f`, `--url`, `--file-id` |
| `--url` | `-u` | Remote file URL to send | One of `-f`, `--url`, `--file-id` |
| `--file-id` | `-F` | Telegram `file_id` to resend | One of `-f`, `--url`, `--file-id` |
| `--album` | `-a` | Photo/video files or URLs to send as one album (2-10) | No |
| `--type` | `-t` | Override auto-detection: `photo`, `document`, `audio`, `video`, `voice`, `animation`, `sticker` | Required with `--file-id` |
| `--caption` | `-c` | Caption text for files (0-1024 chars) | No |
| `--parse-mode` | `-p` | `MarkdownV2`, `HTML`, or empty (default: plain text) | No |
| `--reply-to` | `-r` | Message ID to reply to | No |
| `--json` | `-j` | Print machine-readable JSON instead of text | No |
| `--no-trim` | | Preserve whitespace in a piped message | No |
| `--whoami` | `-w` | Print the bot's identity and exit | Mode switch |
| `--token` | `-T` | Bot token (overrides `TELEGRAM_BOT_TOKEN`) | No |
| `--chat-id` | `-C` | Chat ID (overrides `TELEGRAM_CHAT_ID`) | No |
| `--no-retry` | `-n` | Disable auto-retry on 429 rate limits and transient errors | No |
| `--retries` | `-R` | Max retries on transient errors (default 60, 0 disables) | No |
| `--base-wait` | `-B` | First backoff wait on transient errors (default 2s) | No |
| `--discover-chat-id` | `-d` | Print the chat ID from the latest bot update | Mode switch |
| `--offset` | `-o` | Past update ID to skip in `--discover-chat-id` | No |
| `--silent` | `-S` | Deliver without a phone notification | No |
| `--proxy` | `-P` | Proxy URL (`http`, `https`, `socks5`, `socks5h`) overriding env proxies | No |
| `--base-url` | `-U` | Bot API base URL for a self-hosted server | No |
| `--dry-run` | `-D` | Print the resolved request without sending | No |
| `--completion` | `-A` | Print a completion script (`bash`, `zsh`, `fish`) and exit | Mode switch |
| `--version` | `-v` | Print the version and exit | Mode switch |
| `--help` | `-h` | Print usage and exit | Mode switch |

Mode selection: `-f`, `--url`, or `--file-id` present sends a file,
and a positional text argument becomes the file caption; `--album`
sends a photo/video album; without a file or album flag,
`--discover-chat-id` prints the chat ID, `--whoami` prints the bot
identity, and positional text or `--message` is sent as a message. Flags
parse anywhere on the command line, including after the positional text.

File type is auto-detected from the extension and MIME type; unknown
types are sent as `document`. URL sends default to `document` when
`--type` is absent. Local uploads are checked client-side against the
Telegram limits: 10 MB for photos, 50 MB for everything else. When
`--base-url` targets a self-hosted Bot API server, the limit rises to
2000 MB for every type.

### Self-hosted API

Point `--base-url` (or `TELEGRAM_BASE_URL`) at a self-hosted Bot API
server to route requests through your own gateway instead of
`api.telegram.org`. This bypasses the official HTTP gateway's upload
cap, raising the client-side limit to 2000 MB per file. The server is
the `telegram-bot-api` binary from `tdlib/telegram-bot-api`; it still
requires a bot token and an internet route to Telegram.

### Proxy and .env

`--proxy` (or `TELEGRAM_PROXY`) routes every request through an HTTP,
HTTPS, SOCKS5, or SOCKS5h proxy, overriding any `HTTP_PROXY`/
`HTTPS_PROXY`/`NO_PROXY` settings. On startup, an optional `./.env`
file is loaded before the environment is read, so file-supplied values
fill in any unset variable; real environment variables and flags always
win over the file.

Note: this CLI treats `socks5://` and `socks5h://` identically (the
Go SOCKS5 dialer always sends hostnames to the proxy), so the
curl-style local-vs-remote DNS distinction does not apply.

### JSON output

Pass `--json` to print a single machine-readable line instead of the
human summary, for scripting:

```bash
tg-notify --json "hello"
# {"ok":true,"message_id":123}
```

The same flag works with file and album sends (albums emit a
`message_ids` array) and with `--discover-chat-id` (which emits
`{"chat_id": …}`). Errors still print to stderr and exit non-zero.

### Behaviour

- On success the CLI prints `Sent (message_id: <id>)` and exits 0.
- On failure it prints `Failed: <reason>` to stderr and exits 1; bot
  tokens are masked out of any Bot API URL in error output.
- On a 429 rate limit it waits `retry_after` seconds (default 5) and
  retries exactly once, unless `--no-retry` is set.
- On a transient failure (network timeout, connection error, or HTTP
  5xx) it retries up to `--retries` times (default 60) with exponential
  backoff starting at `--base-wait` (default 2s, doubling to a 60s
  per-wait cap, jittered), unless `--no-retry` is set. Missing local
  files are not transient and always fail immediately.
- With no message, no positional text, and stdin attached to a terminal,
  the CLI prints usage and exits 1 instead of waiting for piped input;
  pipe input to send it (`echo "text" | tg-notify`).

#### Retries

The CLI handles two kinds of failure automatically, each with its own
schedule.

**Rate limits (429).** When Telegram replies with HTTP 429, the CLI
sleeps for the `retry_after` interval Telegram specifies (default 5
seconds) and retries exactly once. There is no retry cap to tune here:
the single retry is all it gets.

**Transient errors.** Network timeouts, connection errors, and HTTP
5xx responses are retried up to `--retries` times (default 60) with
exponential backoff. The first retry waits `--base-wait` (default 2s);
each subsequent attempt doubles that wait up to a 60s per-wait cap,
with ±25% jitter so many parallel clients do not retry in lockstep.
For example, with the defaults the waits run roughly 2s, 4s, 8s,
16s, 32s, then 60s from then on.

Both mechanisms are disabled by `--no-retry`, which makes every
failure return immediately. Setting `--retries 0` disables only the
transient-error retries; 429 rate-limit retries still happen once
unless `--no-retry` is also set. `--retries` must be zero or more and
`--base-wait` greater than zero; invalid values are rejected with an
error.

Retrying a timed-out request can rarely duplicate a message, because a
request that already reached Telegram but timed out on the response
may be sent twice. Connection failures that never reached Telegram
cannot cause duplicates.
