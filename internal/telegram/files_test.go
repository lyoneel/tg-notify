package telegram_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/lyoneel/cli-tg-notify/internal/telegram"
)

func TestSendFileMultipart(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "report.txt")
	if err := os.WriteFile(path, []byte("file contents"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var gotPath string
	var gotChatID, gotCaption, gotParseMode, gotFile string
	var gotFileField, gotFileName string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		gotChatID = r.FormValue("chat_id")
		gotCaption = r.FormValue("caption")
		gotParseMode = r.FormValue("parse_mode")
		for _, field := range []string{"photo", "document", "audio", "video", "voice"} {
			file, header, err := r.FormFile(field)
			if err != nil {
				continue
			}
			gotFileField = field
			gotFileName = header.Filename
			data, _ := io.ReadAll(file)
			_ = file.Close()
			gotFile = string(data)
			break
		}
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":7}}`)
	})

	id, err := bot.SendFile(context.Background(), "123", telegram.TypeDocument, path, "a caption", "HTML", 0, false)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}
	if id != 7 {
		t.Errorf("message id = %d, want 7", id)
	}
	if gotPath != "/botTOKEN/sendDocument" {
		t.Errorf("request path = %q, want /botTOKEN/sendDocument", gotPath)
	}
	if gotChatID != "123" {
		t.Errorf("chat_id = %q, want 123", gotChatID)
	}
	if gotCaption != "a caption" {
		t.Errorf("caption = %q, want %q", gotCaption, "a caption")
	}
	if gotParseMode != "HTML" {
		t.Errorf("parse_mode = %q, want HTML", gotParseMode)
	}
	if gotFileField != "document" {
		t.Errorf("file field = %q, want document", gotFileField)
	}
	if gotFileName != "report.txt" {
		t.Errorf("file name = %q, want report.txt", gotFileName)
	}
	if gotFile != "file contents" {
		t.Errorf("file contents = %q, want %q", gotFile, "file contents")
	}
}

func TestSendFileOmitsEmptyOptionals(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "photo.png")
	if err := os.WriteFile(path, []byte("png"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var gotCaption, gotParseMode, gotReplyTo string
	var captionPresent, parseModePresent, replyToPresent bool
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		if vals, ok := r.Form["caption"]; ok && len(vals) > 0 {
			captionPresent = true
			gotCaption = vals[0]
		}
		if vals, ok := r.Form["parse_mode"]; ok && len(vals) > 0 {
			parseModePresent = true
			gotParseMode = vals[0]
		}
		if vals, ok := r.Form["reply_to_message_id"]; ok && len(vals) > 0 {
			replyToPresent = true
			gotReplyTo = vals[0]
		}
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":8}}`)
	})

	if _, err := bot.SendFile(context.Background(), "123", telegram.TypePhoto, path, "", "", 0, false); err != nil {
		t.Fatalf("SendFile: %v", err)
	}
	if captionPresent {
		t.Errorf("caption = %q present, want omitted", gotCaption)
	}
	if parseModePresent {
		t.Errorf("parse_mode = %q present, want omitted", gotParseMode)
	}
	if replyToPresent {
		t.Errorf("reply_to_message_id = %q present, want omitted", gotReplyTo)
	}
}

func TestSendFileReplyTo(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "photo.png")
	if err := os.WriteFile(path, []byte("png"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	var gotReplyTo string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		gotReplyTo = r.FormValue("reply_to_message_id")
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":9}}`)
	})

	if _, err := bot.SendFile(context.Background(), "123", telegram.TypePhoto, path, "", "", 42, false); err != nil {
		t.Fatalf("SendFile: %v", err)
	}
	if gotReplyTo != "42" {
		t.Errorf("reply_to_message_id = %q, want 42", gotReplyTo)
	}
}

func TestSendFileDisableNotification(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "photo.png")
	if err := os.WriteFile(path, []byte("png"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	tests := []struct {
		name      string
		silent    bool
		wantValue string
		wantKey   bool
	}{
		{name: "silent true sets disable_notification", silent: true, wantValue: "true", wantKey: true},
		{name: "silent false omits disable_notification", silent: false, wantKey: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			var present bool
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Fatalf("parse multipart form: %v", err)
				}
				if vals, ok := r.Form["disable_notification"]; ok && len(vals) > 0 {
					present = true
					got = vals[0]
				}
				_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":9}}`)
			})

			if _, err := bot.SendFile(context.Background(), "123", telegram.TypePhoto, path, "", "", 0, tt.silent); err != nil {
				t.Fatalf("SendFile: %v", err)
			}
			if present != tt.wantKey {
				t.Fatalf("disable_notification present = %v, want %v", present, tt.wantKey)
			}
			if tt.wantKey && got != tt.wantValue {
				t.Errorf("disable_notification = %q, want %q", got, tt.wantValue)
			}
		})
	}
}

