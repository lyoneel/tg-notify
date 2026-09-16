// Package telegram implements a minimal Telegram Bot API client using
// only the Go standard library. It covers the endpoints needed by the
// tg-notify CLI: sendMessage, getUpdates, and the file-sending methods
// (sendPhoto, sendDocument, sendAudio, sendVideo, sendVoice).
package tgnotify

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"golang.org/x/net/proxy"
)

// DefaultBaseURL is the production Telegram Bot API base URL.
const DefaultBaseURL = "https://api.telegram.org"

// Request timeouts per call class, matching the tg-notify skill:
// 15s for sendMessage/getUpdates JSON calls, 30s for file-by-URL and
// file-id JSON calls (Telegram downloads the URL server-side), and
// 60s for multipart uploads.
const (
	jsonTimeout     = 15 * time.Second
	fileJSONTimeout = 30 * time.Second
	uploadTimeout   = 60 * time.Second
)

// Bot is a Telegram Bot API client bound to a single bot token.
type Bot struct {
	token   string
	baseURL string

	jsonClient     *http.Client
	fileJSONClient *http.Client
	uploadClient   *http.Client

	proxyRootCAs *x509.CertPool
}

// New creates a Bot for the given token against the production API.
func New(token string) *Bot {
	return &Bot{
		token:          token,
		baseURL:        DefaultBaseURL,
		jsonClient:     &http.Client{Timeout: jsonTimeout},
		fileJSONClient: &http.Client{Timeout: fileJSONTimeout},
		uploadClient:   &http.Client{Timeout: uploadTimeout},
	}
}

// SetBaseURL overrides the API base URL; intended for tests and for
// targeting a self-hosted Bot API server.
func (b *Bot) SetBaseURL(u string) {
	b.baseURL = u
}

// SetProxy routes all three HTTP clients through the given proxy URL.
// Supported schemes are http, https, socks5, and socks5h. Empty string
// disables the explicit proxy, restoring the default transport (which
// honours the standard HTTP_PROXY/HTTPS_PROXY/NO_PROXY environment
// variables).
func (b *Bot) SetProxy(proxyURL string) error {
	if proxyURL == "" {
		b.jsonClient.Transport = nil
		b.fileJSONClient.Transport = nil
		b.uploadClient.Transport = nil
		return nil
	}
	u, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid proxy URL: %q (want scheme://host)", proxyURL)
	}

	transport := &http.Transport{}
	switch u.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(u)
	case "socks5", "socks5h":
		dialer, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return fmt.Errorf("invalid SOCKS proxy: %w", err)
		}
		dialCtx, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return fmt.Errorf("SOCKS proxy does not support context dialing")
		}
		transport.DialContext = dialCtx.DialContext
	default:
		return fmt.Errorf("unsupported proxy scheme: %s", u.Scheme)
	}
	if b.proxyRootCAs != nil {
		transport.TLSClientConfig = &tls.Config{RootCAs: b.proxyRootCAs, MinVersion: tls.VersionTLS12}
	}

	b.jsonClient.Transport = transport
	b.fileJSONClient.Transport = transport
	b.uploadClient.Transport = transport
	return nil
}

// SetProxyTLSRootCAs adds PEM-encoded certificates to the root pool
// used to verify TLS proxies (the https:// scheme), e.g. a
// self-signed test proxy. It is callable before or after SetProxy:
// transports configured later pick the pool up, and transports
// already in place are updated. It returns an error when no
// certificate in pemCerts parses.
func (b *Bot) SetProxyTLSRootCAs(pemCerts []byte) error {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemCerts) {
		return fmt.Errorf("no proxy root certificates parsed from PEM data")
	}
	b.proxyRootCAs = pool
	for _, client := range []*http.Client{b.jsonClient, b.fileJSONClient, b.uploadClient} {
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			continue
		}
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{}
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	return nil
}

// APIError is an error reported by the Telegram Bot API (ok: false).
// Code holds the API error_code, or a bare HTTP status when the server
// returned an undecodable 5xx error body.
type APIError struct {
	Code        int
	Description string
	RetryAfter  int
}

// Error formats the API error as "HTTP <code>: <description>".
func (e *APIError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Description)
}

// Update is a minimal view of a Telegram update: only the message and
// edited_message chats are decoded.
type Update struct {
	Message       *Message `json:"message"`
	EditedMessage *Message `json:"edited_message"`
}

// Message is a minimal view of a Telegram message: only the chat.
type Message struct {
	Chat Chat `json:"chat"`
}

// Chat is a minimal view of a Telegram chat: only the ID.
type Chat struct {
	ID int64 `json:"id"`
}

// User is a minimal view of a Telegram user or bot.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

// ChatID returns the chat ID carried by the update, checking the
// message first and then the edited message. ok is false when the
// update carries neither.
func (u Update) ChatID() (int64, bool) {
	if u.Message != nil {
		return u.Message.Chat.ID, true
	}
	if u.EditedMessage != nil {
		return u.EditedMessage.Chat.ID, true
	}
	return 0, false
}

