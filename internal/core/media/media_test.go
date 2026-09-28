package media

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		name   string
		header string
		kind   Kind
		mime   string
	}{
		{"jpeg", "\xFF\xD8\xFF\xE0", Image, "image/jpeg"},
		{"png", "\x89PNG\r\n\x1a\n", Image, "image/png"},
		{"gif87", "GIF87a", Image, "image/gif"},
		{"gif89", "GIF89a", Image, "image/gif"},
		{"webp", "RIFF\x00\x00\x00\x00WEBPVP8 ", Image, "image/webp"},
		{"bmp", "BM\x00\x00", Image, "image/bmp"},
		{"tiff little endian", "II*\x00", Image, "image/tiff"},
		{"tiff big endian", "MM\x00*", Image, "image/tiff"},
		{"avif", "\x00\x00\x00\x1cftypavif", Image, "image/avif"},
		{"avif sequence", "\x00\x00\x00\x1cftypavis", Image, "image/avif"},
		{"mp4", "\x00\x00\x00\x18ftypisom", Video, "video/mp4"},
		{"mp4 other brand", "\x00\x00\x00\x20ftypmp42", Video, "video/mp4"},
	}
	for _, tc := range cases {
		kind, mime, ok := Detect([]byte(tc.header))
		if !ok || kind != tc.kind || mime != tc.mime {
			t.Errorf("%s: %v %q %v", tc.name, kind, mime, ok)
		}
	}
	for _, rejected := range []string{"hola", "", "RIFF\x00\x00\x00\x00WAVE", "%PDF-1.7"} {
		if _, _, ok := Detect([]byte(rejected)); ok {
			t.Errorf("%q was accepted", rejected)
		}
	}
}

func TestLimits(t *testing.T) {
	if LimitFor(Image) != 10<<20 || LimitFor(Video) != 1<<30 {
		t.Fatalf("limits = %d %d", LimitFor(Image), LimitFor(Video))
	}
}

func TestPages(t *testing.T) {
	if Pages(0) != 1 || Pages(18) != 1 || Pages(19) != 2 {
		t.Fatalf("pages = %d %d %d", Pages(0), Pages(18), Pages(19))
	}
}
