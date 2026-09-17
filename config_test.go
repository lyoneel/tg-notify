package tgnotify_test

import (
	"testing"

	"gitlab.com/lyoneel/tg-notify"
)

func TestFromEnv(t *testing.T) {
	set := func(t *testing.T, key, value string) {
		t.Helper()
		t.Setenv(key, value)
	}

	tests := []struct {
		name     string
		env      map[string]string
		wantErr  string
		wantChat string
		wantBase string
	}{
		{
			name:    "missing token fails",
			env:     map[string]string{tgnotify.EnvChatID: "42"},
			wantErr: "bot token required: set " + tgnotify.EnvToken,
		},
		{
			name:    "missing chat id fails",
			env:     map[string]string{tgnotify.EnvToken: "t"},
			wantErr: "chat ID required: set " + tgnotify.EnvChatID,
		},
		{
			name: "token and chat id only",
			env: map[string]string{
				tgnotify.EnvToken:  "tok",
				tgnotify.EnvChatID: "-100",
			},
			wantChat: "-100",
		},
		{
			name: "base url applied",
			env: map[string]string{
				tgnotify.EnvToken:   "tok",
				tgnotify.EnvChatID:  "42",
				tgnotify.EnvBaseURL: "https://example.org",
			},
			wantChat: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Neutralize any real environment values first.
			for _, key := range []string{tgnotify.EnvToken, tgnotify.EnvChatID, tgnotify.EnvBaseURL, tgnotify.EnvProxy} {
				t.Setenv(key, "")
			}
			for key, value := range tt.env {
				set(t, key, value)
			}
			bot, chatID, err := tgnotify.FromEnv()
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromEnv: %v", err)
			}
			if chatID != tt.wantChat {
				t.Errorf("chatID = %q, want %q", chatID, tt.wantChat)
			}
			if bot == nil {
				t.Fatal("bot is nil")
			}
		})
	}
}

func TestFromEnvInvalidProxyFails(t *testing.T) {
	t.Setenv(tgnotify.EnvToken, "tok")
	t.Setenv(tgnotify.EnvChatID, "42")
	t.Setenv(tgnotify.EnvBaseURL, "")
	t.Setenv(tgnotify.EnvProxy, "ftp://bad")
	if _, _, err := tgnotify.FromEnv(); err == nil {
		t.Fatal("invalid proxy scheme must fail")
	}
}
