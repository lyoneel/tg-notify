package tgnotify_test

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/lyoneel/tgnotify"
)

// capture returns a handler that records the request path and the
// JSON body, then answers with a single-message result.
func capture(t *testing.T, gotPath *string, gotBody *map[string]any) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.URL.Path
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		var body map[string]any
		if err := json.Unmarshal(data, &body); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		*gotBody = body
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":11}}`))
	}
}

func newCapturingServer(t *testing.T, gotPath *string, gotBody *map[string]any) *tgnotify.Bot {
	t.Helper()
	server := httptest.NewServer(capture(t, gotPath, gotBody))
	t.Cleanup(server.Close)
	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(server.URL)
	return bot
}

func TestSendAPIFormsEquivalent(t *testing.T) {
	var positionalPath, optsPath, funcPath string
	var positionalBody, optsBody, funcBody map[string]any

	botP := newCapturingServer(t, &positionalPath, &positionalBody)
	botO := newCapturingServer(t, &optsPath, &optsBody)
	botF := newCapturingServer(t, &funcPath, &funcBody)

	ctx := context.Background()
	if _, err := botP.SendMessage(ctx, "123", "hello", "HTML", 42, true); err != nil {
		t.Fatalf("positional send: %v", err)
	}
	if _, err := botO.SendMessageOpts(ctx, "123", "hello", &tgnotify.SendOptions{
		ParseMode: "HTML",
		ReplyTo:   42,
		Silent:    true,
	}); err != nil {
		t.Fatalf("opts send: %v", err)
	}
	if _, err := botF.SendMessageOpts(ctx, "123", "hello",
		tgnotify.NewSendOptions(tgnotify.WithParseMode("HTML"), tgnotify.WithReplyTo(42), tgnotify.WithSilent())); err != nil {
		t.Fatalf("functional send: %v", err)
	}

	if positionalPath != optsPath || optsPath != funcPath {
		t.Fatalf("paths differ: %q %q %q", positionalPath, optsPath, funcPath)
	}
	if !reflect.DeepEqual(positionalBody, optsBody) || !reflect.DeepEqual(optsBody, funcBody) {
		t.Fatalf("bodies differ:\npositional %+v\nopts %+v\nfunc %+v", positionalBody, optsBody, funcBody)
	}
	want := map[string]any{
		"chat_id":              "123",
		"text":                 "hello",
		"parse_mode":           "HTML",
		"reply_to_message_id":  float64(42),
		"disable_notification": true,
	}
	if !reflect.DeepEqual(positionalBody, want) {
		t.Fatalf("body = %+v, want %+v", positionalBody, want)
	}
}

func TestNilOptionsEqualsZeroOptions(t *testing.T) {
	var nilBody, zeroBody map[string]any
	botN := newCapturingServer(t, nil, &nilBody)
	botZ := newCapturingServer(t, nil, &zeroBody)

	ctx := context.Background()
	if _, err := botN.SendMessageOpts(ctx, "1", "hi", nil); err != nil {
		t.Fatalf("nil opts: %v", err)
	}
	if _, err := botZ.SendMessageOpts(ctx, "1", "hi", &tgnotify.SendOptions{}); err != nil {
		t.Fatalf("zero opts: %v", err)
	}
	if !reflect.DeepEqual(nilBody, zeroBody) {
		t.Fatalf("nil and zero options differ: %+v vs %+v", nilBody, zeroBody)
	}
	if _, ok := nilBody["parse_mode"]; ok {
		t.Fatalf("zero options must omit parse_mode, got %+v", nilBody)
	}
}

func TestEmptyOptionListEqualsNil(t *testing.T) {
	var emptyBody, nilBody map[string]any
	botE := newCapturingServer(t, nil, &emptyBody)
	botN := newCapturingServer(t, nil, &nilBody)

	ctx := context.Background()
	if _, err := botE.SendMessageOpts(ctx, "1", "hi", tgnotify.NewSendOptions()); err != nil {
		t.Fatalf("empty option list: %v", err)
	}
	if _, err := botN.SendMessageOpts(ctx, "1", "hi", nil); err != nil {
		t.Fatalf("nil opts: %v", err)
	}
	if !reflect.DeepEqual(emptyBody, nilBody) {
		t.Fatalf("empty list and nil differ: %+v vs %+v", emptyBody, nilBody)
	}
}

func TestLibraryValidation(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":1}}`))
	}))
	defer server.Close()
	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(server.URL)
	ctx := context.Background()

	long := strings.Repeat("a", tgnotify.MaxMessageRunes+1)
	if _, err := bot.SendMessageOpts(ctx, "1", long, nil); err == nil {
		t.Fatal("oversized message must fail client-side")
	}
	if _, err := bot.SendMessageOpts(ctx, "1", "", nil); err == nil {
		t.Fatal("empty message must fail client-side")
	}

	longCaption := strings.Repeat("c", tgnotify.MaxCaptionRunes+1)
	if _, err := bot.SendFileOpts(ctx, "1", tgnotify.TypePhoto, "x.png",
		tgnotify.NewSendOptions(tgnotify.WithCaption(longCaption))); err == nil {
		t.Fatal("oversized caption must fail client-side")
	}
	if called {
		t.Fatal("server must not be called on validation failure")
	}
}

func TestClientScrubsTokenInTransportErrors(t *testing.T) {
	// A listener that is already closed produces a connection-refused
	// transport error; the URL inside must never carry the token.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()

	bot := tgnotify.New("SECRET-TOKEN-123")
	bot.SetBaseURL("http://" + addr)
	bot.SetRetryPolicy(tgnotify.RetryPolicy{Disabled: true})
	if _, err := bot.SendMessageOpts(context.Background(), "1", "hi", nil); err == nil {
		t.Fatal("expected transport error")
	} else if strings.Contains(err.Error(), "SECRET-TOKEN-123") {
		t.Fatalf("error leaks token: %s", err)
	}
}

func TestOptionsAppliedToLocalFileUpload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	var gotPath string
	var gotForm map[string]string
	bot, _ := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart form: %v", err)
		}
		gotForm = map[string]string{
			"caption":              r.FormValue("caption"),
			"parse_mode":           r.FormValue("parse_mode"),
			"reply_to_message_id":  r.FormValue("reply_to_message_id"),
			"disable_notification": r.FormValue("disable_notification"),
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":7}}`))
	})

	if _, err := bot.SendFileOpts(context.Background(), "55", tgnotify.TypeDocument, path,
		tgnotify.NewSendOptions(tgnotify.WithCaption("cap"), tgnotify.WithSilent())); err != nil {
		t.Fatalf("SendFileOpts: %v", err)
	}
	if gotPath != "/botTOKEN/sendDocument" {
		t.Fatalf("path = %q", gotPath)
	}
	want := map[string]string{
		"caption":              "cap",
		"parse_mode":           "",
		"reply_to_message_id":  "",
		"disable_notification": "true",
	}
	for k, v := range want {
		if gotForm[k] != v {
			t.Fatalf("form %s = %q, want %q", k, gotForm[k], v)
		}
	}
}
