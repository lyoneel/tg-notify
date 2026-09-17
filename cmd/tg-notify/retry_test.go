package main

import (
	"testing"

	"gitlab.com/lyoneel/tg-notify"
)

func TestRunFlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "negative retries rejected",
			args:    []string{"--retries", "-1", "hello"},
			wantErr: "--retries must be >= 0",
		},
		{
			name:    "zero base-wait rejected",
			args:    []string{"--base-wait", "0s", "hello"},
			wantErr: "--base-wait must be > 0",
		},
		{
			name:    "negative base-wait rejected",
			args:    []string{"--base-wait", "-1s", "hello"},
			wantErr: "--base-wait must be > 0",
		},
		{
			name:    "negative reply-to rejected",
			args:    []string{"--reply-to", "-1", "hello"},
			wantErr: "--reply-to must be >= 0",
		},
		{
			name:    "negative offset rejected",
			args:    []string{"--discover-chat-id", "--offset", "-1"},
			wantErr: "--offset must be >= 0",
		},
		{
			name:    "offset without discover rejected",
			args:    []string{"--offset", "5", "hello"},
			wantErr: "--offset is only valid with --discover-chat-id",
		},
		{
			name:    "whoami with message rejected",
			args:    []string{"--whoami", "hello"},
			wantErr: "--whoami cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "whoami with reply-to rejected",
			args:    []string{"--whoami", "--reply-to", "1"},
			wantErr: "--whoami cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "discover with message rejected",
			args:    []string{"--discover-chat-id", "hello"},
			wantErr: "--discover-chat-id cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "discover with file rejected",
			args:    []string{"--discover-chat-id", "-f", "a.jpg"},
			wantErr: "--discover-chat-id cannot be combined with message, file, album, caption, or reply-to flags",
		},
		{
			name:    "no-trim on positional input rejected",
			args:    []string{"--no-trim", "hello"},
			wantErr: "--no-trim only applies to a message read from stdin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args)
			if err == nil {
				t.Fatalf("run(%v) = nil, want error %q", tt.args, tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Errorf("run(%v) = %q, want %q", tt.args, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestConfigureBotInstallsFlagRetryPolicy(t *testing.T) {
	opts := options{noRetry: true, retries: 3, baseWait: 7}
	bot := tgnotify.New("t")
	if err := configureBot(bot, opts); err != nil {
		t.Fatalf("configureBot: %v", err)
	}
	p := bot.RetryPolicy()
	if !p.Disabled || p.MaxRetries != 3 || p.BaseWait != 7 {
		t.Fatalf("retry policy = %+v, want disabled, 3 retries, 7s base wait", p)
	}
}