func TestSendFileRef(t *testing.T) {
	tests := []struct {
		name      string
		send      func(bot *telegram.Bot, ref string) (int64, error)
		ref       string
		wantPath  string
		wantField string
	}{
		{
			name: "by url",
			send: func(bot *telegram.Bot, ref string) (int64, error) {
				return bot.SendFileByURL(context.Background(), "123", telegram.TypeDocument, ref, "", "", 0, false)
			},
			ref:       "https://example.com/report.pdf",
			wantPath:  "/botTOKEN/sendDocument",
			wantField: "document",
		},
		{
			name: "by file id",
			send: func(bot *telegram.Bot, ref string) (int64, error) {
				return bot.SendFileByID(context.Background(), "123", telegram.TypePhoto, ref, "cap", "MarkdownV2", 0, false)
			},
			ref:       "AgACAgIAAxk",
			wantPath:  "/botTOKEN/sendPhoto",
			wantField: "photo",
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
				_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":9}}`)
			})

			id, err := tt.send(bot, tt.ref)
			if err != nil {
				t.Fatalf("send: %v", err)
			}
			if id != 9 {
				t.Errorf("message id = %d, want 9", id)
			}
			if gotPath != tt.wantPath {
				t.Errorf("request path = %q, want %q", gotPath, tt.wantPath)
			}
			if gotBody[tt.wantField] != tt.ref {
				t.Errorf("field %s = %q, want %q", tt.wantField, gotBody[tt.wantField], tt.ref)
			}
			if gotBody["chat_id"] != "123" {
				t.Errorf("chat_id = %q, want 123", gotBody["chat_id"])
			}
		})
	}
}

func TestSendFileRefReplyTo(t *testing.T) {
	var gotBody map[string]string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":9}}`)
	})

	if _, err := bot.SendFileByURL(context.Background(), "123", telegram.TypeDocument, "https://example.com/x", "", "", 42, false); err != nil {
		t.Fatalf("SendFileByURL: %v", err)
	}
	if gotBody["reply_to_message_id"] != "42" {
		t.Errorf("reply_to_message_id = %q, want 42", gotBody["reply_to_message_id"])
	}
}

func TestSendFileRefDisableNotification(t *testing.T) {
	tests := []struct {
		name    string
		silent  bool
		wantKey bool
		wantVal string
	}{
		{name: "silent true sets disable_notification", silent: true, wantKey: true, wantVal: "true"},
		{name: "silent false omits disable_notification", silent: false, wantKey: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]string
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				data, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(data, &gotBody)
				_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":9}}`)
			})

			if _, err := bot.SendFileByURL(context.Background(), "123", telegram.TypeDocument, "https://example.com/x", "", "", 0, tt.silent); err != nil {
				t.Fatalf("SendFileByURL: %v", err)
			}
			got, present := gotBody["disable_notification"]
			if present != tt.wantKey {
				t.Fatalf("disable_notification present = %v, want %v", present, tt.wantKey)
			}
			if tt.wantKey && got != tt.wantVal {
				t.Errorf("disable_notification = %q, want %q", got, tt.wantVal)
			}
		})
	}
}

func TestSendFileRefOmitsReplyTo(t *testing.T) {
	var gotBody map[string]string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = fmt.Fprint(w, `{"ok":true,"result":{"message_id":9}}`)
	})

	if _, err := bot.SendFileByID(context.Background(), "123", telegram.TypePhoto, "id", "", "", 0, false); err != nil {
		t.Fatalf("SendFileByID: %v", err)
	}
	if _, present := gotBody["reply_to_message_id"]; present {
		t.Errorf("reply_to_message_id present, want omitted")
	}
}

