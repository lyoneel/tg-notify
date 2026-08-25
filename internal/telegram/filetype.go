package telegram

import (
	"mime"
	"path/filepath"
	"strings"
)

// Upload size limits enforced client-side before an upload starts, in
// bytes. Telegram rejects photos above 10 MB and other uploads above
// 50 MB on the official API; a self-hosted Bot API server allows up to
// 2000 MB per file.
const (
	maxPhotoUpload  = int64(10 * 1024 * 1024)
	maxOtherUploads = int64(50 * 1024 * 1024)
	maxSelfHosted   = int64(2000 * 1024 * 1024)
)

// mimeToType maps known MIME types to Telegram media types. The table
// mirrors the tg-notify skill's send_file.py.
var mimeToType = map[string]FileType{
	"image/jpeg":       TypePhoto,
	"image/png":        TypePhoto,
	"image/gif":        TypePhoto,
	"image/webp":       TypePhoto,
	"audio/mpeg":       TypeAudio,
	"audio/mp3":        TypeAudio,
	"audio/mp4":        TypeAudio,
	"audio/x-m4a":      TypeAudio,
	"audio/m4a":        TypeAudio,
	"audio/ogg":        TypeVoice,
	"audio/opus":       TypeVoice,
	"video/mp4":        TypeVideo,
	"video/webm":       TypeVideo,
	"video/x-matroska": TypeVideo,
	"video/quicktime":  TypeVideo,
	"video/avi":        TypeVideo,
	"video/x-msvideo":  TypeVideo,
	"video/x-flv":      TypeVideo,
	"video/mpeg":       TypeVideo,
	"video/x-ms-wmv":   TypeVideo,
}

// extToType maps lower-case file extensions (with leading dot) to
// Telegram media types. The table mirrors the tg-notify skill's
// send_file.py.
var extToType = map[string]FileType{
	".jpg":  TypePhoto,
	".jpeg": TypePhoto,
	".png":  TypePhoto,
	".gif":  TypePhoto,
	".webp": TypePhoto,
	".bmp":  TypePhoto,
	".mp3":  TypeAudio,
	".m4a":  TypeAudio,
	".aac":  TypeAudio,
	".flac": TypeAudio,
	".ogg":  TypeVoice,
	".opus": TypeVoice,
	".mp4":  TypeVideo,
	".webm": TypeVideo,
	".mkv":  TypeVideo,
	".mov":  TypeVideo,
	".avi":  TypeVideo,
	".flv":  TypeVideo,
	".wmv":  TypeVideo,
	".m4v":  TypeVideo,
	".ogv":  TypeVideo,
}

// DetectType guesses the Telegram media type for a local file path.
// The extension table is consulted first (deterministic across
// platforms), then the MIME type derived from the extension, with
// document as the fallback.
func DetectType(path string) FileType {
	ext := strings.ToLower(filepath.Ext(path))
	if t, ok := extToType[ext]; ok {
		return t
	}
	if mimeType := mime.TypeByExtension(ext); mimeType != "" {
		if t, ok := mimeToType[mimeType]; ok {
			return t
		}
	}
	return TypeDocument
}

// MaxUploadSize returns the client-side upload limit in bytes for the
// file type against the official API.
func MaxUploadSize(t FileType) int64 {
	return MaxUploadSizeFor(t, false)
}

// MaxUploadSizeFor returns the client-side upload limit in bytes for
// the file type. When selfHosted is true (a self-hosted Bot API server
// is targeted), every type is allowed up to 2000 MB; otherwise photos
// are limited to 10 MB and everything else to 50 MB.
func MaxUploadSizeFor(t FileType, selfHosted bool) int64 {
	if selfHosted {
		return maxSelfHosted
	}
	if t == TypePhoto {
		return maxPhotoUpload
	}
	return maxOtherUploads
}
