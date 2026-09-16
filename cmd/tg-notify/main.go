// Command tg-notify sends Telegram messages and files via the Bot API
// using a self-contained standard-library client.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"net/http"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"gitlab.com/lyoneel/tgnotify/internal/telegram"
)

// version can be injected at build time:
// go build -ldflags "-X main.version=v1.2.20260820". When empty, the version
// is derived from the embedded build info (module version for
// go install, VCS revision for local builds).
var version = ""

// versionString resolves the display version: the injected value wins,
// then the module version recorded by go install, then the VCS
// revision, and finally the literal dev.
func versionString() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		var revision, modified string
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revision = setting.Value
			case "vcs.modified":
				modified = setting.Value
			}
		}
		if revision != "" {
			if len(revision) > 12 {
				revision = revision[:12]
			}
			if modified == "true" {
				revision += "-dirty"
			}
			return revision
		}
	}
	return "dev"
}

// errUsage signals that usage help should be printed without the
// Failed prefix.
var errUsage = errors.New("usage requested")

// errHelp signals that the user asked for help via -h/--help; main
// prints usage text and exits successfully.
var errHelp = errors.New("help requested")

const usageText = `usage: tg-notify [message] [flags]

  tg-notify "text"                          send a text message
  tg-notify -m "text" --parse-mode HTML     send a formatted message
  tg-notify -f path/to/file                 send a local file
  tg-notify "caption" -f path/to/file       send a local file with caption
  tg-notify --url URL --type document       send a remote file
  tg-notify --file-id ID --type photo       resend a file by file_id
  tg-notify --album a.jpg b.jpg             send a photo/video album
  tg-notify --reply-to 42 "done"            reply to message 42
  tg-notify --whoami                        print the bot's identity
  tg-notify --discover-chat-id              print the latest chat ID
  tg-notify -v                              print the version

flags:
  -m, --message          message text (1-4096 chars); read from stdin if omitted
  -f, --file             local file path to upload
  -u, --url              remote file URL to send
  -F, --file-id          Telegram file_id to resend
  -a, --album            photo/video files or URLs to send as one album (2-10)
  -t, --type             override type: photo, document, audio, video, voice, animation, sticker
  -c, --caption          caption text (0-1024 chars)
  -p, --parse-mode       MarkdownV2, HTML, or empty for plain text
  -r, --reply-to         message ID to reply to
  -j, --json             print machine-readable JSON instead of text
      --no-trim          do not trim whitespace from the stdin message
  -w, --whoami           print the bot's identity and exit
  -T, --token            bot token (overrides TELEGRAM_BOT_TOKEN)
  -C, --chat-id          chat ID (overrides TELEGRAM_CHAT_ID)
  -n, --no-retry         disable auto-retry on 429 and transient errors
  -R, --retries          max retries on transient errors (default 60)
  -B, --base-wait        first backoff wait on transient errors (default 2s)
  -d, --discover-chat-id print the chat ID from the latest bot update
  -o, --offset           past update ID to skip in --discover-chat-id
  -S, --silent           deliver without a phone notification
  -P, --proxy            proxy URL (http, https, socks5, socks5h)
  -U, --base-url         Bot API base URL for a self-hosted server
  -D, --dry-run          print the resolved request without sending
  -A, --completion       print a completion script (bash, zsh, fish) and exit
  -v, --version          print the version
  -h, --help             show this help`

// options carries every parsed flag; command functions pick the fields
// they need.
type options struct {
	message    string
	filePath   string
	fileURL    string
	fileID     string
	fileType   string
	caption    string
	token      string
	chatID     string
	proxy      string
	baseURL    string
	parseMode  string
	replyTo    int64
	offset     int64
	noRetry    bool
	discover   bool
	whoami     bool
	jsonOut    bool
	noTrim     bool
	silent     bool
	dryRun     bool
	completion string
	album      []string
	retries    int
	baseWait   time.Duration
}

// stringSlice is a flag.Value that accumulates repeated flags into a
// slice; used by --album (and other repeatable flags).
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	// Load an optional ./.env before running so file-supplied values
	// become available to the CLI; the real environment and flags still
	// take precedence.
	if err := loadDotEnv(".env"); err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %s\n", err)
		os.Exit(1)
	}
	err := run(os.Args[1:])
	if errors.Is(err, errHelp) {
		_, _ = fmt.Fprintln(os.Stdout, usageText)
		os.Exit(0)
	}
	if errors.Is(err, errUsage) {
		_, _ = fmt.Fprintln(os.Stderr, usageText)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed: %s\n", scrubSecrets(err.Error()))
		os.Exit(1)
	}
}

