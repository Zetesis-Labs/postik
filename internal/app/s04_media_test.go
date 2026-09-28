package app_test

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var (
	pngBytes  = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"), bytes.Repeat([]byte{7}, 64)...)
	jpegBytes = append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}, bytes.Repeat([]byte{1}, 64)...)
	mp4Bytes  = append([]byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 2, 0}, bytes.Repeat([]byte{2}, 64)...)
)

type mediaView struct {
	ID               string
	Name             string
	URL              string
	Kind             string
	Mime             string
	Alt              string
	ThumbnailURL     string
	ThumbnailSeconds int
}

func toMediaView(raw map[string]any) mediaView {
	m := mediaView{
		ID:   raw["id"].(string),
		Name: raw["name"].(string),
		URL:  raw["url"].(string),
		Kind: raw["kind"].(string),
		Mime: raw["mime"].(string),
	}
	m.Alt, _ = raw["alt"].(string)
	m.ThumbnailURL, _ = raw["thumbnailUrl"].(string)
	if seconds, ok := raw["thumbnailSeconds"].(float64); ok {
		m.ThumbnailSeconds = int(seconds)
	}
	return m
}

// upload sends a file the way a browser does; without length it streams the
// body with chunked encoding, so the server cannot trust any declared size.
func (h *harness) upload(session requestOption, name string, data []byte, withLength bool) response {
	h.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		h.t.Fatalf("multipart: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		h.t.Fatalf("multipart: %v", err)
	}
	if err := writer.Close(); err != nil {
		h.t.Fatalf("multipart: %v", err)
	}
	var reader io.Reader = &body
	if !withLength {
		reader = io.MultiReader(&body)
	}
	req, err := http.NewRequest(http.MethodPost, h.server.URL+"/api/v1/media", reader)
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	session(req)
	res, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("upload %s: %v", name, err)
	}
	defer res.Body.Close()
	data, _ = io.ReadAll(res.Body)
	return response{status: res.StatusCode, header: res.Header, body: data}
}

func (h *harness) mustUpload(session requestOption, name string, data []byte) mediaView {
	h.t.Helper()
	res := h.upload(session, name, data, true)
	expectStatus(h.t, res, http.StatusCreated)
	return toMediaView(res.json(h.t))
}

func (h *harness) library(session requestOption, query string) ([]mediaView, int) {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/media"+query, nil, session)
	expectStatus(h.t, res, http.StatusOK)
	body := res.json(h.t)
	var items []mediaView
	for _, raw := range body["items"].([]any) {
		items = append(items, toMediaView(raw.(map[string]any)))
	}
	return items, int(body["pages"].(float64))
}

func (h *harness) storedMediaFiles() []string {
	h.t.Helper()
	var files []string
	root := filepath.Join(h.config.StorageDir, "media")
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

// S04.1 Subir una imagen la deja en la biblioteca con una URL pública.
func TestUploadingAnImage(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	session := h.member(ana)

	media := h.mustUpload(session, "foto.png", pngBytes)
	if media.Name != "foto.png" || media.Kind != "image" || media.Mime != "image/png" {
		t.Fatalf("media = %+v", media)
	}
	if !strings.HasPrefix(media.URL, "/uploads/media/") {
		t.Fatalf("url = %q", media.URL)
	}
	served := h.do(http.MethodGet, media.URL, nil)
	expectStatus(t, served, http.StatusOK)
	if !bytes.Equal(served.body, pngBytes) {
		t.Fatal("the served file differs from the upload")
	}
	items, _ := h.library(session, "")
	if len(items) != 1 || items[0].ID != media.ID {
		t.Fatalf("library = %+v", items)
	}
}

// S04.2 El tipo sale del contenido, no del nombre.
func TestMediaTypeComesFromTheContent(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	session := h.member(ana)
	accepted := []struct {
		name string
		data []byte
		kind string
		mime string
	}{
		{"a.jpg", jpegBytes, "image", "image/jpeg"},
		{"b.png", pngBytes, "image", "image/png"},
		{"c.gif", []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), "image", "image/gif"},
		{"d.webp", []byte("RIFF\x24\x00\x00\x00WEBPVP8 \x18\x00\x00\x00"), "image", "image/webp"},
		{"e.bmp", []byte("BM\x46\x00\x00\x00\x00\x00\x00\x00\x36\x00"), "image", "image/bmp"},
		{"f.tiff", []byte("II*\x00\x08\x00\x00\x00\x00\x00"), "image", "image/tiff"},
		{"g.avif", []byte("\x00\x00\x00\x1cftypavif\x00\x00\x00\x00avifmif1"), "image", "image/avif"},
		{"h.mp4", mp4Bytes, "video", "video/mp4"},
		{"video.mp4", pngBytes, "image", "image/png"},
	}
	for _, tc := range accepted {
		media := h.mustUpload(session, tc.name, tc.data)
		if media.Kind != tc.kind || media.Mime != tc.mime {
			t.Errorf("%s: kind %q mime %q, want %q %q", tc.name, media.Kind, media.Mime, tc.kind, tc.mime)
		}
	}
	res := h.upload(session, "foto.jpg", []byte("hola, esto es texto"), true)
	expectStatus(t, res, http.StatusUnsupportedMediaType)
	if res.json(t)["code"] != "unsupported_type" {
		t.Errorf("body = %s", res.body)
	}
}

