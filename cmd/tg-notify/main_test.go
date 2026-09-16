package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestReorderArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "flags before positional unchanged",
			in:   []string{"-f", "a.png", "-c", "cap"},
			want: []string{"-f", "a.png", "-c", "cap"},
		},
		{
			name: "flags after positional moved ahead",
			in:   []string{"stray", "-f", "a.png"},
			want: []string{"-f", "a.png", "stray"},
		},
		{
			name: "bool flag after positional keeps no value",
			in:   []string{"text", "--no-retry"},
			want: []string{"--no-retry", "text"},
		},
		{
			name: "equals form is single token",
			in:   []string{"text", "--file=a.png"},
			want: []string{"--file=a.png", "text"},
		},
		{
			name: "long and short mixed",
			in:   []string{"hello", "--file", "a.png", "-c", "cap"},
			want: []string{"--file", "a.png", "-c", "cap", "hello"},
		},
		{
			name: "retries value flag after positional moved ahead",
			in:   []string{"text", "--retries", "3"},
			want: []string{"--retries", "3", "text"},
		},
		{
			name: "base-wait equals form is single token",
			in:   []string{"text", "--base-wait=5s"},
			want: []string{"--base-wait=5s", "text"},
		},
		{
			name: "short retry flags after positional moved ahead",
			in:   []string{"text", "-R", "10", "-B", "1s"},
			want: []string{"-R", "10", "-B", "1s", "text"},
		},
		{
			name: "json bool after positional stays bool",
			in:   []string{"text", "--json", "-C", "123"},
			want: []string{"--json", "-C", "123", "text"},
		},
		{
			name: "short json and whoami after positional stay bool",
			in:   []string{"text", "-j", "-w"},
			want: []string{"-j", "-w", "text"},
		},
		{
			name: "reply-to value flag after positional moved ahead",
			in:   []string{"text", "-r", "42"},
			want: []string{"-r", "42", "text"},
		},
		{
			name: "silent short bool after positional stays bool",
			in:   []string{"text", "-S"},
			want: []string{"-S", "text"},
		},
		{
			name: "proxy value flag after positional moved ahead",
			in:   []string{"text", "-P", "socks5://127.0.0.1:1080"},
			want: []string{"-P", "socks5://127.0.0.1:1080", "text"},
		},
		{
			name: "base-url value flag after positional moved ahead",
			in:   []string{"text", "-U", "http://localhost:8081"},
			want: []string{"-U", "http://localhost:8081", "text"},
		},
		{
			name: "dry-run short bool after positional stays bool",
			in:   []string{"text", "-D"},
			want: []string{"-D", "text"},
		},
		{
			name: "completion value flag after positional moved ahead",
			in:   []string{"text", "-A", "bash"},
			want: []string{"-A", "bash", "text"},
		},
		{
			name: "all new short flags together",
			in:   []string{"-S", "text", "-P", "http://p:1", "-U", "http://b", "-D", "-A", "fish"},
			want: []string{"-S", "-P", "http://p:1", "-U", "http://b", "-D", "-A", "fish", "text"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reorderArgs(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reorderArgs(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestScrubSecrets(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "token in url",
			in:   `Post "https://api.tgnotify.org/bot12345:ABC-xyz/sendMessage": timeout`,
			want: `Post "https://api.tgnotify.org/bot<token>/sendMessage": timeout`,
		},
		{
			name: "no token untouched",
			in:   "caption too long (1025 chars, max 1024)",
			want: "caption too long (1025 chars, max 1024)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scrubSecrets(tt.in); got != tt.want {
				t.Errorf("scrubSecrets(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		if err := run(args); !errors.Is(err, errHelp) {
			t.Errorf("run(%v) = %v, want errHelp", args, err)
		}
	}
}
