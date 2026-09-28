// Package storage keeps postik's files on disk and serves them publicly.
package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const PublicPrefix = "/uploads/"

type Files struct {
	Dir string
}

// SaveAvatar stores the picture of a channel and returns its public path.
// Every save gets a new name, so browsers never show a stale picture.
func (f Files) SaveAvatar(channelID uuid.UUID, data []byte, now time.Time) (string, error) {
	dir := filepath.Join(f.Dir, "avatars")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%d%s", channelID, now.UnixNano(), extensionFor(data))
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", err
	}
	return PublicPrefix + "avatars/" + name, nil
}

// Remove deletes a file by its public path. Missing files are not an error.
func (f Files) Remove(publicPath string) error {
	relative, ok := strings.CutPrefix(publicPath, PublicPrefix)
	if !ok || strings.Contains(relative, "..") {
		return fmt.Errorf("not a stored file: %q", publicPath)
	}
	err := os.Remove(filepath.Join(f.Dir, filepath.FromSlash(relative)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Open opens a stored file by its public path.
func (f Files) Open(publicPath string) (*os.File, error) {
	relative, ok := strings.CutPrefix(publicPath, PublicPrefix)
	if !ok || strings.Contains(relative, "..") {
		return nil, fmt.Errorf("not a stored file: %q", publicPath)
	}
	return os.Open(filepath.Join(f.Dir, filepath.FromSlash(relative)))
}

// Handler serves the stored files under PublicPrefix, without directory listings.
func (f Files) Handler() http.Handler {
	files := http.FileServer(http.Dir(f.Dir))
	return http.StripPrefix(strings.TrimSuffix(PublicPrefix, "/"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		files.ServeHTTP(w, r)
	}))
}

func extensionFor(data []byte) string {
	switch http.DetectContentType(data) {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}
