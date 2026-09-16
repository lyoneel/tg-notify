package tgnotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FileType names the Telegram media types supported by the file
// commands. The string value doubles as the Bot API form/JSON field
// name.
type FileType string

// Supported Telegram media types.
const (
	TypePhoto     FileType = "photo"
	TypeDocument  FileType = "document"
	TypeAudio     FileType = "audio"
	TypeVideo     FileType = "video"
	TypeVoice     FileType = "voice"
	TypeAnimation FileType = "animation"
	TypeSticker   FileType = "sticker"
)

// Endpoint returns the Bot API method for the file type.
func (t FileType) Endpoint() string {
	switch t {
	case TypePhoto:
		return "sendPhoto"
	case TypeDocument:
		return "sendDocument"
	case TypeAudio:
		return "sendAudio"
	case TypeVideo:
		return "sendVideo"
	case TypeVoice:
		return "sendVoice"
	case TypeAnimation:
		return "sendAnimation"
	case TypeSticker:
		return "sendSticker"
	default:
		return ""
	}
}

// Valid reports whether the file type is one of the supported types.
func (t FileType) Valid() bool {
	return t.Endpoint() != ""
}

// SendFile uploads the local file at path via multipart/form-data and
// returns the new message ID. The file is streamed, never read fully
// into memory. silent, when true, delivers without a phone notification.
// It is a wrapper over SendFileOpts.
func (b *Bot) SendFile(ctx context.Context, chatID string, t FileType, path, caption, parseMode string, replyTo int64, silent bool) (int64, error) {
	return b.SendFileOpts(ctx, chatID, t, path, &SendOptions{
		Caption:   caption,
		ParseMode: parseMode,
		ReplyTo:   replyTo,
		Silent:    silent,
	})
}

// SendFileOpts uploads the local file at path via multipart/form-data
// and returns the new message ID. Nil opts means the Telegram defaults.
func (b *Bot) SendFileOpts(ctx context.Context, chatID string, t FileType, path string, opts *SendOptions) (int64, error) {
	if opts == nil {
		opts = &SendOptions{}
	}
	if !t.Valid() {
		return 0, fmt.Errorf("unknown file type: %s", t)
	}
	if err := validateCaption(opts.Caption); err != nil {
		return 0, err
	}

	return applyRetry(b.retryPolicy, func() (int64, error) {
		f, err := os.Open(path)
		if err != nil {
			return 0, err
		}
		defer func() { _ = f.Close() }()

		pr, pw := io.Pipe()
		mw := multipart.NewWriter(pw)
		go func() {
			err := writeMultipart(mw, string(t), filepath.Base(path), f, chatID, opts)
			pw.CloseWithError(err)
		}()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.url(t.Endpoint()), pr)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Content-Type", mw.FormDataContentType())

		resp, err := b.uploadClient.Do(req)
		if err != nil {
			return 0, scrubTokenErr(err)
		}
		defer func() { _ = resp.Body.Close() }()
		raw, err := decodeEnvelope(resp)
		if err != nil {
			return 0, err
		}
		return messageIDFrom(raw)
	})
}

func writeMultipart(mw *multipart.Writer, fieldName, fileName string, file io.Reader, chatID string, opts *SendOptions) error {
	if err := mw.WriteField("chat_id", chatID); err != nil {
		return err
	}
	if opts.Caption != "" {
		if err := mw.WriteField("caption", opts.Caption); err != nil {
			return err
		}
	}
	if opts.ParseMode != "" {
		if err := mw.WriteField("parse_mode", opts.ParseMode); err != nil {
			return err
		}
	}
	if opts.ReplyTo != 0 {
		if err := mw.WriteField("reply_to_message_id", strconv.FormatInt(opts.ReplyTo, 10)); err != nil {
			return err
		}
	}
	if opts.Silent {
		if err := mw.WriteField("disable_notification", "true"); err != nil {
			return err
		}
	}
	part, err := mw.CreateFormFile(fieldName, fileName)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	return mw.Close()
}

