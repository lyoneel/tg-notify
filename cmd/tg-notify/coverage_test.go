package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/lyoneel/tg-notify"
)

// captureOutput swaps os.Stdout for a pipe (the e2e files already
// define captureStdout), runs fn, and returns what fn printed.
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return string(out)
}

// fakeAPIOptions returns flags that point the CLI at a fake Bot API
// server with retries disabled. env is neutralized so machine
// environment variables cannot leak in.
func fakeAPIOptions(t *testing.T, handler http.HandlerFunc) (server *httptest.Server) {
	t.Helper()
	for _, key := range []string{"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID", "TELEGRAM_BASE_URL", "TELEGRAM_PROXY"} {
		t.Setenv(key, "")
	}
	server = httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func okMessage(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":11}}`))
}

func TestPrintDryRunMessage(t *testing.T) {
	out := captureOutput(t, func() { printDryRunMessage(options{}, "123", "hi") })
	if !strings.HasPrefix(out, "dry-run: sendMessage to 123 (2 chars)") {
		t.Fatalf("text output = %q", out)
	}

	opts := options{jsonOut: true, parseMode: "HTML", replyTo: 4, silent: true}
	out = captureOutput(t, func() { printDryRunMessage(opts, "123", "hi") })
	if out != "{\"ok\":true,\"dry_run\":true,\"method\":\"sendMessage\",\"chat_id\":\"123\",\"text_length\":2,\"parse_mode\":\"HTML\",\"reply_to_message_id\":4,\"disable_notification\":true}\n" {
		t.Fatalf("json output = %q", out)
	}
}

func TestPrintDryRunFile(t *testing.T) {
	opts := options{}
	out := captureOutput(t, func() { printDryRunFile(opts, "123", "local", "x.png", "photo") })
	if !strings.HasPrefix(out, "dry-run: send photo file (local=x.png) to 123") {
		t.Fatalf("text output = %q", out)
	}

	opts = options{jsonOut: true, caption: "cap", parseMode: "HTML", replyTo: 9, silent: true}
	out = captureOutput(t, func() { printDryRunFile(opts, "123", "url", "https://x/y.png", "photo") })
	if !strings.Contains(out, "\"source\":\"url\"") || !strings.Contains(out, "\"caption_length\":3") {
		t.Fatalf("json output = %q", out)
	}
}

func TestPrintDryRunAlbum(t *testing.T) {
	opts := options{}
	items := []tgnotify.MediaItem{{Type: tgnotify.TypePhoto, Ref: "a.jpg"}, {Type: tgnotify.TypeVideo, Ref: "b.mp4"}}
	out := captureOutput(t, func() { printDryRunAlbum(opts, "123", items) })
	if !strings.HasPrefix(out, "dry-run: send album (2 items)") {
		t.Fatalf("text output = %q", out)
	}

	opts = options{jsonOut: true, caption: "c", silent: true}
	out = captureOutput(t, func() { printDryRunAlbum(opts, "123", items) })
	if !strings.Contains(out, "\"item_count\":2") || !strings.Contains(out, "\"types\":[\"photo\",\"video\"]") {
		t.Fatalf("json output = %q", out)
	}
}

func TestRunMessageSend(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)

	out := captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "hello"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "Sent (message_id: 11)\n" {
		t.Fatalf("text output = %q", out)
	}

	out = captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-j", "-m", "hello", "-p", "HTML", "-r", "5", "-S"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "{\"ok\":true,\"message_id\":11}\n" {
		t.Fatalf("json output = %q", out)
	}
}

func TestRunMessageStdin(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)

	old := stdin
	oldTTY := stdinIsTTY
	stdin = strings.NewReader("  piped text \n")
	stdinIsTTY = func() bool { return false }
	defer func() { stdin = old; stdinIsTTY = oldTTY }()

	out := captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-j"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "{\"ok\":true,\"message_id\":11}\n" {
		t.Fatalf("output = %q", out)
	}
}

func TestRunMessageErrors(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "message and positional conflict",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-m", "a", "b"},
			wantErr: "unexpected positional argument \"b\" next to --message",
		},
		{
			name:    "two positionals",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "a", "b"},
			wantErr: "quote multi-word messages: tg-notify \"hello world\"",
		},
		{
			name:    "oversized message",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-m", strings.Repeat("x", 4097)},
			wantErr: "message must be 1-4096 characters (4097 given)",
		},
		{
			name:    "missing token",
			args:    []string{"-C", "123", "-U", server.URL, "-n", "hello"},
			wantErr: "bot token required: use --token or set TELEGRAM_BOT_TOKEN",
		},
		{
			name:    "missing chat id",
			args:    []string{"-T", "tok", "-U", server.URL, "-n", "hello"},
			wantErr: "chat ID required: use --chat-id or set TELEGRAM_CHAT_ID",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestRunFileURLAndID(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)

	out := captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-j", "-u", "https://example.com/doc.pdf", "-t", "document"}); err != nil {
			t.Errorf("run url: %v", err)
		}
	})
	if out != "{\"ok\":true,\"message_id\":11}\n" {
		t.Fatalf("url output = %q", out)
	}

	out = captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-j", "-F", "AgAC-file", "-t", "photo"}); err != nil {
			t.Errorf("run file-id: %v", err)
		}
	})
	if out != "{\"ok\":true,\"message_id\":11}\n" {
		t.Fatalf("file-id output = %q", out)
	}
}

