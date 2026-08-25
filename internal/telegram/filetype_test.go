package telegram_test

import (
	"testing"

	"gitlab.com/lyoneel/cli-tg-notify/internal/telegram"
)

func TestDetectType(t *testing.T) {
	tests := []struct {
		path string
		want telegram.FileType
	}{
		{"/tmp/img.png", telegram.TypePhoto},
		{"/tmp/photo.PNG", telegram.TypePhoto},
		{"/tmp/shot.jpeg", telegram.TypePhoto},
		{"/tmp/anim.gif", telegram.TypePhoto},
		{"/tmp/pic.webp", telegram.TypePhoto},
		{"/tmp/scan.bmp", telegram.TypePhoto},
		{"/tmp/song.mp3", telegram.TypeAudio},
		{"/tmp/song.m4a", telegram.TypeAudio},
		{"/tmp/song.aac", telegram.TypeAudio},
		{"/tmp/song.flac", telegram.TypeAudio},
		{"/tmp/note.ogg", telegram.TypeVoice},
		{"/tmp/note.opus", telegram.TypeVoice},
		{"/tmp/clip.mp4", telegram.TypeVideo},
		{"/tmp/clip.webm", telegram.TypeVideo},
		{"/tmp/clip.mkv", telegram.TypeVideo},
		{"/tmp/clip.mov", telegram.TypeVideo},
		{"/tmp/clip.avi", telegram.TypeVideo},
		{"/tmp/clip.flv", telegram.TypeVideo},
		{"/tmp/clip.wmv", telegram.TypeVideo},
		{"/tmp/clip.m4v", telegram.TypeVideo},
		{"/tmp/clip.ogv", telegram.TypeVideo},
		{"/tmp/report.unknown", telegram.TypeDocument},
		{"/tmp/noext", telegram.TypeDocument},
		{"/tmp/archive.pdf", telegram.TypeDocument},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := telegram.DetectType(tt.path); got != tt.want {
				t.Errorf("DetectType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestMaxUploadSize(t *testing.T) {
	const mb = int64(1024 * 1024)
	tests := []struct {
		fileType telegram.FileType
		want     int64
	}{
		{telegram.TypePhoto, 10 * mb},
		{telegram.TypeDocument, 50 * mb},
		{telegram.TypeAudio, 50 * mb},
		{telegram.TypeVideo, 50 * mb},
		{telegram.TypeVoice, 50 * mb},
	}
	for _, tt := range tests {
		t.Run(string(tt.fileType), func(t *testing.T) {
			if got := telegram.MaxUploadSize(tt.fileType); got != tt.want {
				t.Errorf("MaxUploadSize(%q) = %d, want %d", tt.fileType, got, tt.want)
			}
		})
	}
}

func TestMaxUploadSizeFor(t *testing.T) {
	const mb = int64(1024 * 1024)
	tests := []struct {
		name       string
		fileType   telegram.FileType
		selfHosted bool
		want       int64
	}{
		{name: "photo official", fileType: telegram.TypePhoto, selfHosted: false, want: 10 * mb},
		{name: "document official", fileType: telegram.TypeDocument, selfHosted: false, want: 50 * mb},
		{name: "photo self-hosted", fileType: telegram.TypePhoto, selfHosted: true, want: 2000 * mb},
		{name: "voice self-hosted", fileType: telegram.TypeVoice, selfHosted: true, want: 2000 * mb},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telegram.MaxUploadSizeFor(tt.fileType, tt.selfHosted); got != tt.want {
				t.Errorf("MaxUploadSizeFor(%q, %v) = %d, want %d", tt.fileType, tt.selfHosted, got, tt.want)
			}
		})
	}
}