// S04.3 El límite de tamaño se cumple aunque el cliente mienta.
func TestSizeLimitHoldsWithoutADeclaredLength(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	session := h.member(ana)
	big := append(append([]byte(nil), pngBytes...), bytes.Repeat([]byte{9}, 10<<20)...)
	big = big[:10<<20+1]

	res := h.upload(session, "enorme.png", big, false)
	expectStatus(t, res, http.StatusRequestEntityTooLarge)
	if res.json(t)["code"] != "too_large" {
		t.Errorf("body = %s", res.body)
	}
	if files := h.storedMediaFiles(); len(files) != 0 {
		t.Fatalf("files left on disk: %v", files)
	}
	if items, _ := h.library(session, ""); len(items) != 0 {
		t.Fatalf("library = %+v", items)
	}
}

// S04.4 La biblioteca pagina de 18 en 18 y busca por nombre original.
func TestLibraryPagesAndSearches(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	session := h.member(ana)
	for i := 1; i <= 18; i++ {
		h.mustUpload(session, fmt.Sprintf("foto-%02d.png", i), pngBytes)
		h.clock.Advance(time.Second)
	}
	h.mustUpload(session, "playa-1.jpg", jpegBytes)
	h.clock.Advance(time.Second)
	latest := h.mustUpload(session, "playa-2.jpg", jpegBytes)

	first, pages := h.library(session, "?page=1")
	if len(first) != 18 || pages != 2 || first[0].ID != latest.ID {
		t.Fatalf("page 1: %d items, %d pages, first %q", len(first), pages, first[0].Name)
	}
	if second, _ := h.library(session, "?page=2"); len(second) != 2 || second[1].Name != "foto-01.png" {
		t.Fatalf("page 2 = %+v", second)
	}
	found, _ := h.library(session, "?search=playa")
	if len(found) != 2 || found[0].Name != "playa-2.jpg" || found[1].Name != "playa-1.jpg" {
		t.Fatalf("search = %+v", found)
	}
}

// S04.5 Borrar un medio lo saca de la biblioteca pero su URL sigue sirviendo.
func TestDeletingMediaKeepsItsFile(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	session := h.member(ana)
	media := h.mustUpload(session, "foto.png", pngBytes)

	expectStatus(t, h.do(http.MethodDelete, "/api/v1/media/"+media.ID, nil, session), http.StatusNoContent)
	if items, _ := h.library(session, ""); len(items) != 0 {
		t.Fatalf("library = %+v", items)
	}
	expectStatus(t, h.do(http.MethodGet, media.URL, nil), http.StatusOK)
}

// S04.6 Texto alternativo y miniatura de un vídeo.
func TestAltTextAndVideoThumbnail(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	session := h.member(ana)
	video := h.mustUpload(session, "demo.mp4", mp4Bytes)
	frame := h.mustUpload(session, "frame.png", pngBytes)

	res := h.do(http.MethodPut, "/api/v1/media/"+video.ID, map[string]any{
		"alt":              "Demo",
		"thumbnailMediaId": frame.ID,
		"thumbnailSeconds": 3,
	}, session)
	expectStatus(t, res, http.StatusOK)

	items, _ := h.library(session, "?search=demo")
	if len(items) != 1 {
		t.Fatalf("library = %+v", items)
	}
	got := items[0]
	if got.Alt != "Demo" || got.ThumbnailURL != frame.URL || got.ThumbnailSeconds != 3 {
		t.Fatalf("video = %+v", got)
	}
}

// S04.7 Los medios de otra organización no se ven ni se tocan.
func TestMediaOfAnotherOrganizationIsOutOfReach(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	anaSession, brunoSession := h.member(ana), h.member(bruno)
	theirs := h.mustUpload(brunoSession, "bruno.png", pngBytes)

	if items, _ := h.library(anaSession, ""); len(items) != 0 {
		t.Fatalf("Ana sees %+v", items)
	}
	expectStatus(t, h.do(http.MethodPut, "/api/v1/media/"+theirs.ID, map[string]any{"alt": "robado"}, anaSession), http.StatusNotFound)
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/media/"+theirs.ID, nil, anaSession), http.StatusNotFound)
	if items, _ := h.library(brunoSession, ""); len(items) != 1 || items[0].Alt != "" {
		t.Fatalf("Bruno's media changed: %+v", items)
	}
}
