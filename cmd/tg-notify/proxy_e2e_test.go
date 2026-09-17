package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitlab.com/lyoneel/tg-notify/internal/testproxy"
)

// proxyEnvVars are every environment variable that can route traffic
// through a proxy or change the Bot API endpoint; tests scrub all of
// them before starting servers so runs stay hermetic.
var proxyEnvVars = []string{
	"HTTP_PROXY", "http_proxy",
	"HTTPS_PROXY", "https_proxy",
	"NO_PROXY", "no_proxy",
	"TELEGRAM_PROXY",
	"TELEGRAM_BOT_TOKEN", "TELEGRAM_CHAT_ID", "TELEGRAM_BASE_URL",
}

func scrubProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range proxyEnvVars {
		t.Setenv(k, "")
	}
}

// captureStdout redirects os.Stdout into a pipe for the duration of
// the test. The returned getter flushes the pipe and returns
// everything printed to stdout so far; calling it restores os.Stdout.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.Copy(&buf, r)
	}()

	flush := func() string {
		if os.Stdout == w {
			os.Stdout = old
			_ = w.Close()
			<-done
		}
		return buf.String()
	}
	t.Cleanup(func() { _ = flush() })
	return flush
}

// fakeBotAPI serves the exact envelopes the client decodes for every
// endpoint the CLI exercises, and counts hits so tests can cross-check
// against proxy recordings (bypass detection).
type fakeBotAPI struct {
	server *httptest.Server
	hits   atomic.Int64
}

func newFakeBotAPI(t *testing.T) *fakeBotAPI {
	t.Helper()
	f := &fakeBotAPI{}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.hits.Add(1)
		// Close every connection after responding so SOCKS tunnels are
		// torn down promptly and their byte dumps get recorded.
		w.Header().Set("Connection", "close")
		switch {
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"id":1,"is_bot":true,"first_name":"TestBot","username":"testbot"}}`))
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"message":{"chat":{"id":987}}}]}`))
		case strings.HasSuffix(r.URL.Path, "/sendMediaGroup"):
			_, _ = w.Write([]byte(`{"ok":true,"result":[{"message_id":1},{"message_id":2}]}`))
		case strings.HasSuffix(r.URL.Path, "/sendPhoto"),
			strings.HasSuffix(r.URL.Path, "/sendDocument"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeBotAPI) URL() string { return f.server.URL }

func (f *fakeBotAPI) Hits() int64 { return f.hits.Load() }

// commonFlags are the flags every positive e2e invocation shares.
func commonFlags(apiURL, proxyURL string) []string {
	return []string{
		"--token", "TESTTOKEN",
		"--chat-id", "123",
		"--base-url", apiURL,
		"--proxy", proxyURL,
		"--no-retry",
	}
}

func TestProxyE2EMessage(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)
	getStdout := captureStdout(t)

	args := append(commonFlags(api.URL(), proxySrv.URL()), "hello via proxy")
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}

	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if out := getStdout(); out != "Sent (message_id: 42)\n" {
		t.Errorf("stdout = %q, want %q", out, "Sent (message_id: 42)\n")
	}
	if rec.Count() != 1 || api.Hits() != 1 {
		t.Errorf("bypass cross-check: proxy exchanges %d, API hits %d, want 1/1", rec.Count(), api.Hits())
	}
	ex := rec.All()[0]
	if ex.URL != api.URL()+"/botTESTTOKEN/sendMessage" {
		t.Errorf("recorded URL = %q, want sendMessage endpoint", ex.URL)
	}
	if !strings.Contains(string(ex.RequestBody), `"text":"hello via proxy"`) {
		t.Errorf("recorded request body %q missing message text", ex.RequestBody)
	}
	if !strings.Contains(string(ex.ResponseBody), `"message_id":42`) {
		t.Errorf("recorded response body %q missing message_id", ex.ResponseBody)
	}
}

func TestProxyE2EMessageJSON(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)
	getStdout := captureStdout(t)

	args := append(commonFlags(api.URL(), proxySrv.URL()), "--json", "json output")
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if out := getStdout(); out != "{\"ok\":true,\"message_id\":42}\n" {
		t.Errorf("stdout = %q, want JSON success line", out)
	}
}

func TestProxyE2EWhoami(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)
	getStdout := captureStdout(t)

	args := []string{
		"--whoami",
		"--token", "TESTTOKEN",
		"--base-url", api.URL(),
		"--proxy", proxySrv.URL(),
		"--no-retry",
	}
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if out := getStdout(); out != "@testbot (id: 1)\n" {
		t.Errorf("stdout = %q, want bot identity line", out)
	}
	if ex := rec.All()[0]; ex.Method != http.MethodGet {
		t.Errorf("recorded method = %q, want GET", ex.Method)
	}
	if rec.Count() != int(api.Hits()) {
		t.Errorf("bypass cross-check: proxy exchanges %d != API hits %d", rec.Count(), api.Hits())
	}
}