// tokenURLPattern matches the /bot<token>/ segment of Bot API URLs so
// errors can never print a token, however it was supplied.
var tokenURLPattern = regexp.MustCompile(`/bot[^/\s]+/`)

// scrubSecrets replaces any bot token embedded in a Bot API URL with
// the literal <token> placeholder.
func scrubSecrets(s string) string {
	return tokenURLPattern.ReplaceAllString(s, "/bot<token>/")
}

func run(args []string) error {
	for _, arg := range args {
		if arg == "-v" || arg == "--version" {
			fmt.Println(versionString())
			return nil
		}
	}

	var opts options
	fs := flag.NewFlagSet("tg-notify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.message, "message", "", "message text (1-4096 chars)")
	fs.StringVar(&opts.message, "m", "", "message text (shorthand)")
	fs.StringVar(&opts.filePath, "file", "", "local file path to upload")
	fs.StringVar(&opts.filePath, "f", "", "local file path (shorthand)")
	fs.StringVar(&opts.fileURL, "url", "", "remote file URL to send")
	fs.StringVar(&opts.fileURL, "u", "", "remote file URL (shorthand)")
	fs.StringVar(&opts.fileID, "file-id", "", "Telegram file_id to resend")
	fs.StringVar(&opts.fileID, "fileid", "", "Telegram file_id (alias)")
	fs.StringVar(&opts.fileID, "F", "", "Telegram file_id (shorthand)")
	fs.StringVar(&opts.fileType, "type", "", "override auto-detection: photo, document, audio, video, voice, animation, sticker")
	fs.StringVar(&opts.fileType, "t", "", "override auto-detection (shorthand)")
	fs.StringVar(&opts.caption, "caption", "", "caption text (0-1024 chars)")
	fs.StringVar(&opts.caption, "c", "", "caption text (shorthand)")
	fs.StringVar(&opts.token, "token", "", "bot token (overrides TELEGRAM_BOT_TOKEN)")
	fs.StringVar(&opts.token, "T", "", "bot token (shorthand)")
	fs.StringVar(&opts.chatID, "chat-id", "", "chat ID (overrides TELEGRAM_CHAT_ID)")
	fs.StringVar(&opts.chatID, "chatid", "", "chat ID (alias)")
	fs.StringVar(&opts.chatID, "C", "", "chat ID (shorthand)")
	fs.StringVar(&opts.parseMode, "parse-mode", "", "MarkdownV2, HTML, or empty for plain text")
	fs.StringVar(&opts.parseMode, "parsemode", "", "parse mode (alias)")
	fs.StringVar(&opts.parseMode, "p", "", "parse mode (shorthand)")
	fs.Int64Var(&opts.replyTo, "reply-to", 0, "message ID to reply to")
	fs.Int64Var(&opts.replyTo, "r", 0, "message ID to reply to (shorthand)")
	fs.BoolVar(&opts.whoami, "whoami", false, "print the bot's identity and exit")
	fs.BoolVar(&opts.whoami, "w", false, "print the bot's identity (shorthand)")
	fs.BoolVar(&opts.jsonOut, "json", false, "print machine-readable JSON instead of text")
	fs.BoolVar(&opts.jsonOut, "j", false, "print machine-readable JSON (shorthand)")
	fs.BoolVar(&opts.noTrim, "no-trim", false, "do not trim whitespace from stdin message")
	fs.Int64Var(&opts.offset, "offset", 0, "past update ID to skip in --discover-chat-id")
	fs.Int64Var(&opts.offset, "o", 0, "past update ID to skip in --discover-chat-id (shorthand)")
	fs.Var((*stringSlice)(&opts.album), "album", "photo/video files or URLs to send as one album (2-10)")
	fs.Var((*stringSlice)(&opts.album), "a", "photo/video files or URLs to send as one album (shorthand)")
	fs.BoolVar(&opts.noRetry, "no-retry", false, "disable auto-retry on 429 and transient errors")
	fs.BoolVar(&opts.noRetry, "noretry", false, "disable auto-retry on 429 and transient errors (alias)")
	fs.BoolVar(&opts.noRetry, "n", false, "disable auto-retry on 429 and transient errors (shorthand)")
	fs.IntVar(&opts.retries, "retries", 60, "max retries on transient errors (default 60, 0 disables)")
	fs.IntVar(&opts.retries, "R", 60, "max retries on transient errors (shorthand)")
	fs.DurationVar(&opts.baseWait, "base-wait", 2*time.Second, "first backoff wait on transient errors (default 2s)")
	fs.DurationVar(&opts.baseWait, "basewait", 2*time.Second, "first backoff wait on transient errors (alias)")
	fs.DurationVar(&opts.baseWait, "B", 2*time.Second, "first backoff wait on transient errors (shorthand)")
	fs.BoolVar(&opts.discover, "discover-chat-id", false, "print the chat ID from the latest bot update")
	fs.BoolVar(&opts.discover, "discoverchatid", false, "print the chat ID (alias)")
	fs.BoolVar(&opts.discover, "d", false, "print the chat ID (shorthand)")
	fs.BoolVar(&opts.silent, "silent", false, "deliver without a phone notification")
	fs.BoolVar(&opts.silent, "S", false, "deliver without a phone notification (shorthand)")
	fs.StringVar(&opts.proxy, "proxy", "", "proxy URL (http, https, socks5, socks5h) overriding env proxies")
	fs.StringVar(&opts.proxy, "P", "", "proxy URL (shorthand)")
	fs.StringVar(&opts.baseURL, "base-url", "", "Bot API base URL for a self-hosted server")
	fs.StringVar(&opts.baseURL, "U", "", "Bot API base URL (shorthand)")
	fs.BoolVar(&opts.dryRun, "dry-run", false, "print the resolved request without sending")
	fs.BoolVar(&opts.dryRun, "D", false, "print the resolved request without sending (shorthand)")
	fs.StringVar(&opts.completion, "completion", "", "print a completion script (bash, zsh, fish) and exit")
	fs.StringVar(&opts.completion, "A", "", "print a completion script (shorthand)")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(reorderArgs(args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errHelp
		}
		return err
	}
	if opts.retries < 0 {
		return errors.New("--retries must be >= 0")
	}
	if opts.baseWait <= 0 {
		return errors.New("--base-wait must be > 0")
	}
	if opts.replyTo < 0 {
		return errors.New("--reply-to must be >= 0")
	}
	if opts.offset < 0 {
		return errors.New("--offset must be >= 0")
	}
	if err := validateModeFlags(opts, fs.Args()); err != nil {
		return err
	}

	ctx := context.Background()
	switch {
	case opts.completion != "":
		return runCompletion(opts.completion)
	case opts.whoami:
		return runWhoami(ctx, opts)
	case opts.discover:
		return runDiscover(ctx, opts)
	case len(opts.album) > 0:
		return runAlbum(ctx, opts, fs.Args())
	case opts.filePath != "" || opts.fileURL != "" || opts.fileID != "":
		return runFile(ctx, opts, fs.Args())
	default:
		return runMessage(ctx, opts, fs.Args())
	}
}

// validateModeFlags rejects flags that carry no meaning in the selected
// mode, so a user gets a clear error instead of silently ignoring them.
func validateModeFlags(opts options, positional []string) error {
	switch {
	case opts.completion != "":
		if opts.completion != "bash" && opts.completion != "zsh" && opts.completion != "fish" {
			return fmt.Errorf("unsupported shell %q: want bash, zsh, or fish", opts.completion)
		}
	case opts.whoami:
		if opts.message != "" || len(positional) > 0 || opts.filePath != "" || opts.fileURL != "" ||
			opts.fileID != "" || len(opts.album) > 0 || opts.caption != "" || opts.replyTo != 0 {
			return errors.New("--whoami cannot be combined with message, file, album, caption, or reply-to flags")
		}
	case opts.discover:
		if opts.message != "" || len(positional) > 0 || opts.filePath != "" || opts.fileURL != "" ||
			opts.fileID != "" || len(opts.album) > 0 || opts.caption != "" || opts.replyTo != 0 {
			return errors.New("--discover-chat-id cannot be combined with message, file, album, caption, or reply-to flags")
		}
	case len(opts.album) > 0:
		// album already rejects message, file, url, file-id, no-trim is
		// irrelevant; nothing further to check here.
	default:
		if opts.offset != 0 {
			return errors.New("--offset is only valid with --discover-chat-id")
		}
		if opts.noTrim && opts.message == "" && len(positional) > 0 {
			return errors.New("--no-trim only applies to a message read from stdin")
		}
	}
	return nil
}

// boolFlags take no value; every other flag consumes the next argument
// when written without "=".
var boolFlags = map[string]bool{
	"h":                true,
	"help":             true,
	"no-retry":         true,
	"noretry":          true,
	"n":                true,
	"discover-chat-id": true,
	"discoverchatid":   true,
	"d":                true,
	"whoami":           true,
	"w":                true,
	"json":             true,
	"j":                true,
	"no-trim":          true,
	"silent":           true,
	"S":                true,
	"dry-run":          true,
	"D":                true,
}

// reorderArgs moves flag tokens ahead of positional tokens so the
// standard flag package can parse flags anywhere on the command line
// (tg-notify stray -f x.png still sees -f).
func reorderArgs(args []string) []string {
	flags := make([]string, 0, len(args))
	positional := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if len(arg) < 2 || arg[0] != '-' {
			positional = append(positional, arg)
			continue
		}
		name := strings.TrimLeft(arg, "-")
		flags = append(flags, arg)
		if !strings.Contains(name, "=") && !boolFlags[name] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func resolveToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("TELEGRAM_BOT_TOKEN")
}

func resolveChatID(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("TELEGRAM_CHAT_ID")
}

func resolveBaseURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("TELEGRAM_BASE_URL")
}

