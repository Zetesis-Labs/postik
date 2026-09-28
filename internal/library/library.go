// Package library stores the files of an organization's media library.
package library

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	core "github.com/zetesis-labs/postik/internal/core/media"
	"github.com/zetesis-labs/postik/internal/postgres"
)

var (
	ErrUnsupported = errors.New("unsupported file type")
	ErrTooLarge    = errors.New("file too large")
	ErrUnreadable  = errors.New("the file could not be read")
)

type Library struct {
	Store *postgres.MediaStore
	Dir   string
	Now   func() time.Time
}

// Upload detects the type from the first bytes, streams the file to disk
// while counting it, and records it. Nothing stays on disk if it fails.
func (l *Library) Upload(ctx context.Context, orgID uuid.UUID, filename string, content io.Reader) (*postgres.Media, error) {
	header := make([]byte, core.HeaderSize)
	n, err := io.ReadFull(content, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, ErrUnreadable
	}
	header = header[:n]
	kind, mime, ok := core.Detect(header)
	if !ok {
		return nil, ErrUnsupported
	}
	limit := core.LimitFor(kind)

	now := l.Now()
	id := uuid.New()
	relative := filepath.Join("media", now.Format("2006"), now.Format("01"), id.String()+core.Extension(mime))
	destination := filepath.Join(l.Dir, relative)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(filepath.Dir(destination), ".upload-*")
	if err != nil {
		return nil, err
	}
	discard := func() { _ = temp.Close(); _ = os.Remove(temp.Name()) }

	written, err := io.Copy(temp, io.LimitReader(io.MultiReader(bytes.NewReader(header), content), limit+1))
	if err != nil {
		discard()
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return nil, ErrTooLarge
		}
		return nil, ErrUnreadable
	}
	if written > limit {
		discard()
		return nil, ErrTooLarge
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(temp.Name())
		return nil, err
	}
	if err := os.Rename(temp.Name(), destination); err != nil {
		_ = os.Remove(temp.Name())
		return nil, err
	}

	media := &postgres.Media{
		ID:             id,
		OrganizationID: orgID,
		Name:           filepath.Base(filename),
		Path:           "/uploads/" + filepath.ToSlash(relative),
		Kind:           string(kind),
		Mime:           mime,
		Size:           written,
		CreatedAt:      now,
	}
	if err := l.Store.Create(ctx, media); err != nil {
		_ = os.Remove(destination)
		return nil, err
	}
	return media, nil
}
