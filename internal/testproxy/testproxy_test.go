package testproxy_test

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/proxy"

	"gitlab.com/lyoneel/tg-notify/internal/testproxy"
)

// scrubProxyEnv neutralises ambient proxy environment variables so
// the default transport never leaves loopback.
func scrubProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(k, "")
	}
}

// startTarget returns an httptest target server that echoes the
// request method and body into its response.
func startTarget(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("target: read body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Target", "echo")
		_, _ = w.Write([]byte("method=" + r.Method + ";body=" + string(body)))
	}))
	t.Cleanup(server.Close)
	return server
}

// proxiedClient returns an http.Client routing through the given
// proxy URL with the default transport.
func proxiedClient(proxyURL string) *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyURL(mustURL(proxyURL)),
	}}
}

func mustURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestHTTPProxyRecordsExchange(t *testing.T) {
	scrubProxyEnv(t)
	target := startTarget(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	client := proxiedClient(proxySrv.URL())
	resp, err := client.Post(target.URL+"/sendMessage", "application/json", strings.NewReader(`{"text":"hi"}`))
	if err != nil {
		t.Fatalf("POST through proxy: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	if rec.Count() != 1 {
		t.Fatalf("recorder count = %d, want 1", rec.Count())
	}
	ex := rec.All()[0]
	if ex.Method != http.MethodPost {
		t.Errorf("recorded method = %q, want POST", ex.Method)
	}
	if ex.URL != target.URL+"/sendMessage" {
		t.Errorf("recorded URL = %q, want %q", ex.URL, target.URL+"/sendMessage")
	}
	if string(ex.RequestBody) != `{"text":"hi"}` {
		t.Errorf("recorded request body = %q, want JSON payload", ex.RequestBody)
	}
	if ex.Status != http.StatusOK {
		t.Errorf("recorded status = %d, want 200", ex.Status)
	}
	if !strings.Contains(string(ex.ResponseBody), "body={\"text\":\"hi\"}") {
		t.Errorf("recorded response body = %q, want echoed payload", ex.ResponseBody)
	}
	if string(body) != string(ex.ResponseBody) {
		t.Errorf("client body %q differs from recorded response %q", body, ex.ResponseBody)
	}
}

func TestHTTPProxyBasicAuth(t *testing.T) {
	scrubProxyEnv(t)
	target := startTarget(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t, testproxy.WithBasicAuth("puser", "ppass"))
	client := proxiedClient(proxySrv.URL())

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantCount  int
	}{
		{name: "missing credentials", authHeader: "", wantStatus: http.StatusProxyAuthRequired, wantCount: 1},
		{
			name:       "wrong credentials",
			authHeader: "Basic " + base64.StdEncoding.EncodeToString([]byte("puser:wrong")),
			wantStatus: http.StatusProxyAuthRequired,
			wantCount:  2,
		},
		{
			name:       "valid credentials",
			authHeader: "Basic " + base64.StdEncoding.EncodeToString([]byte("puser:ppass")),
			wantStatus: http.StatusOK,
			wantCount:  3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, target.URL+"/", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			if tt.authHeader != "" {
				req.Header.Set("Proxy-Authorization", tt.authHeader)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if rec.Count() != tt.wantCount {
				t.Fatalf("recorder count = %d, want %d", rec.Count(), tt.wantCount)
			}
		})
	}

	ex := rec.All()[0]
	if ex.Status != http.StatusProxyAuthRequired {
		t.Errorf("first recorded exchange status = %d, want 407", ex.Status)
	}
	if ex.RequestHeader.Get("Proxy-Authorization") != "" {
		t.Errorf("missing-auth exchange carries unexpected Proxy-Authorization header")
	}
}

func TestTLSProxyRecordsExchange(t *testing.T) {
	scrubProxyEnv(t)
	target := startTarget(t)
	proxySrv, rec, caPEM := testproxy.NewHTTPSProxy(t)

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		t.Fatalf("failed to parse proxy CA certificate")
	}
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyURL(mustURL(proxySrv.URL())),
		TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
	}}

	resp, err := client.Get(target.URL + "/getMe")
	if err != nil {
		t.Fatalf("GET through TLS proxy: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if rec.Count() != 1 {
		t.Fatalf("recorder count = %d, want 1", rec.Count())
	}
	ex := rec.All()[0]
	if ex.URL != target.URL+"/getMe" || ex.Method != http.MethodGet || ex.Status != http.StatusOK {
		t.Errorf("recorded exchange = %+v, want GET %s 200", ex, target.URL+"/getMe")
	}
}