func resolveProxy(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("TELEGRAM_PROXY")
}

// resolveTokenOnly resolves the bot token for modes that do not need a
// chat ID (discover, whoami).
func resolveTokenOnly(flagValue string) (string, error) {
	token := resolveToken(flagValue)
	if token == "" {
		return "", errors.New("bot token required: use --token or set TELEGRAM_BOT_TOKEN")
	}
	return token, nil
}

// printResult emits the success line. With --json it prints a
// machine-readable object on one line; otherwise the human-readable
// "Sent (message_id: ...)" summary.
func printResult(jsonOut bool, id int64) {
	if jsonOut {
		fmt.Printf("{\"ok\":true,\"message_id\":%d}\n", id)
		return
	}
	fmt.Printf("Sent (message_id: %d)\n", id)
}

// retryWithBackoff runs send and applies the retry policy. A 429
// rate-limit API error waits retry_after seconds (default 5) and
// retries exactly once. Transient errors (5xx APIError, net.Error)
// retry up to maxRetries times with exponential backoff (baseWait
// doubled per attempt, capped at maxTransientWait, ±25% jitter).
// Every other error returns immediately. noRetry disables all of it;
// maxRetries == 0 disables transient retries only.
func retryWithBackoff[T any](send func() (T, error), noRetry bool, maxRetries int, baseWait time.Duration) (T, error) {
	val, err := send()
	if err == nil {
		return val, nil
	}
	if noRetry {
		return val, err
	}

	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusTooManyRequests {
		wait := apiErr.RetryAfter
		if wait <= 0 {
			wait = 5
		}
		fmt.Fprintf(os.Stderr, "Rate limited. Retrying after %ds...\n", wait)
		sleep(time.Duration(wait) * time.Second)
		return send()
	}

	if !transient(err) {
		return val, err
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		wait := backoffWait(attempt, baseWait)
		fmt.Fprintf(os.Stderr, "Transient error (%v). Retry %d/%d in %s...\n", err, attempt, maxRetries, wait)
		sleep(wait)
		val, err = send()
		if err == nil {
			return val, nil
		}
		var apiErr429 *telegram.APIError
		if errors.As(err, &apiErr429) && apiErr429.Code == http.StatusTooManyRequests {
			wait := apiErr429.RetryAfter
			if wait <= 0 {
				wait = 5
			}
			fmt.Fprintf(os.Stderr, "Rate limited. Retrying after %ds...\n", wait)
			sleep(time.Duration(wait) * time.Second)
			val, err = send()
			if err == nil {
				return val, nil
			}
		}
		if !transient(err) {
			return val, err
		}
	}
	return val, err
}

