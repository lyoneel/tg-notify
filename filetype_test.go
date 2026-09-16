package tgnotify_test

import (
	"testing"

	"gitlab.com/lyoneel/tgnotify"
)

func TestDetectType(t *testing.T) {
	tests := []struct {
		path string
		want tgnotify.FileType
	}{
		{"/tmp/img.png", tgnotify.TypePhoto},
		{"/tmp/photo.PNG", tgnotify.TypePhoto},
		{"/tmp/shot.jpeg", tgnotify.TypePhoto},
		{"/tmp/anim.gif", tgnotify.TypePhoto},
		{"/tmp/pic.webp", tgnotify.TypePhoto},
		{"/tmp/scan.bmp", tgnotify.TypePhoto},
		{"/tmp/song.mp3", tgnotify.TypeAudio},
		{"/tmp/song.m4a", tgnotify.TypeAudio},
		{"/tmp/song.aac", tgnotify.TypeAudio},
		{"/tmp/song.flac", tgnotify.TypeAudio},
		{"/tmp/note.ogg", tgnotify.TypeVoice},
		{"/tmp/note.opus", tgnotify.TypeVoice},
		{"/tmp/clip.mp4", tgnotify.TypeVideo},
		{"/tmp/clip.webm", tgnotify.TypeVideo},
		{"/tmp/clip.mkv", tgnotify.TypeVideo},
		{"/tmp/clip.mov", tgnotify.TypeVideo},
		{"/tmp/clip.avi", tgnotify.TypeVideo},
		{"/tmp/clip.flv", tgnotify.TypeVideo},
		{"/tmp/clip.wmv", tgnotify.TypeVideo},
		{"/tmp/clip.m4v", tgnotify.TypeVideo},
		{"/tmp/clip.ogv", tgnotify.TypeVideo},
		{"/tmp/report.unknown", tgnotify.TypeDocument},
		{"/tmp/noext", tgnotify.TypeDocument},
		{"/tmp/archive.pdf", tgnotify.TypeDocument},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := tgnotify.DetectType(tt.path); got != tt.want {
				t.Errorf("DetectType(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestMaxUploadSize(t *testing.T) {
	const mb = int64(1024 * 1024)
	tests := []struct {
		fileType tgnotify.FileType
		want     int64
	}{
		{tgnotify.TypePhoto, 10 * mb},
		{tgnotify.TypeDocument, 50 * mb},
		{tgnotify.TypeAudio, 50 * mb},
		{tgnotify.TypeVideo, 50 * mb},
		{tgnotify.TypeVoice, 50 * mb},
	}
	for _, tt := range tests {
		t.Run(string(tt.fileType), func(t *testing.T) {
			if got := tgnotify.MaxUploadSize(tt.fileType); got != tt.want {
				t.Errorf("MaxUploadSize(%q) = %d, want %d", tt.fileType, got, tt.want)
			}
		})
	}
}

func TestMaxUploadSizeFor(t *testing.T) {
	const mb = int64(1024 * 1024)
	tests := []struct {
		name       string
		fileType   tgnotify.FileType
		selfHosted bool
		want       int64
	}{
		{name: "photo official", fileType: tgnotify.TypePhoto, selfHosted: false, want: 10 * mb},
		{name: "document official", fileType: tgnotify.TypeDocument, selfHosted: false, want: 50 * mb},
		{name: "photo self-hosted", fileType: tgnotify.TypePhoto, selfHosted: true, want: 2000 * mb},
		{name: "voice self-hosted", fileType: tgnotify.TypeVoice, selfHosted: true, want: 2000 * mb},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tgnotify.MaxUploadSizeFor(tt.fileType, tt.selfHosted); got != tt.want {
				t.Errorf("MaxUploadSizeFor(%q, %v) = %d, want %d", tt.fileType, tt.selfHosted, got, tt.want)
			}
		})
	}
}