// envelope is the response wrapper used by every Bot API method.
type envelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

type sendMessageParams struct {
	ChatID              string `json:"chat_id"`
	Text                string `json:"text"`
	ParseMode           string `json:"parse_mode,omitempty"`
	ReplyToMessageID    int64  `json:"reply_to_message_id,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

// SendMessage sends text to chatID and returns the new message ID.
// parseMode may be empty (plain text), "MarkdownV2", or "HTML".
// replyTo, when non-zero, makes the message a reply to that message.
// silent, when true, delivers the message without a phone notification.
// It is a wrapper over SendMessageOpts.
func (b *Bot) SendMessage(ctx context.Context, chatID, text, parseMode string, replyTo int64, silent bool) (int64, error) {
	return b.SendMessageOpts(ctx, chatID, text, &SendOptions{
		ParseMode: parseMode,
		ReplyTo:   replyTo,
		Silent:    silent,
	})
}

// SendMessageOpts sends text to chatID and returns the new message
// ID. Nil opts means the Telegram defaults (plain text, no reply,
// notifications on).
func (b *Bot) SendMessageOpts(ctx context.Context, chatID, text string, opts *SendOptions) (int64, error) {
	if opts == nil {
		opts = &SendOptions{}
	}
	if err := validateText(text); err != nil {
		return 0, err
	}
	raw, err := b.callJSON(ctx, b.jsonClient, "sendMessage", sendMessageParams{
		ChatID:              chatID,
		Text:                text,
		ParseMode:           opts.ParseMode,
		ReplyToMessageID:    opts.ReplyTo,
		DisableNotification: opts.Silent,
	})
	if err != nil {
		return 0, err
	}
	return messageIDFrom(raw)
}

// GetMe returns the bot's own user record, used to validate a token.
func (b *Bot) GetMe(ctx context.Context) (*User, error) {
	raw, err := b.call(ctx, b.jsonClient, http.MethodGet, "getMe", nil)
	if err != nil {
		return nil, err
	}
	var user User
	if err := json.Unmarshal(raw, &user); err != nil {
		return nil, fmt.Errorf("unexpected getMe result: %w", err)
	}
	return &user, nil
}

// GetUpdates returns pending updates for the bot, oldest first.
// offset, when positive, skips updates with an ID less than or equal
// to it, so callers can page past updates they have already seen.
func (b *Bot) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	method := "getUpdates"
	if offset > 0 {
		method = fmt.Sprintf("getUpdates?offset=%d", offset)
	}
	raw, err := b.call(ctx, b.jsonClient, http.MethodGet, method, nil)
	if err != nil {
		return nil, err
	}
	var updates []Update
	if err := json.Unmarshal(raw, &updates); err != nil {
		return nil, fmt.Errorf("unexpected getUpdates result: %w", err)
	}
	return updates, nil
}

// tokenURLPattern matches the /bot<token>/ segment of Bot API URLs so
// errors can never carry a raw token.
var tokenURLPattern = regexp.MustCompile(`/bot[^/\s]+/`)

// scrubTokenErr replaces any bot token embedded in an error's text
// (a request URL, for example) with a <token> placeholder. The
// original error value returns unchanged when the text holds no
// token.
func scrubTokenErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	scrubbed := tokenURLPattern.ReplaceAllString(msg, "/bot<token>/")
	if scrubbed == msg {
		return err
	}
	return errors.New(scrubbed)
}

func (b *Bot) callJSON(ctx context.Context, client *http.Client, method string, payload any) (json.RawMessage, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return b.call(ctx, client, http.MethodPost, method, body)
}

func (b *Bot) url(method string) string {
	return fmt.Sprintf("%s/bot%s/%s", b.baseURL, b.token, method)
}

func (b *Bot) call(ctx context.Context, client *http.Client, httpMethod, apiMethod string, body []byte) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethod, b.url(apiMethod), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, scrubTokenErr(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return decodeEnvelope(resp)
}

// decodeEnvelope reads a Bot API response body and unwraps the
// {ok, result, error_code, description, parameters} envelope.
func decodeEnvelope(resp *http.Response) (json.RawMessage, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		if resp.StatusCode >= 500 {
			return nil, &APIError{
				Code:        resp.StatusCode,
				Description: fmt.Sprintf("invalid JSON response from Telegram (HTTP %d)", resp.StatusCode),
			}
		}
		return nil, fmt.Errorf("invalid JSON response from Telegram (HTTP %d): %w", resp.StatusCode, err)
	}
	if !env.OK {
		code := env.ErrorCode
		if code == 0 {
			code = resp.StatusCode
		}
		return nil, &APIError{
			Code:        code,
			Description: env.Description,
			RetryAfter:  env.Parameters.RetryAfter,
		}
	}
	return env.Result, nil
}

func messageIDFrom(raw json.RawMessage) (int64, error) {
	var res struct {
		MessageID int64 `json:"message_id"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return 0, fmt.Errorf("unexpected result: %w", err)
	}
	return res.MessageID, nil
}
