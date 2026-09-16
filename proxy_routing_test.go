package tgnotify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gitlab.com/lyoneel/tgnotify"
	"gitlab.com/lyoneel/tgnotify/internal/testproxy"
)

// scrubProxyEnv neutralises ambient proxy environment variables so
// the default transport can never leave loopback.
func scrubProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy",
		"NO_PROXY", "no_proxy", "TELEGRAM_PROXY",
	} {
		t.Setenv(k, "")
	}
}

// fakeAPI serves the Bot API envelopes the client decodes and counts
// every hit so tests can cross-check against proxy recordings.
type fakeAPI struct {
	server *httptest.Server
	hits   atomic.Int64
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
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
		case strings.HasSuffix(r.URL.Path, "/sendDocument"):
			_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":42}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeAPI) URL() string { return f.server.URL }

func (f *fakeAPI) Hits() int64 { return f.hits.Load() }

func (f *fakeAPI) HostPort(t *testing.T) (host string, port uint16) {
	t.Helper()
	u, err := url.Parse(f.server.URL)
	if err != nil {
		t.Fatalf("parse fake API URL: %v", err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse fake API port: %v", err)
	}
	return u.Hostname(), uint16(p)
}

func TestProxyRoutingHTTP(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy(proxySrv.URL()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	id, err := bot.SendMessage(context.Background(), "123", "hello", "", 0, false)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if id != 42 {
		t.Errorf("message id = %d, want 42", id)
	}

	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if rec.Count() != 1 {
		t.Fatalf("proxy exchange count = %d, want 1", rec.Count())
	}
	if api.Hits() != 1 {
		t.Fatalf("fake API hit count = %d, want 1", api.Hits())
	}
	if rec.Count() != int(api.Hits()) {
		t.Errorf("bypass cross-check: proxy exchanges %d != API hits %d", rec.Count(), api.Hits())
	}

	ex := rec.All()[0]
	wantURL := api.URL() + "/botTOKEN/sendMessage"
	if ex.URL != wantURL {
		t.Errorf("recorded URL = %q, want %q", ex.URL, wantURL)
	}
	if !strings.Contains(string(ex.RequestBody), `"text":"hello"`) {
		t.Errorf("recorded request body %q missing message text", ex.RequestBody)
	}
	if !strings.Contains(string(ex.ResponseBody), `message_id`) {
		t.Errorf("recorded response body %q missing message_id", ex.ResponseBody)
	}
	if ex.Status != http.StatusOK {
		t.Errorf("recorded status = %d, want 200", ex.Status)
	}
}

func TestProxyRoutingUpload(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy(proxySrv.URL()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	filePath := filepath.Join(t.TempDir(), "doc.txt")
	content := "upload payload for proxy test"
	if err := os.WriteFile(filePath, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	id, err := bot.SendFile(context.Background(), "123", tgnotify.TypeDocument, filePath, "a caption", "", 0, false)
	if err != nil {
		t.Fatalf("SendFile: %v", err)
	}
	if id != 42 {
		t.Errorf("message id = %d, want 42", id)
	}

	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if rec.Count() != int(api.Hits()) || api.Hits() != 1 {
		t.Fatalf("bypass cross-check: proxy exchanges %d, API hits %d, want 1/1", rec.Count(), api.Hits())
	}
	ex := rec.All()[0]
	if !strings.Contains(ex.URL, "/botTOKEN/sendDocument") {
		t.Errorf("recorded URL = %q, want sendDocument endpoint", ex.URL)
	}
	if !strings.Contains(string(ex.RequestBody), content) {
		t.Errorf("recorded multipart body missing file content")
	}
	if !strings.Contains(string(ex.RequestBody), "a caption") {
		t.Errorf("recorded multipart body missing caption field")
	}
}

func TestProxyRoutingGetMe(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy(proxySrv.URL()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	user, err := bot.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe: %v", err)
	}
	if user.Username != "testbot" {
		t.Errorf("username = %q, want testbot", user.Username)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	ex := rec.All()[0]
	if ex.Method != http.MethodGet {
		t.Errorf("recorded method = %q, want GET", ex.Method)
	}
	if rec.Count() != int(api.Hits()) {
		t.Errorf("bypass cross-check: proxy exchanges %d != API hits %d", rec.Count(), api.Hits())
	}
}

func TestProxyRoutingTLSWithSeam(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, rec, caPEM := testproxy.NewHTTPSProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxyTLSRootCAs(caPEM); err != nil {
		t.Fatalf("SetProxyTLSRootCAs: %v", err)
	}
	if err := bot.SetProxy(proxySrv.URL()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	if _, err := bot.SendMessage(context.Background(), "123", "tls", "", 0, false); err != nil {
		t.Fatalf("SendMessage through TLS proxy: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if rec.Count() != int(api.Hits()) || api.Hits() != 1 {
		t.Errorf("bypass cross-check: proxy exchanges %d, API hits %d, want 1/1", rec.Count(), api.Hits())
	}
	if ex := rec.All()[0]; !strings.Contains(string(ex.RequestBody), `"text":"tls"`) {
		t.Errorf("recorded request body %q missing message text", ex.RequestBody)
	}
}

func TestProxyRoutingTLSTrustedAfterSetProxy(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, _, caPEM := testproxy.NewHTTPSProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy(proxySrv.URL()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}
	if err := bot.SetProxyTLSRootCAs(caPEM); err != nil {
		t.Fatalf("SetProxyTLSRootCAs: %v", err)
	}

	if _, err := bot.SendMessage(context.Background(), "123", "order-independent", "", 0, false); err != nil {
		t.Fatalf("SendMessage with seam applied after SetProxy: %v", err)
	}
}

func TestProxyRoutingTLSUntrusted(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, _, _ := testproxy.NewHTTPSProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy(proxySrv.URL()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	_, err := bot.SendMessage(context.Background(), "123", "tls", "", 0, false)
	if err == nil {
		t.Fatalf("SendMessage through untrusted TLS proxy succeeded, want certificate error")
	}
	if !strings.Contains(err.Error(), "certificate") {
		t.Errorf("error = %v, want a certificate verification failure", err)
	}
	if api.Hits() != 0 {
		t.Errorf("fake API hit count = %d, want 0", api.Hits())
	}
}

func TestSetProxyTLSRootCAsRejectsBadPEM(t *testing.T) {
	bot := tgnotify.New("TOKEN")
	if err := bot.SetProxyTLSRootCAs([]byte("not a certificate")); err == nil {
		t.Fatalf("SetProxyTLSRootCAs with garbage PEM = nil, want error")
	}
}

func TestProxyRoutingSOCKS5(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	apiHost, apiPort := api.HostPort(t)
	proxySrv, rec := testproxy.NewSOCKS5Proxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy("socks5://" + proxySrv.Addr()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	if _, err := bot.SendMessage(context.Background(), "123", "socks", "", 0, false); err != nil {
		t.Fatalf("SendMessage through SOCKS5 proxy: %v", err)
	}

	handshakes := rec.Handshakes()
	if len(handshakes) != 1 {
		t.Fatalf("handshake count = %d, want 1", len(handshakes))
	}
	h := handshakes[0]
	if h.SelectedMethod != 0x00 {
		t.Errorf("selected method = %d, want 0 (no auth)", h.SelectedMethod)
	}
	if h.TargetHost != apiHost || h.TargetPort != apiPort {
		t.Errorf("handshake target = %s:%d, want %s:%d", h.TargetHost, h.TargetPort, apiHost, apiPort)
	}
	if err := rec.WaitForTunnels(1, 5*time.Second); err != nil {
		t.Fatalf("tunnel recording: %v", err)
	}
	tunnel := rec.Tunnels()[0]
	waitForSubstring(t, func() string { return string(tunnel.ClientBytes()) }, `"text":"socks"`)
	waitForSubstring(t, func() string { return string(tunnel.TargetBytes()) }, `message_id`)
	if api.Hits() != 1 {
		t.Errorf("fake API hit count = %d, want 1", api.Hits())
	}
}

// waitForSubstring polls a getter until its output contains want or
// the timeout elapses; SOCKS tunnel dumps fill live while the
// connection stays open.
func waitForSubstring(t *testing.T, get func() string, want string) {
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

func TestProxyRoutingSOCKS5WithAuth(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, rec := testproxy.NewSOCKS5Proxy(t, testproxy.WithUserPass("socksuser", "sockspass"))

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy(proxySrv.URLWithAuth("socksuser", "sockspass")); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	if _, err := bot.SendMessage(context.Background(), "123", "auth", "", 0, false); err != nil {
		t.Fatalf("SendMessage through authenticated SOCKS5 proxy: %v", err)
	}

	handshakes := rec.Handshakes()
	if len(handshakes) == 0 {
		t.Fatalf("no handshake recorded")
	}
	h := handshakes[len(handshakes)-1]
	if h.SelectedMethod != 0x02 {
		t.Errorf("selected method = %d, want 2 (username/password)", h.SelectedMethod)
	}
	if h.Username != "socksuser" || h.Password != "sockspass" {
		t.Errorf("recorded credentials = %q/%q, want socksuser/sockspass", h.Username, h.Password)
	}
}

func TestProxyRoutingSOCKS5hEquivalent(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	apiHost, apiPort := api.HostPort(t)
	proxySrv, rec := testproxy.NewSOCKS5Proxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy("socks5h://" + proxySrv.Addr()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	if _, err := bot.SendMessage(context.Background(), "123", "socks5h", "", 0, false); err != nil {
		t.Fatalf("SendMessage through socks5h proxy: %v", err)
	}

	handshakes := rec.Handshakes()
	if len(handshakes) != 1 {
		t.Fatalf("handshake count = %d, want 1", len(handshakes))
	}
	h := handshakes[0]
	if h.TargetHost != apiHost || h.TargetPort != apiPort {
		t.Errorf("handshake target = %s:%d, want %s:%d", h.TargetHost, h.TargetPort, apiHost, apiPort)
	}
	if api.Hits() != 1 {
		t.Errorf("fake API hit count = %d, want 1", api.Hits())
	}
}

func TestNoProxyControl(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	_ = proxySrv

	if _, err := bot.SendMessage(context.Background(), "123", "direct", "", 0, false); err != nil {
		t.Fatalf("SendMessage without proxy: %v", err)
	}
	if rec.Count() != 0 {
		t.Errorf("proxy exchange count = %d, want 0 (bypass detected)", rec.Count())
	}
	if api.Hits() != 1 {
		t.Errorf("fake API hit count = %d, want 1", api.Hits())
	}
}

func TestSetProxyEmptyReset(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	bot := tgnotify.New("TOKEN")
	bot.SetBaseURL(api.URL())
	if err := bot.SetProxy(proxySrv.URL()); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}
	if _, err := bot.SendMessage(context.Background(), "123", "proxied", "", 0, false); err != nil {
		t.Fatalf("first SendMessage: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}

	if err := bot.SetProxy(""); err != nil {
		t.Fatalf("SetProxy(\"\"): %v", err)
	}
	if _, err := bot.SendMessage(context.Background(), "123", "direct again", "", 0, false); err != nil {
		t.Fatalf("second SendMessage: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	if rec.Count() != 1 {
		t.Errorf("proxy exchange count = %d after reset, want 1 (second request must bypass the proxy)", rec.Count())
	}
	if api.Hits() != 2 {
		t.Errorf("fake API hit count = %d, want 2", api.Hits())
	}
}