func TestRunFileLocal(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	out := captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-j", "-f", path, "caption text"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "{\"ok\":true,\"message_id\":11}\n" {
		t.Fatalf("output = %q", out)
	}
}

func TestRunFileTooLarge(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)
	dir := t.TempDir()
	path := filepath.Join(dir, "big.png")
	if err := os.WriteFile(path, make([]byte, 10*1024*1024+1), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// The -U flag marks the server self-hosted, which raises the photo
	// limit to 2000 MB, so this path goes through sendLocalFile with a
	// plain (non-self-hosted) options value.
	bot := tgnotify.New("tok")
	bot.SetBaseURL(server.URL)
	bot.SetRetryPolicy(tgnotify.RetryPolicy{Disabled: true})
	opts := options{}
	_, err := sendLocalFile(context.Background(), bot, "123", opts, path, "", "", "", 0)
	if err == nil || !strings.Contains(err.Error(), "file too large") {
		t.Fatalf("err = %v, want file too large", err)
	}
}

func TestRunFileErrors(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.png")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing local file",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-f", missing},
			wantErr: "file not found: " + missing,
		},
		{
			name:    "multiple sources",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-f", "a.png", "-u", "https://x/y.png"},
			wantErr: "use a local file path, --url, or --file-id, not multiple",
		},
		{
			name:    "file id without type",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-F", "AgAC"},
			wantErr: "--file-id requires --type (cannot auto-detect type from file_id)",
		},
		{
			name:    "unknown type",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-f", "a.png", "-t", "wav"},
			wantErr: "unknown file type: wav",
		},
		{
			name:    "message with file",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-f", "a.png", "-m", "hi"},
			wantErr: "--message cannot be combined with file sending; use --caption or positional text for the file caption",
		},
		{
			name:    "caption and positional conflict",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-f", "a.png", "-c", "cap", "text"},
			wantErr: "positional text conflicts with --caption; use only one",
		},
		{
			name:    "two positionals",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-f", "a.png", "one", "two"},
			wantErr: "unexpected positional argument \"two\"; quote the caption as a single argument",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestRunAlbumSend(t *testing.T) {
	server := fakeAPIOptions(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"message_id":1},{"message_id":2}]}`))
	})
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jpg")
	b := filepath.Join(dir, "b.jpg")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte("img"), 0o600); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}

	out := captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-j", "-a", a, b}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "{\"ok\":true,\"message_ids\":[1,2]}\n" {
		t.Fatalf("json output = %q", out)
	}

	out = captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-a", a, "-a", b}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "Sent album (2 messages)\n" {
		t.Fatalf("text output = %q", out)
	}
}

func TestRunAlbumErrors(t *testing.T) {
	server := fakeAPIOptions(t, okMessage)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.jpg")
	if err := os.WriteFile(a, []byte("img"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "album with message",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-a", a, "-m", "hi"},
			wantErr: "--message cannot be combined with --album",
		},
		{
			name:    "album with file",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-a", a, "-f", "b.jpg"},
			wantErr: "--album cannot be combined with -f, --url, or --file-id",
		},
		{
			name:    "album item missing",
			args:    []string{"-T", "tok", "-C", "123", "-U", server.URL, "-n", "-a", a, filepath.Join(dir, "gone.jpg")},
			wantErr: "file not found: " + filepath.Join(dir, "gone.jpg"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestRunWhoami(t *testing.T) {
	server := fakeAPIOptions(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":{"id":7,"is_bot":true,"first_name":"Test","username":"testbot"}}`))
	})

	out := captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-U", server.URL, "-n", "-w"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "@testbot (id: 7)\n" {
		t.Fatalf("text output = %q", out)
	}

	out = captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-U", server.URL, "-n", "-w", "-j"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "{\"ok\":true,\"id\":7,\"is_bot\":true,\"first_name\":\"Test\",\"username\":\"testbot\"}\n" {
		t.Fatalf("json output = %q", out)
	}
}

func TestRunWhoamiWithoutToken(t *testing.T) {
	fakeAPIOptions(t, okMessage)
	err := run([]string{"-w"})
	if err == nil || err.Error() != "bot token required: use --token or set TELEGRAM_BOT_TOKEN" {
		t.Fatalf("err = %v, want missing token error", err)
	}
}

