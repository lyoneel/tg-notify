package telegram_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/lyoneel/tgnotify/internal/telegram"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) (*telegram.Bot, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	bot := telegram.New("TOKEN")
	bot.SetBaseURL(server.URL)
	return bot, server
}

func TestSendMessageSuccess(t *testing.T) {
	tests := []struct {
		name      string
		chatID    string
		text      string
		parseMode string
		wantPath  string
	}{
		{
			name:      "plain text",
			chatID:    "123",
			text:      "hello",
			parseMode: "",
			wantPath:  "/botTOKEN/sendMessage",
		},
		{
			name:      "markdown",
			chatID:    "-1001234567890",
			text:      "*bold*",
			parseMode: "MarkdownV2",
			wantPath:  "/botTOKEN/sendMessage",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			var gotBody map[string]string
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read request body: %v", err)
				}
				if err := json.Unmarshal(data, &gotBody); err != nil {
					t.Fatalf("unmarshal request body: %v", err)
				}
				_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":42}}`)
			})

			id, err := bot.SendMessage(context.Background(), tt.chatID, tt.text, tt.parseMode, 0, false)
			if err != nil {
				t.Fatalf("SendMessage: %v", err)
			}
			if id != 42 {
				t.Errorf("message id = %d, want 42", id)
			}
			if gotPath != tt.wantPath {
				t.Errorf("request path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotBody["chat_id"] != tt.chatID {
				t.Errorf("chat_id = %q, want %q", gotBody["chat_id"], tt.chatID)
			}
			if gotBody["text"] != tt.text {
				t.Errorf("text = %q, want %q", gotBody["text"], tt.text)
			}
			if tt.parseMode == "" {
				if _, present := gotBody["parse_mode"]; present {
					t.Errorf("parse_mode present, want omitted")
				}
			} else if gotBody["parse_mode"] != tt.parseMode {
				t.Errorf("parse_mode = %q, want %q", gotBody["parse_mode"], tt.parseMode)
			}
		})
	}
}

func TestSendMessageAPIError(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		body      string
		wantCode  int
		wantDesc  string
		wantRetry int
	}{
		{
			name:     "bad request",
			status:   http.StatusBadRequest,
			body:     `{"ok":false,"error_code":400,"description":"Bad Request: chat not found"}`,
			wantCode: 400,
			wantDesc: "Bad Request: chat not found",
		},
		{
			name:      "rate limited with retry_after",
			status:    http.StatusTooManyRequests,
			body:      `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":7}}`,
			wantCode:  429,
			wantDesc:  "Too Many Requests",
			wantRetry: 7,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			})

			_, err := bot.SendMessage(context.Background(), "123", "hello", "", 0, false)
			var apiErr *telegram.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v, want *telegram.APIError", err)
			}
			if apiErr.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", apiErr.Code, tt.wantCode)
			}
			if apiErr.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", apiErr.Description, tt.wantDesc)
			}
			if apiErr.RetryAfter != tt.wantRetry {
				t.Errorf("RetryAfter = %d, want %d", apiErr.RetryAfter, tt.wantRetry)
			}
			wantMsg := fmt.Sprintf("HTTP %d: %s", tt.wantCode, tt.wantDesc)
			if apiErr.Error() != wantMsg {
				t.Errorf("Error() = %q, want %q", apiErr.Error(), wantMsg)
			}
		})
	}
}

func TestSendMessageMalformedResponses(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantAPI  bool
		wantCode int
	}{
		{name: "invalid json", status: http.StatusOK, body: `not json`},
		{name: "bad gateway html", status: http.StatusBadGateway, body: `<html>gateway</html>`, wantAPI: true, wantCode: http.StatusBadGateway},
		{name: "internal error invalid json", status: http.StatusInternalServerError, body: `not json`, wantAPI: true, wantCode: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = fmt.Fprint(w, tt.body)
			})

			_, err := bot.SendMessage(context.Background(), "123", "hello", "", 0, false)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var apiErr *telegram.APIError
			gotAPI := errors.As(err, &apiErr)
			if gotAPI != tt.wantAPI {
				t.Fatalf("errors.As(err, *APIError) = %v, want %v (err = %v)", gotAPI, tt.wantAPI, err)
			}
			if tt.wantAPI && apiErr.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", apiErr.Code, tt.wantCode)
			}
		})
	}
}

