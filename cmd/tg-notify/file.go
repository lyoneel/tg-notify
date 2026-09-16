package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"

	"gitlab.com/lyoneel/tgnotify"
)

const maxCaptionRunes = 1024

func runFile(ctx context.Context, opts options, positional []string) error {
	if opts.message != "" {
		return errors.New("--message cannot be combined with file sending; use --caption or positional text for the file caption")
	}
	switch len(positional) {
	case 0:
	case 1:
		if opts.caption != "" {
			return errors.New("positional text conflicts with --caption; use only one")
		}
		opts.caption = positional[0]
	default:
		return fmt.Errorf("unexpected positional argument %q; quote the caption as a single argument", positional[1])
	}

	sources := 0
	for _, v := range []string{opts.filePath, opts.fileURL, opts.fileID} {
		if v != "" {
			sources++
		}
	}
	if sources > 1 {
		return errors.New("use a local file path, --url, or --file-id, not multiple")
	}
	if opts.fileID != "" && opts.fileType == "" {
		return errors.New("--file-id requires --type (cannot auto-detect type from file_id)")
	}
	if opts.fileType != "" && !tgnotify.FileType(opts.fileType).Valid() {
		return fmt.Errorf("unknown file type: %s", opts.fileType)
	}

	bot, target, err := newBotFor(opts)
	if err != nil {
		return err
	}
	if opts.caption != "" {
		if n := utf8.RuneCountInString(opts.caption); n > maxCaptionRunes {
			return fmt.Errorf("caption too long (%d chars, max %d)", n, maxCaptionRunes)
		}
	}

	var id int64
	switch {
	case opts.filePath != "":
		if opts.dryRun {
			printDryRunFile(opts, target, "local", opts.filePath, opts.fileType)
			return nil
		}
		id, err = sendLocalFile(ctx, bot, target, opts, opts.filePath, opts.fileType, opts.caption, opts.parseMode, opts.replyTo)
	case opts.fileURL != "":
		ft := tgnotify.TypeDocument
		if opts.fileType != "" {
			ft = tgnotify.FileType(opts.fileType)
		}
		if opts.dryRun {
			printDryRunFile(opts, target, "url", opts.fileURL, string(ft))
			return nil
		}
		fmt.Fprintf(os.Stderr, "Sending %s by URL: %s...\n", ft, opts.fileURL)
		id, err = retryWithBackoff(func() (int64, error) {
			return bot.SendFileByURL(ctx, target, ft, opts.fileURL, opts.caption, opts.parseMode, opts.replyTo, opts.silent)
		}, opts.noRetry, opts.retries, opts.baseWait)
	default:
		ft := tgnotify.FileType(opts.fileType)
		if opts.dryRun {
			printDryRunFile(opts, target, "file_id", opts.fileID, string(ft))
			return nil
		}
		fmt.Fprintf(os.Stderr, "Resending %s by file_id: %s...\n", ft, opts.fileID)
		id, err = retryWithBackoff(func() (int64, error) {
			return bot.SendFileByID(ctx, target, ft, opts.fileID, opts.caption, opts.parseMode, opts.replyTo, opts.silent)
		}, opts.noRetry, opts.retries, opts.baseWait)
	}
	if err != nil {
		return err
	}
	printResult(opts.jsonOut, id)
	return nil
}

// printDryRunFile prints the resolved file-send request without sending
// it. The bot token is never included.
func printDryRunFile(opts options, target, sourceKind, source, fileType string) {
	if opts.jsonOut {
		fmt.Printf("{\"ok\":true,\"dry_run\":true,\"method\":\"sendFile\",\"chat_id\":%q,\"source\":%q,\"type\":%q,\"caption_length\":%d,\"parse_mode\":%q,\"reply_to_message_id\":%d,\"disable_notification\":%t}\n",
			target, sourceKind, fileType, utf8.RuneCountInString(opts.caption), opts.parseMode, opts.replyTo, opts.silent)
		return
	}
	fmt.Printf("dry-run: send %s file (%s=%s) to %s", fileType, sourceKind, source, target)
	if opts.caption != "" {
		fmt.Printf(", caption (%d chars)", utf8.RuneCountInString(opts.caption))
	}
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

func sendLocalFile(ctx context.Context, bot *tgnotify.Bot, chatID string, opts options, path, fileTypeFlag, caption, parseMode string, replyTo int64) (int64, error) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0, fmt.Errorf("file not found: %s", path)
	}

	ft := tgnotify.FileType(fileTypeFlag)
	if fileTypeFlag == "" {
		ft = tgnotify.DetectType(path)
	}
	size := info.Size()
	maxSize := tgnotify.MaxUploadSizeFor(ft, opts.baseURL != "")
	if size > maxSize {
		return 0, fmt.Errorf("file too large (%.1f MB, max %.0f MB for %s)",
			float64(size)/(1024*1024), float64(maxSize)/(1024*1024), ft)
	}
	fmt.Fprintf(os.Stderr, "Sending %s: %s (%.1f MB)...\n", ft, filepath.Base(path), float64(size)/(1024*1024))

	return retryWithBackoff(func() (int64, error) {
		return bot.SendFile(ctx, chatID, ft, path, caption, parseMode, replyTo, opts.silent)
	}, opts.noRetry, opts.retries, opts.baseWait)
}
