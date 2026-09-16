package tgnotify

import (
	"fmt"
	"unicode/utf8"
)

// SendOptions carries the optional parameters shared by the send
// methods. The zero value means the Telegram defaults: plain text, no
// reply, notifications on, no caption. Send methods accept a
// *SendOptions and treat nil as the zero value.
//
// FileType is deliberately not an option: it is a required parameter
// of the file methods, not an optional attribute.
type SendOptions struct {
	// ParseMode is "", "MarkdownV2", or "HTML".
	ParseMode string
	// ReplyTo, when non-zero, makes the message a reply to that
	// message ID.
	ReplyTo int64
	// Silent, when true, delivers without a phone notification.
	Silent bool
	// Caption is the media caption (file and album methods only,
	// 0-MaxCaptionRunes runes).
	Caption string
}

// SendOption mutates a SendOptions value. Collect options with
// NewSendOptions and pass the result to any -Opts method.
type SendOption func(*SendOptions)

// WithParseMode sets the parse mode: "MarkdownV2", "HTML", or "" for
// plain text.
func WithParseMode(mode string) SendOption {
	return func(o *SendOptions) { o.ParseMode = mode }
}

// WithReplyTo makes the message a reply to the given message ID.
func WithReplyTo(messageID int64) SendOption {
	return func(o *SendOptions) { o.ReplyTo = messageID }
}

// WithSilent delivers the message without a phone notification.
func WithSilent() SendOption {
	return func(o *SendOptions) { o.Silent = true }
}

// WithCaption sets the media caption (file and album methods only).
func WithCaption(caption string) SendOption {
	return func(o *SendOptions) { o.Caption = caption }
}

// NewSendOptions builds a *SendOptions from functional options. It is
// the bridge between the two API styles:
//
//	bot.SendMessageOpts(ctx, chatID, "hi",
//	    tgnotify.NewSendOptions(tgnotify.WithParseMode("HTML"), tgnotify.WithSilent()))
//
// Passing the result of NewSendOptions() with no options equals nil.
func NewSendOptions(opts ...SendOption) *SendOptions {
	o := &SendOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}
	return o
}

// Telegram payload limits enforced by the send methods.
const (
	MaxMessageRunes = 4096
	MaxCaptionRunes = 1024
)

// validateText checks a text message length (1-MaxMessageRunes runes).
func validateText(text string) error {
	if n := utf8.RuneCountInString(text); n == 0 {
		return fmt.Errorf("message must be 1-%d characters (0 given)", MaxMessageRunes)
	} else if n > MaxMessageRunes {
		return fmt.Errorf("message must be 1-%d characters (%d given)", MaxMessageRunes, n)
	}
	return nil
}

// validateCaption checks a caption length (0-MaxCaptionRunes runes).
func validateCaption(caption string) error {
	if n := utf8.RuneCountInString(caption); n > MaxCaptionRunes {
		return fmt.Errorf("caption too long (%d chars, max %d)", n, MaxCaptionRunes)
	}
	return nil
}