func TestProxyE2EDiscover(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)
	getStdout := captureStdout(t)

	args := []string{
		"--discover-chat-id",
		"--token", "TESTTOKEN",
		"--base-url", api.URL(),
		"--proxy", proxySrv.URL(),
		"--no-retry",
	}
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if out := getStdout(); out != "987\n" {
		t.Errorf("stdout = %q, want 987", out)
	}
	if ex := rec.All()[0]; !strings.Contains(ex.URL, "/botTESTTOKEN/getUpdates") {
		t.Errorf("recorded URL = %q, want getUpdates endpoint", ex.URL)
	}
}

func TestProxyE2EFileUpload(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)
	getStdout := captureStdout(t)

	filePath := t.TempDir() + "/doc.txt"
	if err := os.WriteFile(filePath, []byte("uploaded file content"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	args := append(commonFlags(api.URL(), proxySrv.URL()),
		"--file", filePath, "positional caption")
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if out := getStdout(); out != "Sent (message_id: 42)\n" {
		t.Errorf("stdout = %q, want success line", out)
	}
	if rec.Count() != int(api.Hits()) || api.Hits() != 1 {
		t.Fatalf("bypass cross-check: proxy exchanges %d, API hits %d, want 1/1", rec.Count(), api.Hits())
	}
	ex := rec.All()[0]
	if !strings.Contains(ex.URL, "/botTESTTOKEN/sendDocument") {
		t.Errorf("recorded URL = %q, want sendDocument endpoint", ex.URL)
	}
	if !strings.Contains(string(ex.RequestBody), "uploaded file content") {
		t.Errorf("recorded multipart body missing file content")
	}
	if !strings.Contains(string(ex.RequestBody), "positional caption") {
		t.Errorf("recorded multipart body missing caption field")
	}
}

func TestProxyE2EFileByURL(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)
	getStdout := captureStdout(t)

	args := append(commonFlags(api.URL(), proxySrv.URL()),
		"--url", "http://127.0.0.1:9/pic.png", "--type", "photo")
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if out := getStdout(); out != "Sent (message_id: 42)\n" {
		t.Errorf("stdout = %q, want success line", out)
	}
	ex := rec.All()[0]
	if !strings.Contains(ex.URL, "/botTESTTOKEN/sendPhoto") {
		t.Errorf("recorded URL = %q, want sendPhoto endpoint", ex.URL)
	}
	if !strings.Contains(string(ex.RequestBody), `"photo":"http://127.0.0.1:9/pic.png"`) {
		t.Errorf("recorded request body %q missing photo url field", ex.RequestBody)
	}
}

func TestProxyE2EAlbum(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)
	getStdout := captureStdout(t)

	dir := t.TempDir()
	aPath := dir + "/a.jpg"
	bPath := dir + "/b.jpg"
	for _, p := range []struct{ path, content string }{
		{aPath, "first photo bytes"},
		{bPath, "second photo bytes"},
	} {
		if err := os.WriteFile(p.path, []byte(p.content), 0o600); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}

	args := append(commonFlags(api.URL(), proxySrv.URL()),
		"--album", aPath, "--album", bPath)
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if out := getStdout(); out != "Sent album (2 messages)\n" {
		t.Errorf("stdout = %q, want album success line", out)
	}
	if rec.Count() != int(api.Hits()) || api.Hits() != 1 {
		t.Fatalf("bypass cross-check: proxy exchanges %d, API hits %d, want 1/1", rec.Count(), api.Hits())
	}
	ex := rec.All()[0]
	if !strings.Contains(ex.URL, "/botTESTTOKEN/sendMediaGroup") {
		t.Errorf("recorded URL = %q, want sendMediaGroup endpoint", ex.URL)
	}
	body := string(ex.RequestBody)
	for _, want := range []string{"first photo bytes", "second photo bytes", "attach://file0", "attach://file1"} {
		if !strings.Contains(body, want) {
			t.Errorf("recorded multipart body missing %q", want)
		}
	}
}

func TestProxyE2ESOCKS5(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewSOCKS5Proxy(t)
	getStdout := captureStdout(t)

	args := append(commonFlags(api.URL(), "socks5://"+proxySrv.Addr()), "via socks")
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if out := getStdout(); out != "Sent (message_id: 42)\n" {
		t.Errorf("stdout = %q, want success line", out)
	}

	handshakes := rec.Handshakes()
	if len(handshakes) != 1 {
		t.Fatalf("handshake count = %d, want 1", len(handshakes))
	}
	if h := handshakes[0]; h.SelectedMethod != 0x00 {
		t.Errorf("selected method = %d, want 0 (no auth)", h.SelectedMethod)
	}
	if err := rec.WaitForTunnels(1, 5*time.Second); err != nil {
		t.Fatalf("tunnel recording: %v", err)
	}
	tunnel := rec.Tunnels()[0]
	waitForCLIBytes(t, func() string { return string(tunnel.ClientBytes()) }, `"text":"via socks"`)
	waitForCLIBytes(t, func() string { return string(tunnel.TargetBytes()) }, `"message_id":42`)
	if api.Hits() != 1 {
		t.Errorf("fake API hit count = %d, want 1", api.Hits())
	}
}

// waitForCLIBytes polls a getter until its output contains want or
// the timeout elapses; SOCKS tunnel dumps fill live while the
// connection stays open.
func waitForCLIBytes(t *testing.T, get func() string, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(get(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("dump never contained %q; got %q", want, get())
}
