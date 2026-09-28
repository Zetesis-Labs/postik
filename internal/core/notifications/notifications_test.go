package notifications

import (
	"strings"
	"testing"
)

func TestTextOfEachTemplate(t *testing.T) {
	cases := []struct {
		notice Notice
		want   string
	}{
		{Notice{Template: "published", Params: map[string]string{"provider": "telegram", "url": "https://t.me/demo/1"}}, "Tu post se ha publicado en Telegram: https://t.me/demo/1"},
		{Notice{Template: "failed", Params: map[string]string{"provider": "telegram", "channel": "Demo", "reason": "Bad Request: chat not found"}}, "Error al publicar en Telegram en Demo: Bad Request: chat not found"},
		{Notice{Template: "failed", Params: map[string]string{"provider": "telegram", "channel": "Demo", "reason": "unreachable"}}, "Error al publicar en Telegram en Demo: no se ha podido conectar con la red"},
		{Notice{Template: "comment_failed", Params: map[string]string{"provider": "telegram", "channel": "Demo", "reason": "x"}}, "Error al publicar los comentarios en Telegram en Demo: x"},
		{Notice{Template: "unconfirmed", Params: map[string]string{"provider": "telegram", "channel": "Demo"}}, "No se ha podido confirmar tu post en Telegram. Revisa Demo"},
		{Notice{Template: "channel_disabled", Params: map[string]string{"channel": "Demo"}}, "No se ha podido publicar en Demo porque está desactivado. Actívalo y vuelve a intentarlo"},
		{Notice{Template: "channel_refresh", Params: map[string]string{"channel": "Demo"}}, "No se ha podido publicar en Demo porque hay que volver a conectarlo"},
		{Notice{Template: "refresh_failed", Params: map[string]string{"provider": "linkedin", "channel": "Ana García"}}, "No se ha podido renovar tu canal Ana García de LinkedIn. Vuelve a conectarlo"},
		{Notice{Template: "channel_expiring", Params: map[string]string{"provider": "linkedin-page", "channel": "Zetesis", "days": "6"}}, "Tu canal Zetesis de LinkedIn Page caduca en 6 días. Vuelve a conectarlo antes para que sus posts no fallen"},
	}
	for _, c := range cases {
		if got := Text(c.notice); got != c.want {
			t.Errorf("Text(%s) = %q, want %q", c.notice.Template, got, c.want)
		}
	}
}

func TestEmailEscapesAndLinksToSettings(t *testing.T) {
	subject, html := Email(Notice{Template: "failed", Params: map[string]string{"provider": "telegram", "channel": "<Demo>", "reason": "a & b"}}, "https://postik.test")
	if subject == "" || strings.Contains(html, "<Demo>") || !strings.Contains(html, "&lt;Demo&gt;") || !strings.Contains(html, "a &amp; b") {
		t.Fatalf("subject %q, html %s", subject, html)
	}
	if !strings.Contains(html, `href="https://postik.test/settings"`) || !strings.Contains(html, `href="https://postik.test/launches"`) {
		t.Fatalf("links missing: %s", html)
	}
}

func TestDigestListsEveryPublication(t *testing.T) {
	items := []Notice{
		{Template: "published", Params: map[string]string{"provider": "telegram", "url": "https://t.me/demo/1"}},
		{Template: "published", Params: map[string]string{"provider": "telegram", "url": "https://t.me/demo/2"}},
	}
	subject, html := Digest(items, "https://postik.test")
	if !strings.Contains(subject, "2") || !strings.Contains(html, "https://t.me/demo/1") || !strings.Contains(html, "https://t.me/demo/2") || !strings.Contains(html, "https://postik.test/settings") {
		t.Fatalf("subject %q, html %s", subject, html)
	}
}