// transient reports whether err is a retryable transient failure: an
// APIError with a 5xx code or a network-level error. Filesystem errors
// (missing files, permissions) are deliberately excluded so a bad local
// path fails fast instead of exhausting the retry budget.
func transient(err error) bool {
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code >= 500 && apiErr.Code <= 599
	}
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr)
}

// schedule constants for the transient backoff: factor, per-wait cap,
// and jitter fraction are fixed; count and base wait via flags.
const (
	waitFactor       = 2
	maxTransientWait = 60 * time.Second
	jitterFraction   = 0.25
)

// backoffWait computes the jittered wait for a 1-based attempt number:
// the uncapped backoff is jittered by ±jitterFraction.
func backoffWait(attempt int, baseWait time.Duration) time.Duration {
	wait := uncappedWait(attempt, baseWait)
	jitter := float64(wait) * jitterFraction
	offset := rand.Float64()*2*jitter - jitter
	return time.Duration(float64(wait) + offset)
}

// uncappedWait computes the pre-jitter backoff for a 1-based attempt:
// baseWait * waitFactor^(attempt-1), capped at maxTransientWait.
func uncappedWait(attempt int, baseWait time.Duration) time.Duration {
	wait := baseWait
	for i := 1; i < attempt; i++ {
		wait *= waitFactor
		if wait >= maxTransientWait {
			wait = maxTransientWait
			break
		}
	}
	return wait
}

// sleep is indirection for time.Sleep so tests can replace it.
var sleep = time.Sleep
