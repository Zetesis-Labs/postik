package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/zetesis-labs/postik/internal/jobs"
	"github.com/zetesis-labs/postik/internal/testsupport/fakelinkedin"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeresend"
)

func (h *harness) tokenExpiry() {
	h.t.Helper()
	job := &river.Job[jobs.TokenExpiryArgs]{JobRow: &rivertype.JobRow{Attempt: 1}}
	if err := h.jobs.TokenExpiry.Work(context.Background(), job); err != nil {
		h.t.Fatalf("token expiry: %v", err)
	}
}

// storedNotices reads the notifications with template straight from the
// database, for tests whose clock outlives the sessions.
func (h *harness) storedNotices(template string) []map[string]string {
	h.t.Helper()
	rows, err := h.db.QueryContext(context.Background(), "SELECT params FROM notifications WHERE template = ? ORDER BY created_at", template)
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			h.t.Fatal(err)
		}
		params := map[string]string{}
		if err := json.Unmarshal(raw, &params); err != nil {
			h.t.Fatal(err)
		}
		out = append(out, params)
	}
	return out
}

func (h *harness) templates(session requestOption, template string) []notificationView {
	h.t.Helper()
	items, _ := h.notifications(session)
	var out []notificationView
	for _, n := range items {
		if n.Template == template {
			out = append(out, n)
		}
	}
	return out
}

// S06.10 Un post sale en LinkedIn como texto plano y queda Publicado con enlace.
func TestALinkedInPostGoesOutAsPlainText(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	profile := b.connectLinkedIn(anaLinkedIn)
	page := b.connectPage(anaAdmin, zetesis)
	at := tomorrowAt(h.clock.Now(), 10)
	content := "<p>Hola <strong>mundo</strong> (#1)</p><ul><li><p>uno</p></li></ul>"
	ids := []string{
		h.scheduleValues(b.session(), "schedule", at, profile, []value{{Content: content}}),
		h.scheduleValues(b.session(), "schedule", at, page, []value{{Content: content}}),
	}

	h.clock.Set(at)
	for _, id := range ids {
		h.mustPublishValue(id, at, 0)
	}

	posts := h.linkedIn.Posts()
	if len(posts) != 2 {
		t.Fatalf("posts = %+v", posts)
	}
	authors := []string{"urn:li:person:ana-li", "urn:li:organization:" + zetesis.ID}
	for i, p := range posts {
		if p.Author != authors[i] || p.Commentary != `Hola 𝗺𝘂𝗻𝗱𝗼 \(\#1\)`+"\n- uno" || p.Version != "202609" || p.Protocol != "2.0.0" {
			t.Fatalf("post %d = %+v", i, p)
		}
		if s := h.postState(ids[i]); s.Status != "published" || s.ReleaseURL != "https://www.linkedin.com/feed/update/"+p.URN+"/" {
			t.Fatalf("post %d state = %+v", i, s)
		}
	}
}

// S06.11 Los medios se suben desde el disco y el tipo decide el post.
func TestLinkedInMediaIsUploadedFromDisk(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	channel := b.connectLinkedIn(anaLinkedIn)
	bigVideo := append(slices.Clone(mp4Bytes), bytes.Repeat([]byte{5}, 9<<20)...)
	photo := h.mustUpload(b.session(), "foto.png", pngBytes)
	video := h.mustUpload(b.session(), "clip.mp4", bigVideo)
	ref := func(id string) map[string]any { return map[string]any{"id": id} }
	at := tomorrowAt(h.clock.Now(), 10)
	ids := []string{
		h.scheduleValues(b.session(), "schedule", at, channel, []value{{Content: "<p>Una</p>", Media: []map[string]any{ref(photo.ID)}}}),
		h.scheduleValues(b.session(), "schedule", at, channel, []value{{Content: "<p>Tres</p>", Media: []map[string]any{ref(photo.ID), ref(photo.ID), ref(photo.ID)}}}),
		h.scheduleValues(b.session(), "schedule", at, channel, []value{{Content: "<p>Vídeo</p>", Media: []map[string]any{ref(video.ID)}}}),
	}

	h.clock.Set(at)
	for _, id := range ids {
		h.mustPublishValue(id, at, 0)
	}

	posts := h.linkedIn.Posts()
	if len(posts) != 3 || len(posts[0].Media) != 1 || len(posts[1].MultiImage) != 3 || len(posts[1].Media) != 0 || len(posts[2].Media) != 1 {
		t.Fatalf("posts = %+v", posts)
	}
	for _, urn := range append(posts[0].Media, posts[1].MultiImage...) {
		if u, ok := h.linkedIn.Upload(urn); !ok || u.Kind != "image" || u.Owner != "urn:li:person:ana-li" || !bytes.Equal(u.Data, pngBytes) {
			t.Fatalf("image %s = %+v", urn, u.URN)
		}
	}
	u, ok := h.linkedIn.Upload(posts[2].Media[0])
	if !ok || u.Kind != "video" || !u.Finalized || u.Parts != 3 || !bytes.Equal(u.Data, bigVideo) {
		t.Fatalf("video = %s, finalized %v, %d parts, %d bytes", u.URN, u.Finalized, u.Parts, len(u.Data))
	}
}