func TestRunDiscover(t *testing.T) {
	server := fakeAPIOptions(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "offset=") {
			_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":[{"message":{"chat":{"id":55}}}]}`))
	})

	out := captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-U", server.URL, "-n", "-d"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "55\n" {
		t.Fatalf("text output = %q", out)
	}

	out = captureOutput(t, func() {
		if err := run([]string{"-T", "tok", "-U", server.URL, "-n", "-d", "-j"}); err != nil {
			t.Errorf("run: %v", err)
		}
	})
	if out != "{\"ok\":true,\"chat_id\":55}\n" {
		t.Fatalf("json output = %q", out)
	}
}

func TestRunDiscoverEmptyUpdates(t *testing.T) {
	server := fakeAPIOptions(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":[]}`))
	})
	err := run([]string{"-T", "tok", "-U", server.URL, "-n", "-d"})
	if err == nil || !strings.Contains(err.Error(), "no updates found") {
		t.Fatalf("err = %v, want no updates error", err)
	}
}

func TestVersionStringInjected(t *testing.T) {
	old := version
	version = "vTest-1"
	defer func() { version = old }()
	if got := versionString(); got != "vTest-1" {
		t.Fatalf("versionString = %q, want vTest-1", got)
	}

	version = ""
	if got := versionString(); got == "" {
		t.Fatal("versionString fallback is empty")
	}
}

func TestMainVersionFlag(t *testing.T) {
	oldArgs := os.Args
	os.Args = []string{"tg-notify", "-v"}
	defer func() { os.Args = oldArgs }()

	out := captureOutput(t, func() { main() })
	if strings.TrimSpace(out) == "" {
		t.Fatal("main -v printed nothing")
	}
}

func TestGetMeInvalidResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":"not-a-user"}`))
	}))
	defer server.Close()
	bot := tgnotify.New("t")
	bot.SetBaseURL(server.URL)
	bot.SetRetryPolicy(tgnotify.RetryPolicy{Disabled: true})
	if _, err := bot.GetMe(context.Background()); err == nil || !strings.Contains(err.Error(), "unexpected getMe result") {
		t.Fatalf("err = %v, want unexpected getMe result", err)
	}
}

func TestGetUpdatesInvalidResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":42}`))
	}))
	defer server.Close()
	bot := tgnotify.New("t")
	bot.SetBaseURL(server.URL)
	bot.SetRetryPolicy(tgnotify.RetryPolicy{Disabled: true})
	if _, err := bot.GetUpdates(context.Background(), 0); err == nil || !strings.Contains(err.Error(), "unexpected getUpdates result") {
		t.Fatalf("err = %v, want unexpected getUpdates result", err)
	}
}

func TestWriteMultipartAllFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	var gotPath string
	var gotForm map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart form: %v", err)
		}
		gotForm = map[string]string{
			"chat_id":              r.FormValue("chat_id"),
			"caption":              r.FormValue("caption"),
			"parse_mode":           r.FormValue("parse_mode"),
			"reply_to_message_id":  r.FormValue("reply_to_message_id"),
			"disable_notification": r.FormValue("disable_notification"),
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":3}}`))
	}))
	defer server.Close()

	bot := tgnotify.New("t")
	bot.SetBaseURL(server.URL)
	bot.SetRetryPolicy(tgnotify.RetryPolicy{Disabled: true})
	id, err := bot.SendFileOpts(context.Background(), "99", tgnotify.TypeDocument, path, &tgnotify.SendOptions{
		Caption:   "cap",
		ParseMode: "HTML",
		ReplyTo:   4,
		Silent:    true,
	})
	if err != nil {
		t.Fatalf("SendFileOpts: %v", err)
	}
	if id != 3 {
		t.Fatalf("id = %d, want 3", id)
	}
	if gotPath != "/bott/sendDocument" {
		t.Fatalf("path = %q", gotPath)
	}
	want := map[string]string{
		"chat_id":              "99",
		"caption":              "cap",
		"parse_mode":           "HTML",
		"reply_to_message_id":  "4",
		"disable_notification": "true",
	}
	for k, v := range want {
		if gotForm[k] != v {
			t.Errorf("form %s = %q, want %q", k, gotForm[k], v)
		}
	}
}

func TestRunCompletionPrintsScripts(t *testing.T) {
	for shell, want := range map[string]string{
		"bash": "complete -F _tg_notify",
		"zsh":  "#compdef tg-notify",
		"fish": "complete -c tg-notify",
	} {
		out := captureOutput(t, func() {
			if err := run([]string{"-A", shell}); err != nil {
				t.Errorf("run completion %s: %v", shell, err)
			}
		})
		if !strings.Contains(out, want) {
			t.Errorf("%s completion output missing %q", shell, want)
		}
	}
}

func TestPrintDryRunFileAllExtras(t *testing.T) {
	opts := options{caption: "cap", parseMode: "HTML", replyTo: 3, silent: true}
	out := captureOutput(t, func() { printDryRunFile(opts, "1", "file_id", "AgAC", "video") })
	want := "dry-run: send video file (file_id=AgAC) to 1, caption (3 chars), parse_mode=HTML, reply_to=3, silent"
	if out != want+"\n" {
		t.Fatalf("output = %q, want %q", out, want)
	}
}
