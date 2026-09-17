package main

import (
	"strings"
	"testing"
	"time"

	"gitlab.com/lyoneel/tg-notify/internal/testproxy"
)

func TestProxyE2ENegativeBadScheme(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	args := append(commonFlags(api.URL(), proxySrv.URL()), "text")
	// Override the proxy with an unsupported scheme.
	for i, a := range args {
		if a == "--proxy" {
			args[i+1] = "ftp://127.0.0.1:21"
		}
	}
	err := run(args)
	if err == nil {
		t.Fatalf("run with ftp:// proxy = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported proxy scheme") {
		t.Errorf("error = %v, want unsupported proxy scheme", err)
	}
	if rec.Count() != 0 || api.Hits() != 0 {
		t.Errorf("traffic leaked: proxy exchanges %d, API hits %d, want 0/0", rec.Count(), api.Hits())
	}
}

func TestProxyE2ENegativeDeadProxy(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)

	start := time.Now()
	args := append(commonFlags(api.URL(), "http://127.0.0.1:1"), "text")
	err := run(args)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("run with dead proxy = nil, want network error")
	}
	if elapsed > 10*time.Second {
		t.Errorf("dead proxy took %s, want a fast failure (--no-retry)", elapsed)
	}
	if api.Hits() != 0 {
		t.Errorf("fake API hit count = %d, want 0", api.Hits())
	}
}

func TestProxyE2EEnvVarRouting(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	t.Setenv("TELEGRAM_PROXY", proxySrv.URL())
	args := []string{
		"--token", "TESTTOKEN",
		"--chat-id", "123",
		"--base-url", api.URL(),
		"--no-retry",
		"env-routed message",
	}
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("TELEGRAM_PROXY env did not route traffic: proxy exchanges = %d, want 1", rec.Count())
	}
}

func TestProxyE2EFlagOverridesEnv(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	envProxy, envRec := testproxy.NewHTTPProxy(t)
	flagProxy, flagRec := testproxy.NewHTTPProxy(t)

	t.Setenv("TELEGRAM_PROXY", envProxy.URL())
	args := append(commonFlags(api.URL(), flagProxy.URL()), "flag wins")
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := flagRec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if flagRec.Count() != 1 {
		t.Errorf("flag proxy exchanges = %d, want 1", flagRec.Count())
	}
	if envRec.Count() != 0 {
		t.Errorf("env proxy recorded %d exchanges, want 0 (flag must win)", envRec.Count())
	}
}

func TestProxyE2EAmbientHTTPProxyLoopbackBypass(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	// No TELEGRAM_PROXY and no --proxy: the default transport honours
	// the ambient HTTP_PROXY variable, but Go's env-proxy
	// implementation bypasses loopback targets. The test API is
	// loopback-only, so traffic must go direct and the recorder must
	// stay empty; this proves an ambient proxy can never silently
	// divert tg-notify traffic on this machine's own services.
	t.Setenv("HTTP_PROXY", proxySrv.URL())
	args := []string{
		"--token", "TESTTOKEN",
		"--chat-id", "123",
		"--base-url", api.URL(),
		"--no-retry",
		"ambient proxy message",
	}
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if rec.Count() != 0 {
		t.Errorf("ambient HTTP_PROXY diverted loopback traffic: proxy exchanges = %d, want 0", rec.Count())
	}
	if api.Hits() != 1 {
		t.Errorf("fake API hit count = %d, want 1", api.Hits())
	}
}

func TestProxyE2EExplicitProxyBeatsAmbientHTTPSProxy(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, rec := testproxy.NewHTTPProxy(t)

	// An ambient HTTPS_PROXY pointing at a dead port must not interfere
	// with an explicit --proxy over a plain-HTTP base URL.
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	args := append(commonFlags(api.URL(), proxySrv.URL()), "explicit wins")
	if err := run(args); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := rec.WaitFor(1, 5*time.Second); err != nil {
		t.Fatalf("proxy recording: %v", err)
	}
	if rec.Count() != 1 {
		t.Errorf("explicit proxy exchanges = %d, want 1", rec.Count())
	}
}

func TestProxyE2ESOCKS5WrongPassword(t *testing.T) {
	scrubProxyEnv(t)
	api := newFakeBotAPI(t)
	proxySrv, _ := testproxy.NewSOCKS5Proxy(t, testproxy.WithUserPass("socksuser", "sockspass"))

	args := append(commonFlags(api.URL(), proxySrv.URLWithAuth("socksuser", "wrong")), "auth fail")
	err := run(args)
	if err == nil {
		t.Fatalf("run with wrong SOCKS5 password = nil, want error")
	}
	if api.Hits() != 0 {
		t.Errorf("fake API hit count = %d, want 0", api.Hits())
	}
}