func TestEndpointMapping(t *testing.T) {
	tests := []struct {
		fileType telegram.FileType
		want     string
	}{
		{telegram.TypePhoto, "sendPhoto"},
		{telegram.TypeDocument, "sendDocument"},
		{telegram.TypeAudio, "sendAudio"},
		{telegram.TypeVideo, "sendVideo"},
		{telegram.TypeVoice, "sendVoice"},
		{telegram.TypeAnimation, "sendAnimation"},
		{telegram.TypeSticker, "sendSticker"},
		{telegram.FileType("foo"), ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.fileType), func(t *testing.T) {
			if got := tt.fileType.Endpoint(); got != tt.want {
				t.Errorf("Endpoint() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSendFileInvalidType(t *testing.T) {
	bot := telegram.New("TOKEN")
	if _, err := bot.SendFile(context.Background(), "123", telegram.FileType("foo"), "/tmp/x", "", "", 0, false); err == nil {
		t.Error("expected error for unknown type, got nil")
	}
	if _, err := bot.SendFileByURL(context.Background(), "123", telegram.FileType("foo"), "https://example.com/x", "", "", 0, false); err == nil {
		t.Error("expected error for unknown type, got nil")
	}
	if _, err := bot.SendFileByID(context.Background(), "123", telegram.FileType("foo"), "id", "", "", 0, false); err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}

func TestSendMediaGroupRemote(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = fmt.Fprint(w, `{"ok":true,"result":[{"message_id":1},{"message_id":2}]}`)
	})

	items := []telegram.MediaItem{
		{Type: telegram.TypePhoto, Ref: "https://example.com/a.jpg"},
		{Type: telegram.TypeVideo, Ref: "https://example.com/b.mp4"},
	}
	ids, err := bot.SendMediaGroup(context.Background(), "123", items, "cap", "HTML", 0, false)
	if err != nil {
		t.Fatalf("SendMediaGroup: %v", err)
	}
	if gotPath != "/botTOKEN/sendMediaGroup" {
		t.Errorf("request path = %q, want /botTOKEN/sendMediaGroup", gotPath)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Errorf("ids = %v, want [1 2]", ids)
	}
	media, ok := gotBody["media"].([]any)
	if !ok || len(media) != 2 {
		t.Fatalf("media = %v, want 2 items", gotBody["media"])
	}
	first := media[0].(map[string]any)
	if first["type"] != "photo" || first["media"] != "https://example.com/a.jpg" {
		t.Errorf("first item = %v", first)
	}
	if first["caption"] != "cap" || first["parse_mode"] != "HTML" {
		t.Errorf("first item caption/parse_mode = %v", first)
	}
	second := media[1].(map[string]any)
	if second["type"] != "video" || second["media"] != "https://example.com/b.mp4" {
		t.Errorf("second item = %v", second)
	}
	if _, present := second["caption"]; present {
		t.Errorf("second item has caption, want only first")
	}
}

func TestSendMediaGroupReplyTo(t *testing.T) {
	var gotBody map[string]any
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = fmt.Fprint(w, `{"ok":true,"result":[{"message_id":1},{"message_id":2}]}`)
	})

	items := []telegram.MediaItem{
		{Type: telegram.TypePhoto, Ref: "https://example.com/a.jpg"},
		{Type: telegram.TypePhoto, Ref: "https://example.com/b.jpg"},
	}
	if _, err := bot.SendMediaGroup(context.Background(), "123", items, "", "", 42, false); err != nil {
		t.Fatalf("SendMediaGroup: %v", err)
	}

	media := gotBody["media"].([]any)
	first := media[0].(map[string]any)
	if first["reply_to_message_id"] != "42" {
		t.Errorf("first item reply_to_message_id = %v, want 42", first["reply_to_message_id"])
	}
	second := media[1].(map[string]any)
	if _, present := second["reply_to_message_id"]; present {
		t.Errorf("second item has reply_to_message_id, want only first")
	}
}

func TestSendMediaGroupOmitsReplyTo(t *testing.T) {
	var gotBody map[string]any
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		_, _ = fmt.Fprint(w, `{"ok":true,"result":[{"message_id":1},{"message_id":2}]}`)
	})

	items := []telegram.MediaItem{
		{Type: telegram.TypePhoto, Ref: "https://example.com/a.jpg"},
		{Type: telegram.TypePhoto, Ref: "https://example.com/b.jpg"},
	}
	if _, err := bot.SendMediaGroup(context.Background(), "123", items, "", "", 0, false); err != nil {
		t.Fatalf("SendMediaGroup: %v", err)
	}

	media := gotBody["media"].([]any)
	first := media[0].(map[string]any)
	if _, present := first["reply_to_message_id"]; present {
		t.Errorf("first item has reply_to_message_id, want omitted")
	}
}