func TestTLSProxyUntrustedWithoutCA(t *testing.T) {
	scrubProxyEnv(t)
	target := startTarget(t)
	proxySrv, _, _ := testproxy.NewHTTPSProxy(t)

	client := &http.Client{Transport: &http.Transport{
		Proxy: http.ProxyURL(mustURL(proxySrv.URL())),
	}}
	if _, err := client.Get(target.URL + "/getMe"); err == nil {
		t.Fatalf("GET through untrusted TLS proxy succeeded, want certificate error")
	} else if !strings.Contains(err.Error(), "certificate") {
		t.Errorf("error = %v, want a certificate verification failure", err)
	}
}

// socksDialer builds a proxy dialer via golang.org/x/net so the test
// exercises the same dialer stack the CLI uses.
func socksDialer(t *testing.T, proxyURL string) proxy.ContextDialer {
	t.Helper()
	u := mustURL(proxyURL)
	d, err := proxy.FromURL(u, proxy.Direct)
	if err != nil {
		t.Fatalf("proxy.FromURL(%q): %v", proxyURL, err)
	}
	cd, ok := d.(proxy.ContextDialer)
	if !ok {
		t.Fatalf("dialer for %q does not support context dialing", proxyURL)
	}
	return cd
}

func TestSOCKS5ProxyNoAuth(t *testing.T) {
	scrubProxyEnv(t)
	target := startTarget(t)
	proxySrv, rec := testproxy.NewSOCKS5Proxy(t)

	client := &http.Client{Transport: &http.Transport{
		DialContext:       socksDialer(t, proxySrv.URL()).DialContext,
		DisableKeepAlives: true,
	}}
	resp, err := client.Post(target.URL+"/sendMessage", "application/json", bytes.NewBufferString(`{"text":"socks"}`))
	if err != nil {
		t.Fatalf("POST through SOCKS5 proxy: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read response: %v", err)
	}

	handshakes := rec.Handshakes()
	if len(handshakes) != 1 {
		t.Fatalf("handshake count = %d, want 1", len(handshakes))
	}
	h := handshakes[0]
	if h.Version != 0x05 {
		t.Errorf("handshake version = %d, want 5", h.Version)
	}
	if h.SelectedMethod != 0x00 {
		t.Errorf("selected method = %d, want 0 (no auth)", h.SelectedMethod)
	}
	if h.TargetHost != "127.0.0.1" {
		t.Errorf("target host = %q, want 127.0.0.1", h.TargetHost)
	}
	wantPort, err := strconv.Atoi(mustURL(target.URL).Port())
	if err != nil {
		t.Fatalf("parse target port: %v", err)
	}
	if int(h.TargetPort) != wantPort {
		t.Errorf("target port = %d, want %d", h.TargetPort, wantPort)
	}

	if err := rec.WaitForTunnels(1, 5*time.Second); err != nil {
		t.Fatalf("tunnel recording: %v", err)
	}
	tunnel := rec.Tunnels()[0]
	waitForBytes(t, func() string { return string(tunnel.ClientBytes()) }, `{"text":"socks"}`)
	waitForBytes(t, func() string { return string(tunnel.TargetBytes()) }, string(body))
}

// waitForBytes polls a byte-dump getter until it contains want or the
// timeout elapses; tunnel dumps fill live while the connection stays
// open.
func waitForBytes(t *testing.T, get func() string, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(get(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("byte dump never contained %q; got %q", want, get())
}

func TestSOCKS5ProxyUserPassAuth(t *testing.T) {
	scrubProxyEnv(t)
	target := startTarget(t)
	proxySrv, rec := testproxy.NewSOCKS5Proxy(t, testproxy.WithUserPass("socksuser", "sockspass"))

	t.Run("valid credentials", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{
			DialContext: socksDialer(t, proxySrv.URLWithAuth("socksuser", "sockspass")).DialContext,
		}}
		resp, err := client.Get(target.URL + "/")
		if err != nil {
			t.Fatalf("GET through authenticated SOCKS5 proxy: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

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
	})

	t.Run("wrong password rejected", func(t *testing.T) {
		client := &http.Client{Transport: &http.Transport{
			DialContext: socksDialer(t, proxySrv.URLWithAuth("socksuser", "wrong")).DialContext,
		}}
		if _, err := client.Get(target.URL + "/"); err == nil {
			t.Fatalf("GET with wrong SOCKS5 password succeeded, want auth failure")
		}
	})
}
