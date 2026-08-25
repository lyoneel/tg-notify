package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gitlab.com/lyoneel/cli-tg-notify/internal/telegram"
)

func TestAlbumRefs(t *testing.T) {
	tests := []struct {
		name       string
		flagValues []string
		positional []string
		want       []string
	}{
		{
			name:       "space-separated after --album",
			flagValues: []string{"a.jpg"},
			positional: []string{"b.jpg", "c.jpg"},
			want:       []string{"a.jpg", "b.jpg", "c.jpg"},
		},
		{
			name:       "repeated --album flags",
			flagValues: []string{"a.jpg", "b.jpg"},
			positional: nil,
			want:       []string{"a.jpg", "b.jpg"},
		},
		{
			name:       "mixed flags and positional",
			flagValues: []string{"a.jpg"},
			positional: []string{"b.jpg"},
			want:       []string{"a.jpg", "b.jpg"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := albumRefs(tt.flagValues, tt.positional); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("albumRefs(%v, %v) = %v, want %v", tt.flagValues, tt.positional, got, tt.want)
			}
		})
	}
}

func TestDetectAlbumType(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want telegram.FileType
	}{
		{name: "local jpg", ref: "a.jpg", want: telegram.TypePhoto},
		{name: "local mp4", ref: "b.mp4", want: telegram.TypeVideo},
		{name: "remote jpg", ref: "https://example.com/a.jpg", want: telegram.TypePhoto},
		{name: "remote mp4 with query", ref: "https://example.com/b.mp4?x=1", want: telegram.TypeVideo},
		{name: "unknown extension falls back to photo", ref: "a.pdf", want: telegram.TypePhoto},
		{name: "remote no extension falls back to photo", ref: "https://example.com/x", want: telegram.TypePhoto},
		{name: "uppercase extension", ref: "a.JPG", want: telegram.TypePhoto},
		{name: "mkv video", ref: "clip.mkv", want: telegram.TypeVideo},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectAlbumType(tt.ref); got != tt.want {
				t.Errorf("detectAlbumType(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestReadMessageFromStdin(t *testing.T) {
	orig := stdin
	defer func() { stdin = orig }()

	stdin = bytes.NewBufferString("  hello from stdin  \n")
	got, err := readMessageFromStdin(false)
	if err != nil {
		t.Fatalf("readMessageFromStdin: %v", err)
	}
	if got != "hello from stdin" {
		t.Errorf("got %q, want %q", got, "hello from stdin")
	}
}

func TestReadMessageFromStdinEmpty(t *testing.T) {
	orig := stdin
	defer func() { stdin = orig }()

	stdin = bytes.NewBufferString("   \n")
	got, err := readMessageFromStdin(false)
	if err != nil {
		t.Fatalf("readMessageFromStdin: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestReadMessageFromStdinNoTrim(t *testing.T) {
	orig := stdin
	defer func() { stdin = orig }()

	input := "  keep spaces  \n"
	stdin = bytes.NewBufferString(input)
	got, err := readMessageFromStdin(true)
	if err != nil {
		t.Fatalf("readMessageFromStdin: %v", err)
	}
	if got != input {
		t.Errorf("noTrim got %q, want %q", got, input)
	}
}

func TestValidateAlbumFiles(t *testing.T) {
	tempDir := t.TempDir()
	good := filepath.Join(tempDir, "a.jpg")
	if err := os.WriteFile(good, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	t.Run("all present and remote passes", func(t *testing.T) {
		items := []telegram.MediaItem{
			{Type: telegram.TypePhoto, Ref: good},
			{Type: telegram.TypePhoto, Ref: "https://example.com/b.jpg"},
		}
		if err := validateAlbumFiles(items); err != nil {
			t.Errorf("validateAlbumFiles = %v, want nil", err)
		}
	})

	t.Run("missing local file errors", func(t *testing.T) {
		items := []telegram.MediaItem{
			{Type: telegram.TypePhoto, Ref: filepath.Join(tempDir, "missing.jpg")},
			{Type: telegram.TypePhoto, Ref: "https://example.com/b.jpg"},
		}
		if err := validateAlbumFiles(items); err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})

	t.Run("directory errors", func(t *testing.T) {
		items := []telegram.MediaItem{
			{Type: telegram.TypePhoto, Ref: tempDir},
			{Type: telegram.TypePhoto, Ref: "https://example.com/b.jpg"},
		}
		if err := validateAlbumFiles(items); err == nil {
			t.Error("expected error for directory, got nil")
		}
	})
}

func TestIsRemoteURL(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{"http://example.com/a.jpg", true},
		{"https://example.com/a.jpg", true},
		{"a.jpg", false},
		{"/tmp/a.jpg", false},
	}
	for _, tt := range tests {
		if got := isRemoteURL(tt.ref); got != tt.want {
			t.Errorf("isRemoteURL(%q) = %v, want %v", tt.ref, got, tt.want)
		}
	}
}

func TestRunMessageStdinTTYReturnsUsage(t *testing.T) {
	origTTY := stdinIsTTY
	origStdin := stdin
	defer func() {
		stdinIsTTY = origTTY
		stdin = origStdin
	}()

	stdinIsTTY = func() bool { return true }
	stdin = bytes.NewBufferString("ignored")

	err := runMessage(context.Background(), options{}, nil)
	if err != errUsage {
		t.Errorf("runMessage with TTY stdin = %v, want errUsage", err)
	}
}