func TestSendMediaGroupDisableNotification(t *testing.T) {
	tests := []struct {
		name    string
		silent  bool
		wantKey bool
		wantVal string
	}{
		{name: "silent true sets disable_notification on first item", silent: true, wantKey: true, wantVal: "true"},
		{name: "silent false omits disable_notification", silent: false, wantKey: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]any
			bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				data, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(data, &gotBody)
				_, _ = fmt.Fprint(w, `{"ok":true,"result":[{"message_id":1},{"message_id":2}]}`)
			})

			items := []telegram.MediaItem{
				{Type: telegram.TypePhoto, Ref: "https://example.com/a.jpg"},
				{Type: telegram.TypePhoto, Ref: "https://example.com/b.jpg"},
			}
			if _, err := bot.SendMediaGroup(context.Background(), "123", items, "", "", 0, tt.silent); err != nil {
				t.Fatalf("SendMediaGroup: %v", err)
			}

			media := gotBody["media"].([]any)
			first := media[0].(map[string]any)
			got, present := first["disable_notification"]
			if present != tt.wantKey {
				t.Fatalf("first item disable_notification present = %v, want %v", present, tt.wantKey)
			}
			if tt.wantKey && got != tt.wantVal {
				t.Errorf("first item disable_notification = %v, want %v", got, tt.wantVal)
			}
			second := media[1].(map[string]any)
			if _, present := second["disable_notification"]; present {
				t.Errorf("second item has disable_notification, want only first")
			}
		})
	}
}

func TestSendMediaGroupLocal(t *testing.T) {
	tempDir := t.TempDir()
	a := filepath.Join(tempDir, "a.jpg")
	b := filepath.Join(tempDir, "b.jpg")
	_ = os.WriteFile(a, []byte("A"), 0o600)
	_ = os.WriteFile(b, []byte("B"), 0o600)

	var gotPath string
	var gotFileNames []string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = r.ParseMultipartForm(1 << 20)
		for i := 0; ; i++ {
			_, header, err := r.FormFile(fmt.Sprintf("file%d", i))
			if err != nil {
				break
			}
			gotFileNames = append(gotFileNames, header.Filename)
		}
		_, _ = fmt.Fprint(w, `{"ok":true,"result":[{"message_id":10},{"message_id":11}]}`)
	})

	items := []telegram.MediaItem{
		{Type: telegram.TypePhoto, Ref: a},
		{Type: telegram.TypePhoto, Ref: b},
	}
	if _, err := bot.SendMediaGroup(context.Background(), "123", items, "", "", 0, false); err != nil {
		t.Fatalf("SendMediaGroup: %v", err)
	}
	if gotPath != "/botTOKEN/sendMediaGroup" {
		t.Errorf("request path = %q, want /botTOKEN/sendMediaGroup", gotPath)
	}
	if len(gotFileNames) != 2 || gotFileNames[0] != "a.jpg" || gotFileNames[1] != "b.jpg" {
		t.Errorf("file names = %v, want [a.jpg b.jpg]", gotFileNames)
	}
}

func TestSendMediaGroupValidation(t *testing.T) {
	bot := telegram.New("TOKEN")

	_, err := bot.SendMediaGroup(context.Background(), "123", []telegram.MediaItem{{Type: telegram.TypePhoto, Ref: "a.jpg"}}, "", "", 0, false)
	if err == nil {
		t.Error("expected error for single item, got nil")
	}

	_, err = bot.SendMediaGroup(context.Background(), "123", []telegram.MediaItem{
		{Type: telegram.TypeDocument, Ref: "a.pdf"},
		{Type: telegram.TypePhoto, Ref: "b.jpg"},
	}, "", "", 0, false)
	if err == nil {
		t.Error("expected error for non photo/video item, got nil")
	}

	_, err = bot.SendMediaGroup(context.Background(), "123", []telegram.MediaItem{
		{Type: telegram.TypePhoto, Ref: "a.jpg"},
		{Type: telegram.TypePhoto, Ref: "https://example.com/b.jpg"},
	}, "", "", 0, false)
	if err == nil {
		t.Error("expected error for mixed local/remote, got nil")
	}
}
