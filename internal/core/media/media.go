// Package media holds the pure rules about the files of the library.
package media

import "bytes"

type Kind string

const (
	Image Kind = "image"
	Video Kind = "video"
)

// PageSize is how many files the library shows per page (F15).
const PageSize = 18

// HeaderSize is how many bytes Detect needs to recognise every type.
const HeaderSize = 16

// Detect recognises the file by its first bytes, never by name or declared type.
func Detect(header []byte) (Kind, string, bool) {
	switch {
	case bytes.HasPrefix(header, []byte{0xFF, 0xD8, 0xFF}):
		return Image, "image/jpeg", true
	case bytes.HasPrefix(header, []byte("\x89PNG\r\n\x1a\n")):
		return Image, "image/png", true
	case bytes.HasPrefix(header, []byte("GIF87a")), bytes.HasPrefix(header, []byte("GIF89a")):
		return Image, "image/gif", true
	case len(header) >= 12 && bytes.Equal(header[0:4], []byte("RIFF")) && bytes.Equal(header[8:12], []byte("WEBP")):
		return Image, "image/webp", true
	case bytes.HasPrefix(header, []byte("BM")):
		return Image, "image/bmp", true
	case bytes.HasPrefix(header, []byte("II*\x00")), bytes.HasPrefix(header, []byte("MM\x00*")):
		return Image, "image/tiff", true
	case len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp")):
		brand := string(header[8:12])
		if brand == "avif" || brand == "avis" {
			return Image, "image/avif", true
		}
		return Video, "video/mp4", true
	}
	return "", "", false
}

// LimitFor is the largest file of that kind: 10 MB per image, 1 GB per video.
func LimitFor(kind Kind) int64 {
	if kind == Video {
		return 1 << 30
	}
	return 10 << 20
}

func Pages(total int) int {
	if total <= 0 {
		return 1
	}
	return (total + PageSize - 1) / PageSize
}

// Extension is the file extension stored for a MIME type.
func Extension(mime string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/tiff":
		return ".tiff"
	case "image/avif":
		return ".avif"
	case "video/mp4":
		return ".mp4"
	}
	return ""
}