// SendFileByURL sends a file by URL; Telegram downloads it server-side.
// It is a wrapper over SendFileByURLOpts.
func (b *Bot) SendFileByURL(ctx context.Context, chatID string, t FileType, fileURL, caption, parseMode string, replyTo int64, silent bool) (int64, error) {
	return b.SendFileByURLOpts(ctx, chatID, t, fileURL, &SendOptions{
		Caption:   caption,
		ParseMode: parseMode,
		ReplyTo:   replyTo,
		Silent:    silent,
	})
}

// SendFileByURLOpts sends a file by URL; Telegram downloads it
// server-side. Nil opts means the Telegram defaults.
func (b *Bot) SendFileByURLOpts(ctx context.Context, chatID string, t FileType, fileURL string, opts *SendOptions) (int64, error) {
	return b.sendFileRef(ctx, chatID, t, fileURL, opts)
}

// SendFileByID resends a file already stored on Telegram servers.
// It is a wrapper over SendFileByIDOpts.
func (b *Bot) SendFileByID(ctx context.Context, chatID string, t FileType, fileID, caption, parseMode string, replyTo int64, silent bool) (int64, error) {
	return b.SendFileByIDOpts(ctx, chatID, t, fileID, &SendOptions{
		Caption:   caption,
		ParseMode: parseMode,
		ReplyTo:   replyTo,
		Silent:    silent,
	})
}

// SendFileByIDOpts resends a file already stored on Telegram servers.
// Nil opts means the Telegram defaults.
func (b *Bot) SendFileByIDOpts(ctx context.Context, chatID string, t FileType, fileID string, opts *SendOptions) (int64, error) {
	return b.sendFileRef(ctx, chatID, t, fileID, opts)
}

func (b *Bot) sendFileRef(ctx context.Context, chatID string, t FileType, ref string, opts *SendOptions) (int64, error) {
	if opts == nil {
		opts = &SendOptions{}
	}
	if !t.Valid() {
		return 0, fmt.Errorf("unknown file type: %s", t)
	}
	if err := validateCaption(opts.Caption); err != nil {
		return 0, err
	}
	payload := map[string]string{
		"chat_id":    chatID,
		string(t):    ref,
		"caption":    opts.Caption,
		"parse_mode": opts.ParseMode,
	}
	if opts.ReplyTo != 0 {
		payload["reply_to_message_id"] = strconv.FormatInt(opts.ReplyTo, 10)
	}
	if opts.Silent {
		payload["disable_notification"] = "true"
	}
	clean := make(map[string]string, len(payload))
	for k, v := range payload {
		if v != "" {
			clean[k] = v
		}
	}
	return applyRetry(b.retryPolicy, func() (int64, error) {
		raw, err := b.callJSON(ctx, b.fileJSONClient, t.Endpoint(), clean)
		if err != nil {
			return 0, err
		}
		return messageIDFrom(raw)
	})
}

// MediaItem is one item of an album (sendMediaGroup): a photo or
// video referenced by local path or URL. Captions are applied per item
// by the caller.
type MediaItem struct {
	Type FileType // TypePhoto or TypeVideo
	Ref  string   // local file path, or http(s) URL for remote sending
}

// SendMediaGroup sends a photo/video album (sendMediaGroup) and
// returns the per-message IDs in order. All items must share one
// transport: either all local paths (uploaded as multipart attachments)
// or all http(s) URLs (downloaded server-side). A single caption is
// applied to the first item. replyTo, when non-zero, makes the album a
// reply to that message (applied to the first item). silent, when true,
// delivers without a phone notification (applied to the first item).
// It is a wrapper over SendMediaGroupOpts.
func (b *Bot) SendMediaGroup(ctx context.Context, chatID string, items []MediaItem, caption, parseMode string, replyTo int64, silent bool) ([]int64, error) {
	return b.SendMediaGroupOpts(ctx, chatID, items, &SendOptions{
		Caption:   caption,
		ParseMode: parseMode,
		ReplyTo:   replyTo,
		Silent:    silent,
	})
}

