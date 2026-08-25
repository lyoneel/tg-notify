package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"unicode/utf8"

	"gitlab.com/lyoneel/cli-tg-notify/internal/telegram"
)

// runAlbum sends a photo/video album (sendMediaGroup). Album items come
// from --album/--club flags plus any trailing positional arguments, so
// `tg-notify --album a.jpg b.jpg` works; a positional caption is not
// supported (use --caption).
func runAlbum(ctx context.Context, opts options, positional []string) error {
	if opts.message != "" {
		return errors.New("--message cannot be combined with --album")
	}
	if opts.filePath != "" || opts.fileURL != "" || opts.fileID != "" {
		return errors.New("--album cannot be combined with -f, --url, or --file-id")
	}
	if opts.caption != "" {
		if n := utf8.RuneCountInString(opts.caption); n > maxCaptionRunes {
			return fmt.Errorf("caption too long (%d chars, max %d)", n, maxCaptionRunes)
		}
	}

	refs := albumRefs(opts.album, positional)

	bot, target, err := newBotFor(opts)
	if err != nil {
		return err
	}

	items := make([]telegram.MediaItem, 0, len(refs))
	for _, ref := range refs {
		items = append(items, telegram.MediaItem{
			Type: detectAlbumType(ref),
			Ref:  ref,
		})
	}

	// Pre-validate local files so a missing/short path fails fast with a
	// clear error instead of exhausting the transient retry budget.
	if err := validateAlbumFiles(items); err != nil {
		return err
	}

	if opts.dryRun {
		printDryRunAlbum(opts, target, items)
		return nil
	}

	ids, err := retryWithBackoff(func() ([]int64, error) {
		return bot.SendMediaGroup(ctx, target, items, opts.caption, opts.parseMode, opts.replyTo, opts.silent)
	}, opts.noRetry, opts.retries, opts.baseWait)
	if err != nil {
		return err
	}

	if opts.jsonOut {
		fmt.Printf("{\"ok\":true,\"message_ids\":[")
		for i, id := range ids {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Print(id)
		}
		fmt.Print("]}\n")
		return nil
	}
	fmt.Printf("Sent album (%d messages)\n", len(ids))
	return nil
}

// printDryRunAlbum prints the resolved sendMediaGroup request without
// sending it. The bot token is never included.
func printDryRunAlbum(opts options, target string, items []telegram.MediaItem) {
	types := make([]string, len(items))
	for i, it := range items {
		types[i] = string(it.Type)
	}
	if opts.jsonOut {
		fmt.Printf("{\"ok\":true,\"dry_run\":true,\"method\":\"sendMediaGroup\",\"chat_id\":%q,\"item_count\":%d,\"types\":[", target, len(items))
		for i, t := range types {
			if i > 0 {
				fmt.Print(",")
			}
			fmt.Printf("%q", t)
		}
		fmt.Printf("],\"caption_length\":%d,\"parse_mode\":%q,\"reply_to_message_id\":%d,\"disable_notification\":%t}\n",
			utf8.RuneCountInString(opts.caption), opts.parseMode, opts.replyTo, opts.silent)
		return
	}
	fmt.Printf("dry-run: send album (%d items)", len(items))
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

// albumRefs merges --album flag values with trailing positional
// arguments so both `--album a b` and `--album a --album b` (and any
// mix) work.
func albumRefs(flagValues, positional []string) []string {
	refs := make([]string, 0, len(flagValues)+len(positional))
	refs = append(refs, flagValues...)
	refs = append(refs, positional...)
	return refs
}

// validateAlbumFiles stat-checks every local item, rejecting missing
// paths and directories before any network request is made.
func validateAlbumFiles(items []telegram.MediaItem) error {
	for _, it := range items {
		if isRemoteURL(it.Ref) {
			continue
		}
		info, err := os.Stat(it.Ref)
		if err != nil || info.IsDir() {
			return fmt.Errorf("file not found: %s", it.Ref)
		}
	}
	return nil
}

func isRemoteURL(ref string) bool {
	return len(ref) >= 8 && (ref[:7] == "http://" || ref[:8] == "https://")
}

// detectAlbumType guesses the album media type for a local path or
// remote URL. Remote URLs use their path extension when parseable,
// falling back to photo (the most common album item).
func detectAlbumType(ref string) telegram.FileType {
	path := ref
	if u, err := url.Parse(ref); err == nil && u.Path != "" {
		path = u.Path
	}
	ft := telegram.DetectType(path)
	if ft != telegram.TypePhoto && ft != telegram.TypeVideo {
		return telegram.TypePhoto
	}
	return ft
}
