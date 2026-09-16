package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/lyoneel/tgnotify"
)

func TestCompletionFlagsContainsNewFlags(t *testing.T) {
	joined := strings.Join(completionFlags, " ")
	for _, want := range []string{"--silent", "--proxy", "--base-url", "--dry-run", "--completion"} {
		if !strings.Contains(joined, want) {
			t.Errorf("completionFlags missing %q", want)
		}
	}
}

func TestRunCompletion(t *testing.T) {
	tests := []struct {
		shell   string
		wantSub string
		wantErr bool
	}{
		{shell: "bash", wantSub: "complete -F _tg_notify"},
		{shell: "zsh", wantSub: "#compdef tg-notify"},
		{shell: "fish", wantSub: "complete -c tg-notify"},
		{shell: "powershell", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			// capture stdout by swapping a pipe is brittle; instead test
			// the underlying vars and the error path via runCompletion.
			if tt.wantErr {
				err := runCompletion(tt.shell)
				if err == nil {
					t.Fatalf("runCompletion(%q) = nil, want error", tt.shell)
				}
				return
			}
			completion := map[string]string{
				"bash": bashCompletion,
				"zsh":  zshCompletion,
				"fish": fishCompletion,
			}[tt.shell]
			if !strings.Contains(completion, tt.wantSub) {
				t.Errorf("%s completion missing %q", tt.shell, tt.wantSub)
			}
		})
	}
}

func TestLoadDotEnv(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(path, []byte("# comment\nTELEGRAM_CHAT_ID=123456\nTELEGRAM_BOT_TOKEN=abcdef\nEMPTY=\nNO_VALUE\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	// Ensure these are unset first.
	for _, k := range []string{"TELEGRAM_CHAT_ID", "TELEGRAM_BOT_TOKEN"} {
		if _, ok := os.LookupEnv(k); ok {
			t.Setenv(k, "") // cannot unset directly; set empty so lookup still sees it
		}
		_ = os.Unsetenv(k)
	}

	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("TELEGRAM_CHAT_ID"); got != "123456" {
		t.Errorf("TELEGRAM_CHAT_ID = %q, want 123456", got)
	}
	if got := os.Getenv("TELEGRAM_BOT_TOKEN"); got != "abcdef" {
		t.Errorf("TELEGRAM_BOT_TOKEN = %q, want abcdef", got)
	}
}

func TestLoadDotEnvMissingFile(t *testing.T) {
	if err := loadDotEnv(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Fatalf("loadDotEnv on missing file = %v, want nil", err)
	}
}

func TestLoadDotEnvDoesNotOverrideExisting(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(path, []byte("TELEGRAM_CHAT_ID=fromfile\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	t.Setenv("TELEGRAM_CHAT_ID", "fromenv")
	if err := loadDotEnv(path); err != nil {
		t.Fatalf("loadDotEnv: %v", err)
	}
	if got := os.Getenv("TELEGRAM_CHAT_ID"); got != "fromenv" {
		t.Errorf("TELEGRAM_CHAT_ID = %q, want fromenv (existing env must win)", got)
	}
}

func TestMaxUploadSizeForUsedByLocalFile(t *testing.T) {
	// Guard: the self-hosted path raises the limit. The function itself
	// is covered by telegram tests; here just confirm the wiring value.
	if got := tgnotify.MaxUploadSizeFor(tgnotify.TypePhoto, true); got != 2000*1024*1024 {
		t.Errorf("self-hosted photo limit = %d, want 2000 MB", got)
	}
}