// S06.12 Los comentarios de LinkedIn responden al post principal.
func TestLinkedInCommentsAnswerThePost(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	page := b.connectPage(anaAdmin, zetesis)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.scheduleValues(b.session(), "schedule", at, page, []value{{Content: "<p>Principal</p>"}, {Content: "<p>Primero</p>"}, {Content: "<p>Segundo</p>"}})

	h.clock.Set(at)
	for index := range 3 {
		h.mustPublishValue(id, at, index)
	}

	posts, comments := h.linkedIn.Posts(), h.linkedIn.Comments()
	if len(posts) != 1 || len(comments) != 2 {
		t.Fatalf("posts %+v, comments %+v", posts, comments)
	}
	for i, text := range []string{"Primero", "Segundo"} {
		c := comments[i]
		if c.Object != posts[0].URN || c.Actor != "urn:li:organization:"+zetesis.ID || c.Text != text {
			t.Fatalf("comment %d = %+v", i, c)
		}
	}
}

// S06.13 Un fallo antes del punto sin retorno se reintenta y no publica dos veces.
func TestAFailureBeforeThePointOfNoReturnIsRetried(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	channel := b.connectLinkedIn(anaLinkedIn)
	photo := h.mustUpload(b.session(), "foto.png", pngBytes)
	at := tomorrowAt(h.clock.Now(), 10)
	withImage := h.scheduleValues(b.session(), "schedule", at, channel, []value{{Content: "<p>Con foto</p>", Media: []map[string]any{{"id": photo.ID}}}})
	dropped := h.scheduleValues(b.session(), "schedule", at, channel, []value{{Content: "<p>Sin respuesta</p>"}})
	h.clock.Set(at)

	h.linkedIn.FailNext("/rest/images", http.StatusServiceUnavailable, "Service Unavailable")
	if err := h.publishValue(withImage, at, 0, 1); err == nil {
		t.Fatal("the first attempt should ask River to retry")
	}
	if s := h.postState(withImage); s.Status != "scheduled" || len(h.deliveries(withImage)) != 0 {
		t.Fatalf("after the failed upload: %+v, deliveries %+v", s, h.deliveries(withImage))
	}
	if err := h.publishValue(withImage, at, 0, 2); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if s := h.postState(withImage); s.Status != "published" || len(h.linkedIn.Posts()) != 1 {
		t.Fatalf("after the retry: %+v, posts %d", s, len(h.linkedIn.Posts()))
	}

	h.linkedIn.DropNext("/rest/posts")
	h.mustPublishValue(dropped, at, 0)
	h.mustPublishValue(dropped, at, 0)
	if s := h.postState(dropped); s.Status != "error" || s.Error != "unconfirmed" {
		t.Fatalf("dropped post = %+v", s)
	}
	if n := len(h.linkedIn.Posts()); n != 1 {
		t.Fatalf("LinkedIn has %d posts, want 1", n)
	}
}

// S06.14 Un token caducado se renueva y el post sale una sola vez.
func TestAnExpiredTokenIsRenewedAndThePostGoesOutOnce(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	profile := b.connectLinkedIn(anaLinkedIn)
	page := b.connectPage(anaAdmin, zetesis)
	at := tomorrowAt(h.clock.Now(), 10)
	ids := []string{
		h.scheduleValues(b.session(), "schedule", at, profile, []value{{Content: "<p>Caducado</p>"}}),
		h.scheduleValues(b.session(), "schedule", at, page, []value{{Content: "<p>Revocado</p>"}}),
	}
	h.clock.Set(at)
	h.exec("UPDATE channel_credentials SET expires_at = ? WHERE channel_id = ?", at.Add(-time.Hour), profile)

	h.mustPublishValue(ids[0], at, 0)
	if n := h.linkedIn.Renewals(); n != 1 {
		t.Fatalf("renewals before the first post = %d", n)
	}
	h.linkedIn.FailNext("/rest/posts", http.StatusUnauthorized, "Invalid access token")
	h.mustPublishValue(ids[1], at, 0)

	if n := h.linkedIn.Renewals(); n != 2 {
		t.Fatalf("renewals = %d, want 2", n)
	}
	if n := len(h.linkedIn.Posts()); n != 2 {
		t.Fatalf("LinkedIn has %d posts, want 2", n)
	}
	for i, id := range ids {
		if s := h.postState(id); s.Status != "published" {
			t.Fatalf("post %d = %+v", i, s)
		}
		channel := []string{profile, page}[i]
		if updated := h.credentialsUpdatedAt(channel); !updated.Equal(at) {
			t.Fatalf("tokens of channel %d saved at %s", i, updated)
		}
	}
}

