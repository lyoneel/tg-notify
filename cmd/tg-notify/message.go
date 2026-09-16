package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"gitlab.com/lyoneel/tgnotify"
)

const maxMessageRunes = 4096

// stdin is the reader used for piped message text; it is a variable so
// tests can substitute a buffer.
var stdin io.Reader = os.Stdin

// stdinIsTTY reports whether the real process stdin is an interactive
// terminal. It is a variable so tests can override it.
var stdinIsTTY = func() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func newBotFor(opts options) (*tgnotify.Bot, string, error) {
	token := resolveToken(opts.token)
	if token == "" {
		return nil, "", errors.New("bot token required: use --token or set TELEGRAM_BOT_TOKEN")
	}
	chatID := resolveChatID(opts.chatID)
	if chatID == "" {
		return nil, "", errors.New("chat ID required: use --chat-id or set TELEGRAM_CHAT_ID")
	}
	bot := tgnotify.New(token)
	if err := configureBot(bot, opts); err != nil {
		return nil, "", err
	}
	return bot, chatID, nil
}

// configureBot applies the base-URL and proxy overrides to a freshly
// constructed bot. Any error (invalid proxy URL, in particular) aborts
// before a network call is made.
func configureBot(bot *tgnotify.Bot, opts options) error {
	if baseURL := resolveBaseURL(opts.baseURL); baseURL != "" {
		bot.SetBaseURL(baseURL)
	}
	if proxy := resolveProxy(opts.proxy); proxy != "" {
		if err := bot.SetProxy(proxy); err != nil {
			return err
		}
	}
	return nil
}

func runMessage(ctx context.Context, opts options, positional []string) error {
	text := opts.message
	if text == "" {
		if len(positional) == 0 {
			// No positional text and no --message: read the message
			// from stdin, so tg-notify works in shell pipelines. When
			// stdin is an interactive terminal there is no piped data
			// to read, so fall back to usage instead of hanging.
			if stdinIsTTY() {
				return errUsage
			}
			var err error
			text, err = readMessageFromStdin(opts.noTrim)
			if err != nil {
				return err
			}
		} else {
			text = positional[0]
			if len(positional) > 1 {
				return fmt.Errorf("quote multi-word messages: tg-notify \"hello world\"")
			}
		}
	} else if len(positional) > 0 {
		return fmt.Errorf("unexpected positional argument %q next to --message", positional[0])
	}

	bot, target, err := newBotFor(opts)
	if err != nil {
		return err
	}
	if n := utf8.RuneCountInString(text); n == 0 || n > maxMessageRunes {
		return fmt.Errorf("message must be 1-%d characters (%d given)", maxMessageRunes, n)
	}

	if opts.dryRun {
		printDryRunMessage(opts, target, text)
		return nil
	}

	id, err := retryWithBackoff(func() (int64, error) {
		return bot.SendMessage(ctx, target, text, opts.parseMode, opts.replyTo, opts.silent)
	}, opts.noRetry, opts.retries, opts.baseWait)
	if err != nil {
		return err
	}
	printResult(opts.jsonOut, id)
	return nil
}

// printDryRunMessage prints the resolved sendMessage request without
// sending it. The bot token is never included.
func printDryRunMessage(opts options, target, text string) {
	if opts.jsonOut {
		fmt.Printf("{\"ok\":true,\"dry_run\":true,\"method\":\"sendMessage\",\"chat_id\":%q,\"text_length\":%d,\"parse_mode\":%q,\"reply_to_message_id\":%d,\"disable_notification\":%t}\n",
			target, utf8.RuneCountInString(text), opts.parseMode, opts.replyTo, opts.silent)
		return
	}
	fmt.Printf("dry-run: sendMessage to %s (%d chars)", target, utf8.RuneCountInString(text))
	if opts.parseMode != "" {
		fmt.Printf(", parse_mode=%s", opts.parseMode)
	}
	if opts.replyTo != 0 {
		fmt.Printf(", reply_to=%d", opts.replyTo)
	}
	if opts.silent {
		fmt.Printf(", silent")
	}
	fmt.Println()
}

// readMessageFromStdin reads the message text from stdin. By default it
// trims surrounding whitespace (including the trailing newline) so piped
// input becomes a clean message; noTrim preserves it exactly.
func readMessageFromStdin(noTrim bool) (string, error) {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read message from stdin: %w", err)
	}
	if noTrim {
		return string(data), nil
	}
	return strings.TrimSpace(string(data)), nil
}