// SendMediaGroupOpts sends a photo/video album (sendMediaGroup) and
// returns the per-message IDs in order. Nil opts means the Telegram
// defaults.
func (b *Bot) SendMediaGroupOpts(ctx context.Context, chatID string, items []MediaItem, opts *SendOptions) ([]int64, error) {
	if opts == nil {
		opts = &SendOptions{}
	}
	if len(items) < 2 || len(items) > 10 {
		return nil, fmt.Errorf("album must contain 2-10 items (%d given)", len(items))
	}
	if err := validateCaption(opts.Caption); err != nil {
		return nil, err
	}
	remote := allRemote(items)
	if err := validateMediaItems(items, remote); err != nil {
		return nil, err
	}

	media := make([]map[string]string, 0, len(items))
	for i, it := range items {
		ref := it.Ref
		if !remote {
			ref = fmt.Sprintf("attach://file%d", i)
		}
		entry := map[string]string{"type": string(it.Type), "media": ref}
		if i == 0 {
			if opts.Caption != "" {
				entry["caption"] = opts.Caption
			}
			if opts.ParseMode != "" {
				entry["parse_mode"] = opts.ParseMode
			}
			if opts.ReplyTo != 0 {
				entry["reply_to_message_id"] = strconv.FormatInt(opts.ReplyTo, 10)
			}
			if opts.Silent {
				entry["disable_notification"] = "true"
			}
		}
		media = append(media, entry)
	}
	payload := map[string]any{"chat_id": chatID, "media": media}

	return applyRetry(b.retryPolicy, func() ([]int64, error) {
		return b.sendMediaGroupPayload(ctx, chatID, payload, items, remote)
	})
}

func (b *Bot) sendMediaGroupPayload(ctx context.Context, chatID string, payload map[string]any, items []MediaItem, remote bool) ([]int64, error) {
	if remote {
		raw, err := b.callJSON(ctx, b.fileJSONClient, "sendMediaGroup", payload)
		if err != nil {
			return nil, err
		}
		return mediaGroupIDs(raw)
	}

	// Multipart upload with one attachment part per local file.
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	files := make([]*os.File, 0, len(items))
	var closes []func() error
	for _, it := range items {
		f, err := os.Open(it.Ref)
		if err != nil {
			closeAll(closes)
			return nil, err
		}
		files = append(files, f)
		closes = append(closes, f.Close)
	}
	defer closeAll(closes)

	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var werr error
		if werr = mw.WriteField("chat_id", chatID); werr == nil {
			werr = mw.WriteField("media", string(body))
		}
		for i, f := range files {
			if werr != nil {
				break
			}
			var part io.Writer
			part, werr = mw.CreateFormFile(fmt.Sprintf("file%d", i), filepath.Base(items[i].Ref))
			if werr == nil {
				_, werr = io.Copy(part, f)
			}
		}
		if werr == nil {
			werr = mw.Close()
		}
		pw.CloseWithError(werr)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.url("sendMediaGroup"), pr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := b.uploadClient.Do(req)
	if err != nil {
		return nil, scrubTokenErr(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := decodeEnvelope(resp)
	if err != nil {
		return nil, err
	}
	return mediaGroupIDs(raw)
}

func allRemote(items []MediaItem) bool {
	for _, it := range items {
		if !strings.HasPrefix(it.Ref, "http://") && !strings.HasPrefix(it.Ref, "https://") {
			return false
		}
	}
	return true
}

func validateMediaItems(items []MediaItem, remote bool) error {
	for _, it := range items {
		if it.Type != TypePhoto && it.Type != TypeVideo {
			return fmt.Errorf("album items must be photo or video (got %s)", it.Type)
		}
		if remote {
			continue
		}
		if strings.HasPrefix(it.Ref, "http://") || strings.HasPrefix(it.Ref, "https://") {
			return fmt.Errorf("cannot mix local paths and URLs in one album")
		}
	}
	return nil
}

func closeAll(closers []func() error) {
	for _, c := range closers {
		_ = c()
	}
}

func mediaGroupIDs(raw json.RawMessage) ([]int64, error) {
	var msgs []struct {
		MessageID int64 `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &msgs); err != nil {
		return nil, fmt.Errorf("unexpected sendMediaGroup result: %w", err)
	}
	ids := make([]int64, len(msgs))
	for i, m := range msgs {
		ids[i] = m.MessageID
	}
	return ids, nil
}