// S06.15 Si no se puede renovar, el canal pide reconexión y se avisa a todos.
func TestAChannelThatCannotRenewAsksForReconnection(t *testing.T) {
	var resend *fakeresend.Server
	h := newHarness(t, withOIDC("Fake"), withLinkedIn(), withResend(&resend))
	b := h.memberBrowser(ana)
	noRefresh := anaLinkedIn
	noRefresh.NoRefreshTokens = true
	profile := b.connectLinkedIn(noRefresh)
	page := b.connectPage(anaAdmin, zetesis)
	h.setEmailPreferences(ana.Subject, true, false)
	h.linkedIn.RejectRefresh()
	at := tomorrowAt(h.clock.Now(), 10)
	ids := []string{
		h.scheduleValues(b.session(), "schedule", at, profile, []value{{Content: "<p>Uno</p>"}}),
		h.scheduleValues(b.session(), "schedule", at, page, []value{{Content: "<p>Dos</p>"}}),
	}
	h.clock.Set(at)

	for _, id := range ids {
		h.linkedIn.FailNext("/rest/posts", http.StatusUnauthorized, "Invalid access token")
		h.mustPublishValue(id, at, 0)
	}

	for i, channel := range []string{profile, page} {
		if c := h.mustChannelRow(channel); !c.RefreshNeeded {
			t.Fatalf("channel %d does not need reconnection", i)
		}
		if s := h.postState(ids[i]); s.Status != "error" || s.Error != "channel_refresh" {
			t.Fatalf("post %d = %+v", i, s)
		}
	}
	if n := len(h.templates(b.session(), "refresh_failed")); n != 2 {
		t.Fatalf("%d refresh_failed notifications, want 2", n)
	}
	if n := len(h.templates(b.session(), "channel_refresh")); n != 0 {
		t.Fatalf("%d channel_refresh notifications, want none", n)
	}
	if emails := resend.To(ana.Email); len(emails) != 2 || !strings.Contains(emails[0].Subject, "LinkedIn") {
		t.Fatalf("emails to Ana = %+v", emails)
	}
}

// S06.16 El aviso de caducidad sale una vez, y al caducar el canal pide reconexión.
func TestTheExpiryWarningGoesOutOnce(t *testing.T) {
	var resend *fakeresend.Server
	h := newHarness(t, withOIDC("Fake"), withLinkedIn(), withResend(&resend))
	b := h.memberBrowser(ana)
	noRefresh := anaLinkedIn
	noRefresh.NoRefreshTokens = true
	channel := b.connectLinkedIn(noRefresh)

	h.clock.Advance(fakelinkedin.TokenLifetime*time.Second - 6*24*time.Hour)
	h.tokenExpiry()
	h.clock.Advance(time.Hour)
	h.tokenExpiry()

	warnings := h.storedNotices("channel_expiring")
	if len(warnings) != 1 || warnings[0]["days"] != "6" || warnings[0]["channel"] != "Ana García" {
		t.Fatalf("warnings = %+v", warnings)
	}
	if emails := resend.To(ana.Email); len(emails) != 1 {
		t.Fatalf("emails = %+v", emails)
	}
	if c := h.mustChannelRow(channel); c.RefreshNeeded {
		t.Fatal("the channel should still work")
	}

	h.clock.Advance(7 * 24 * time.Hour)
	h.tokenExpiry()
	h.tokenExpiry()
	if c := h.mustChannelRow(channel); !c.RefreshNeeded {
		t.Fatal("an expired channel needs reconnection")
	}
	if n := len(h.storedNotices("refresh_failed")); n != 1 {
		t.Fatalf("%d refresh_failed notifications, want 1", n)
	}
}

// S06.17 El servidor rechaza lo que LinkedIn no admite.
func TestTheServerRejectsWhatLinkedInDoesNotTake(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	channel := b.connectLinkedIn(anaLinkedIn)
	photo := map[string]any{"id": h.mustUpload(b.session(), "foto.png", pngBytes).ID}
	video := map[string]any{"id": h.mustUpload(b.session(), "clip.mp4", mp4Bytes).ID}
	var many []map[string]any
	for range 21 {
		many = append(many, photo)
	}
	at := rfc(tomorrowAt(h.clock.Now(), 10))
	cases := []struct {
		values []value
		code   string
	}{
		{[]value{{Content: "<p>" + strings.Repeat("a", 3001) + "</p>"}}, "too_long"},
		{[]value{{Content: "<p>Vídeo y foto</p>", Media: []map[string]any{video, photo}}}, "video_alone"},
		{[]value{{Content: "<p>Muchas</p>", Media: many}}, "too_many_media"},
		{[]value{{Content: "<p>Principal</p>"}, {Content: "<p>Comentario</p>", Media: []map[string]any{photo}}}, "comment_media"},
	}
	for _, c := range cases {
		res := h.createPosts(b.session(), newPosts{Type: "schedule", PublishAt: at, Posts: []channelPost{{ChannelID: channel, Values: c.values}}})
		expectStatus(t, res, http.StatusBadRequest)
		problems := res.json(t)["problems"].([]any)
		if len(problems) != 1 || problems[0].(map[string]any)["code"] != c.code {
			t.Fatalf("problems = %v, want %s", problems, c.code)
		}
	}
	if n := h.count("posts"); n != 0 {
		t.Fatalf("%d posts saved", n)
	}
}