func TestGetUpdates(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantIDs []int64
	}{
		{
			name:    "messages and edited message",
			body:    `{"ok":true,"result":[{"message":{"chat":{"id":11}}},{"edited_message":{"chat":{"id":22}}},{"message":{"chat":{"id":33}}}]}`,
			wantIDs: []int64{11, 22, 33},
		},
		{
			name:    "empty result",
			body:    `{"ok":true,"result":[]}`,
			wantIDs: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath string
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = fmt.Fprint(w, tt.body)
			})

			updates, err := bot.GetUpdates(context.Background(), 0)
			if err != nil {
				t.Fatalf("GetUpdates: %v", err)
			}
			if gotPath != "/botTOKEN/getUpdates" {
				t.Errorf("request path = %q, want /botTOKEN/getUpdates", gotPath)
			}
			if len(updates) != len(tt.wantIDs) {
				t.Fatalf("got %d updates, want %d", len(updates), len(tt.wantIDs))
			}
			for i, want := range tt.wantIDs {
				id, ok := updates[i].ChatID()
				if !ok {
					t.Fatalf("update %d: ChatID not ok", i)
				}
				if id != want {
					t.Errorf("update %d chat id = %d, want %d", i, id, want)
				}
			}
		})
	}
}

func TestGetUpdatesOffset(t *testing.T) {
	var gotPath, gotQuery string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = fmt.Fprint(w, `{"ok":true,"result":[{"message":{"chat":{"id":9}}}]}`)
	})

	if _, err := bot.GetUpdates(context.Background(), 42); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if gotPath != "/botTOKEN/getUpdates" {
		t.Errorf("request path = %q, want /botTOKEN/getUpdates", gotPath)
	}
	if gotQuery != "offset=42" {
		t.Errorf("query = %q, want offset=42", gotQuery)
	}
}

func TestGetUpdatesZeroOffsetOmitsQuery(t *testing.T) {
	var gotQuery string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = fmt.Fprint(w, `{"ok":true,"result":[]}`)
	})

	if _, err := bot.GetUpdates(context.Background(), 0); err != nil {
		t.Fatalf("GetUpdates: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty", gotQuery)
	}
}

func TestGetMe(t *testing.T) {
	var gotPath string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Notifier","username":"notifier_bot"}}`)
	})

	user, err := bot.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if gotPath != "/botTOKEN/getMe" {
		t.Errorf("request path = %q, want /botTOKEN/getMe", gotPath)
	}
	if user.ID != 123 || !user.IsBot || user.FirstName != "Notifier" || user.Username != "notifier_bot" {
		t.Errorf("user = %+v, want id 123 is_bot true name Notifier @notifier_bot", user)
	}
}

func TestSendMessageReplyTo(t *testing.T) {
	var gotBody map[string]any
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	})

	if _, err := bot.SendMessage(context.Background(), "123", "hi", "", 42, false); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if gotBody["reply_to_message_id"] != float64(42) {
		t.Errorf("reply_to_message_id = %v, want 42", gotBody["reply_to_message_id"])
	}
}

func TestSendMessageDisableNotification(t *testing.T) {
	tests := []struct {
		name     string
		silent   bool
		wantBool bool
		wantKey  bool
	}{
		{name: "silent true sets disable_notification", silent: true, wantBool: true, wantKey: true},
		{name: "silent false omits disable_notification", silent: false, wantKey: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]any
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				data, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(data, &gotBody)
				_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
			})

			if _, err := bot.SendMessage(context.Background(), "123", "hi", "", 0, tt.silent); err != nil {
				t.Fatalf("SendMessage: %v", err)
			}
			got, present := gotBody["disable_notification"]
			if present != tt.wantKey {
				t.Fatalf("disable_notification present = %v, want %v", present, tt.wantKey)
			}
			if tt.wantKey && got != tt.wantBool {
				t.Errorf("disable_notification = %v, want %v", got, tt.wantBool)
			}
		})
	}
}

func TestSendMessageOmitsReplyTo(t *testing.T) {
	var gotBody map[string]any
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":1}}`)
	})

	if _, err := bot.SendMessage(context.Background(), "123", "hi", "", 0, false); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if _, present := gotBody["reply_to_message_id"]; present {
		t.Errorf("reply_to_message_id present, want omitted")
	}
}
