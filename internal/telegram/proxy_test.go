package telegram_test

import (
	"testing"

	"gitlab.com/lyoneel/cli-tg-notify/internal/telegram"
)

func TestSetProxy(t *testing.T) {
	tests := []struct {
		name     string
		proxyURL string
		wantErr  bool
	}{
		{name: "empty disables", proxyURL: ""},
		{name: "http scheme", proxyURL: "http://127.0.0.1:8080"},
		{name: "https scheme", proxyURL: "https://127.0.0.1:8443"},
		{name: "http with userinfo", proxyURL: "http://user:pass@127.0.0.1:8080"},
		{name: "socks5 scheme", proxyURL: "socks5://127.0.0.1:1080"},
		{name: "socks5 with auth", proxyURL: "socks5://user:pass@127.0.0.1:1080"},
		{name: "socks5h scheme", proxyURL: "socks5h://127.0.0.1:1080"},
		{name: "socks5 default port", proxyURL: "socks5://127.0.0.1"},
		{name: "unsupported scheme", proxyURL: "ftp://127.0.0.1:21", wantErr: true},
		{name: "missing scheme", proxyURL: "127.0.0.1:8080", wantErr: true},
		{name: "missing host", proxyURL: "http://", wantErr: true},
		{name: "malformed url", proxyURL: "://bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bot := telegram.New("TOKEN")
			err := bot.SetProxy(tt.proxyURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("SetProxy(%q) = nil, want error", tt.proxyURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetProxy(%q) = %v, want nil", tt.proxyURL, err)
			}
		})
	}
}
