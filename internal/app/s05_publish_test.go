package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/zetesis-labs/postik/internal/jobs"
	"github.com/zetesis-labs/postik/internal/testsupport/faketelegram"
)

var (
	canalPublico = faketelegram.Chat{ID: -1009876543210, Type: "channel", Title: "Demo", Username: "postik_demo", BotAdmin: true}
	canalPrivado = faketelegram.Chat{ID: -1001234567890, Type: "channel", Title: "Privado", BotAdmin: true}
)

// publishValue runs the publish_value job of a post as River would.
func (h *harness) publishValue(postID string, publishAt time.Time, index, attempt int) error {
	h.t.Helper()
	job := &river.Job[jobs.PublishValueArgs]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: jobs.PublishAttempts},
		Args:   jobs.PublishValueArgs{PostID: uuid.MustParse(postID), PublishAt: publishAt.UTC(), Index: index},
	}
	return h.jobs.PublishValue.Work(context.Background(), job)
}

func (h *harness) mustPublishValue(postID string, publishAt time.Time, index int) {
	h.t.Helper()
	if err := h.publishValue(postID, publishAt, index, 1); err != nil {
		h.t.Fatalf("publish %s value %d: %v", postID, index, err)
	}
}

func (h *harness) sweep() {
	h.t.Helper()
	job := &river.Job[jobs.SweepScheduledArgs]{JobRow: &rivertype.JobRow{Attempt: 1}}
	if err := h.jobs.SweepScheduled.Work(context.Background(), job); err != nil {
		h.t.Fatalf("sweep: %v", err)
	}
}

type queuedPublish struct {
	PostID      string
	PublishAt   time.Time
	Index       int
	ScheduledAt time.Time
}

// queued lists the publish_value jobs that are still to run, in insertion order.
func (h *harness) queued() []queuedPublish {
	h.t.Helper()
	rows, err := h.db.QueryContext(context.Background(), `
		SELECT args, scheduled_at FROM river.river_job
		WHERE kind = 'publish_value' AND state IN ('available', 'scheduled', 'retryable', 'pending')
		ORDER BY id`)
	if err != nil {
		h.t.Fatalf("list jobs: %v", err)
	}
	defer rows.Close()
	var out []queuedPublish
	for rows.Next() {
		var raw []byte
		var q queuedPublish
		if err := rows.Scan(&raw, &q.ScheduledAt); err != nil {
			h.t.Fatalf("scan job: %v", err)
		}
		var args jobs.PublishValueArgs
		decodeJSON(h.t, raw, &args)
		q.PostID, q.PublishAt, q.Index = args.PostID.String(), args.PublishAt, args.Index
		out = append(out, q)
	}
	return out
}

func (h *harness) queuedFor(postID string) []queuedPublish {
	var out []queuedPublish
	for _, q := range h.queued() {
		if q.PostID == postID {
			out = append(out, q)
		}
	}
	return out
}

type deliveryView struct {
	Index      int
	State      string
	ExternalID string
	URL        string
	Error      string
}

func (h *harness) deliveries(postID string) []deliveryView {
	h.t.Helper()
	rows, err := h.db.QueryContext(context.Background(), `
		SELECT value_index, state, coalesce(external_id, ''), coalesce(url, ''), coalesce(error, '')
		FROM post_deliveries WHERE post_id = ? ORDER BY publish_at, value_index`, postID)
	if err != nil {
		h.t.Fatalf("list deliveries: %v", err)
	}
	defer rows.Close()
	var out []deliveryView
	for rows.Next() {
		var d deliveryView
		if err := rows.Scan(&d.Index, &d.State, &d.ExternalID, &d.URL, &d.Error); err != nil {
			h.t.Fatalf("scan delivery: %v", err)
		}
		out = append(out, d)
	}
	return out
}

type publishedView struct {
	Status     string
	ReleaseURL string
	Error      string
}

func (h *harness) postState(postID string) publishedView {
	h.t.Helper()
	var v publishedView
	err := h.db.QueryRowContext(context.Background(),
		"SELECT status, coalesce(release_url, ''), coalesce(error, '') FROM posts WHERE id = ?", postID).
		Scan(&v.Status, &v.ReleaseURL, &v.Error)
	if err != nil {
		h.t.Fatalf("post %s: %v", postID, err)
	}
	return v
}

func (h *harness) sentTo(chatID int64) []faketelegram.Sent {
	var out []faketelegram.Sent
	for _, s := range h.bot.Sent() {
		if s.ChatID == chatID {
			out = append(out, s)
		}
	}
	return out
}

func (h *harness) scheduleValues(session requestOption, kind string, at time.Time, channelID string, values []value) string {
	h.t.Helper()
	res := h.createPosts(session, newPosts{Type: kind, PublishAt: rfc(at), Posts: []channelPost{{ChannelID: channelID, Values: values}}})
	expectStatus(h.t, res, http.StatusCreated)
	return res.json(h.t)["posts"].([]any)[0].(map[string]any)["id"].(string)
}

func (h *harness) exec(query string, args ...any) {
	h.t.Helper()
	if _, err := h.db.ExecContext(context.Background(), query, args...); err != nil {
		h.t.Fatalf("%s: %v", query, err)
	}
}

// S05.1 Programar encola la publicación a su hora, sin duplicarla.
func TestSchedulingQueuesThePublicationOnce(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)

	scheduled := h.schedule(session, "schedule", at, "Mañana", channel)[0]
	now := h.schedule(session, "now", at, "Ahora", channel)[0]
	draft := h.schedule(session, "draft", at, "Borrador", channel)[0]

	if q := h.queuedFor(scheduled); len(q) != 1 || !q[0].PublishAt.Equal(at) || q[0].Index != 0 || !q[0].ScheduledAt.Equal(at) {
		t.Fatalf("scheduled post jobs = %+v", q)
	}
	minute := h.clock.Now().UTC().Truncate(time.Minute)
	if q := h.queuedFor(now); len(q) != 1 || !q[0].PublishAt.Equal(minute) {
		t.Fatalf("publish-now jobs = %+v, want one at %s", q, minute)
	}
	if q := h.queuedFor(draft); len(q) != 0 {
		t.Fatalf("draft jobs = %+v", q)
	}

	expectStatus(t, h.edit(session, scheduled, map[string]any{"mode": "update", "publishAt": rfc(at), "values": text("Mañana, corregido")}), http.StatusOK)
	if q := h.queuedFor(scheduled); len(q) != 1 {
		t.Fatalf("jobs after update = %+v", q)
	}
}

// S05.2 Llegada la hora, sale a Telegram y queda Publicado con enlace.
func TestADuePostGoesOutAndLinksToTheMessage(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	public := h.connect(session, canalPublico)
	private := h.connect(session, canalPrivado)
	at := tomorrowAt(h.clock.Now(), 10)
	content := "<p>Hola <strong>mundo</strong></p><ul><li><p>uno</p></li></ul>"
	ids := []string{
		h.scheduleValues(session, "schedule", at, public, []value{{Content: content}}),
		h.scheduleValues(session, "schedule", at, private, []value{{Content: content}}),
	}

	h.clock.Set(at)
	for _, id := range ids {
		h.mustPublishValue(id, at, 0)
	}

	for i, chat := range []faketelegram.Chat{canalPublico, canalPrivado} {
		sent := h.sentTo(chat.ID)
		if len(sent) != 1 || sent[0].Method != "sendMessage" || sent[0].ParseMode != "HTML" || sent[0].Text != "Hola <b>mundo</b>\n• uno" {
			t.Fatalf("sent to %s = %+v", chat.Title, sent)
		}
		state := h.postState(ids[i])
		want := map[int64]string{
			canalPublico.ID: "https://t.me/postik_demo/",
			canalPrivado.ID: "https://t.me/c/1234567890/",
		}[chat.ID]
		if state.Status != "published" || state.ReleaseURL != want+jsonInt(sent[0].MessageID) {
			t.Fatalf("post in %s = %+v", chat.Title, state)
		}
	}
}

func jsonInt(n int64) string {
	encoded, _ := json.Marshal(n)
	return string(encoded)
}

// S05.3 Los medios se suben como fichero, y el tipo decide la llamada.
func TestMediaIsUploadedAsFilesAndPicksTheCall(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	photo := h.mustUpload(session, "foto.png", pngBytes)
	video := h.mustUpload(session, "clip.mp4", mp4Bytes)
	var twelve []map[string]any
	for range 12 {
		twelve = append(twelve, map[string]any{"id": photo.ID})
	}
	at := tomorrowAt(h.clock.Now(), 10)
	withPhoto := h.scheduleValues(session, "schedule", at, channel, []value{{Content: "<p>Foto</p>", Media: []map[string]any{{"id": photo.ID}}}})
	withVideo := h.scheduleValues(session, "schedule", at, channel, []value{{Content: "<p>Vídeo</p>", Media: []map[string]any{{"id": video.ID}}}})
	withAlbum := h.scheduleValues(session, "schedule", at, channel, []value{{Content: "<p>Álbum</p>", Media: twelve}})

	h.clock.Set(at)
	for _, id := range []string{withPhoto, withVideo, withAlbum} {
		h.mustPublishValue(id, at, 0)
	}

	sent := h.sentTo(canalA.ID)
	if len(sent) != 4 {
		t.Fatalf("sent %d messages, want 4: %+v", len(sent), sent)
	}
	if sent[0].Method != "sendPhoto" || sent[0].Text != "Foto" || len(sent[0].Media) != 1 || !bytes.Equal(sent[0].Media[0].Data, pngBytes) {
		t.Fatalf("photo = %+v", sent[0])
	}
	if sent[1].Method != "sendVideo" || sent[1].Text != "Vídeo" || len(sent[1].Media) != 1 || !bytes.Equal(sent[1].Media[0].Data, mp4Bytes) {
		t.Fatalf("video = %+v", sent[1])
	}
	first, second := sent[2], sent[3]
	if first.Method != "sendMediaGroup" || len(first.Media) != 10 || second.Method != "sendMediaGroup" || len(second.Media) != 2 {
		t.Fatalf("album = %+v / %+v", first, second)
	}
	for i, m := range append(first.Media, second.Media...) {
		wantCaption := ""
		if i == 0 {
			wantCaption = "Álbum"
		}
		if m.Caption != wantCaption || m.Type != "photo" || !bytes.Equal(m.Data, pngBytes) {
			t.Fatalf("album item %d = %+v", i, m)
		}
	}
	if state := h.postState(withAlbum); !strings.HasSuffix(state.ReleaseURL, "/"+jsonInt(first.MessageID)) {
		t.Fatalf("album link = %q, want the first message %d", state.ReleaseURL, first.MessageID)
	}
}

// S05.4 Los comentarios salen en orden, tras su retardo, como respuesta.
func TestCommentsFollowInOrderAfterTheirDelay(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.scheduleValues(session, "schedule", at, channel, []value{
		{Content: "<p>Principal</p>"},
		{Content: "<p>Primero</p>", DelayMinutes: 5},
		{Content: "<p>Segundo</p>", DelayMinutes: 0},
	})

	h.clock.Set(at)
	h.mustPublishValue(id, at, 0)
	main := h.sentTo(canalA.ID)[0]
	next := h.queuedFor(id)
	if len(next) != 2 || next[1].Index != 1 || !next[1].ScheduledAt.Equal(at.Add(5*time.Minute)) {
		t.Fatalf("jobs after the main value = %+v", next)
	}

	h.clock.Set(at.Add(5 * time.Minute))
	h.mustPublishValue(id, at, 1)
	first := h.sentTo(canalA.ID)[1]
	if first.Text != "Primero" || first.ReplyTo != main.MessageID {
		t.Fatalf("first comment = %+v, want a reply to %d", first, main.MessageID)
	}
	last := h.queuedFor(id)
	if len(last) != 3 || last[2].Index != 2 || !last[2].ScheduledAt.Equal(at.Add(5*time.Minute)) {
		t.Fatalf("jobs after the first comment = %+v", last)
	}

	h.mustPublishValue(id, at, 2)
	second := h.sentTo(canalA.ID)[2]
	if second.Text != "Segundo" || second.ReplyTo != first.MessageID {
		t.Fatalf("second comment = %+v, want a reply to %d", second, first.MessageID)
	}
}

// S05.5 Si el post cambió, el trabajo no hace nada.
func TestAStaleJobDoesNothing(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	later := at.Add(time.Hour)
	moved := h.schedule(session, "schedule", at, "Movido", channel)[0]
	toDraft := h.schedule(session, "schedule", at, "A borrador", channel)[0]
	deleted := h.schedule(session, "schedule", at, "Borrado", channel)[0]

	expectStatus(t, h.do(http.MethodPut, "/api/v1/posts/"+moved+"/date", map[string]any{"publishAt": rfc(later), "mode": "schedule"}, session), http.StatusNoContent)
	h.exec("UPDATE posts SET status = 'draft' WHERE id = ?", toDraft)
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/posts/"+deleted+"/group", nil, session), http.StatusNoContent)

	h.clock.Set(at)
	for _, id := range []string{moved, toDraft, deleted} {
		h.mustPublishValue(id, at, 0)
	}
	if sent := h.sentTo(canalA.ID); len(sent) != 0 {
		t.Fatalf("stale jobs sent %+v", sent)
	}
	if s := h.postState(moved).Status; s != "scheduled" {
		t.Fatalf("moved post = %s", s)
	}
	if s := h.postState(toDraft).Status; s != "draft" {
		t.Fatalf("draft post = %s", s)
	}
	found := false
	for _, q := range h.queuedFor(moved) {
		found = found || q.PublishAt.Equal(later)
	}
	if !found {
		t.Fatalf("no job at the new date: %+v", h.queuedFor(moved))
	}
}

// S05.6 Con el canal desactivado o pendiente de reconexión no se llama a Telegram.
func TestAnUnavailableChannelFailsWithoutCallingTelegram(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	disabled := h.connect(session, canalA)
	refresh := h.connect(session, canalB)
	at := tomorrowAt(h.clock.Now(), 10)
	first := h.schedule(session, "schedule", at, "Uno", disabled)[0]
	second := h.schedule(session, "schedule", at, "Dos", refresh)[0]

	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+disabled+"/disabled", map[string]bool{"disabled": true}, session), http.StatusNoContent)
	h.exec("UPDATE channels SET refresh_needed = true WHERE id = ?", refresh)

	h.clock.Set(at)
	h.mustPublishValue(first, at, 0)
	h.mustPublishValue(second, at, 0)
	if sent := h.bot.Sent(); len(sent) != 0 {
		t.Fatalf("sent %+v", sent)
	}
	if s := h.postState(first); s.Status != "error" || s.Error != "channel_disabled" {
		t.Fatalf("disabled channel post = %+v", s)
	}
	if s := h.postState(second); s.Status != "error" || s.Error != "channel_refresh" {
		t.Fatalf("refresh channel post = %+v", s)
	}
}

// S05.7 Si Telegram rechaza el post, queda en Error sin reintento.
func TestARejectedPostFailsWithoutRetry(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.schedule(session, "schedule", at, "Largo", channel)[0]

	h.bot.FailNext(http.StatusBadRequest, "Bad Request: message is too long")
	h.clock.Set(at)
	if err := h.publishValue(id, at, 0, 1); err != nil {
		t.Fatalf("a rejection must not ask River to retry: %v", err)
	}
	if s := h.postState(id); s.Status != "error" || s.Error != "Bad Request: message is too long" {
		t.Fatalf("post = %+v", s)
	}
	for _, d := range h.deliveries(id) {
		if d.State == "sending" {
			t.Fatalf("a delivery was left as sending: %+v", d)
		}
	}
}

// S05.8 Si la llamada no llegó a empezar, se reintenta hasta 5 veces.
func TestACallThatNeverStartedIsRetriedUpToFiveTimes(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	limited := h.schedule(session, "schedule", at, "Con límite", channel)[0]
	unreachable := h.schedule(session, "schedule", at, "Sin conexión", channel)[0]
	h.clock.Set(at)

	h.bot.FailNext(http.StatusTooManyRequests, "Too Many Requests: retry after 1")
	if err := h.publishValue(limited, at, 0, 1); err == nil {
		t.Fatal("a 429 must ask River to retry")
	}
	if s := h.postState(limited).Status; s != "scheduled" {
		t.Fatalf("after the 429 the post is %s", s)
	}
	if err := h.publishValue(limited, at, 0, 2); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	if sent := h.sentTo(canalA.ID); len(sent) != 1 || h.postState(limited).Status != "published" {
		t.Fatalf("sent %+v, post %+v", sent, h.postState(limited))
	}

	h.stopTelegram()
	for attempt := 1; attempt <= 4; attempt++ {
		if err := h.publishValue(unreachable, at, 0, attempt); err == nil {
			t.Fatalf("attempt %d without Telegram must ask River to retry", attempt)
		}
		if s := h.postState(unreachable).Status; s != "scheduled" {
			t.Fatalf("after attempt %d the post is %s", attempt, s)
		}
	}
	if err := h.publishValue(unreachable, at, 0, 5); err != nil {
		t.Fatalf("the last attempt must settle the post: %v", err)
	}
	if s := h.postState(unreachable); s.Status != "error" || s.Error != "unreachable" {
		t.Fatalf("after the fifth attempt = %+v", s)
	}
}

// S05.9 Si no se sabe si llegó, no se reenvía nunca.
func TestAnUnconfirmedSendIsNeverRepeated(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	dropped := h.schedule(session, "schedule", at, "Cortado", channel)[0]
	crashed := h.schedule(session, "schedule", at, "Caído", channel)[0]
	h.clock.Set(at)

	h.bot.DropNext()
	if err := h.publishValue(dropped, at, 0, 1); err != nil {
		t.Fatalf("an unconfirmed send must not ask River to retry: %v", err)
	}
	h.exec(`INSERT INTO post_deliveries (post_id, publish_at, value_index, state, created_at, updated_at)
		VALUES (?, ?, 0, 'sending', ?, ?)`, crashed, at, at, at)
	if err := h.publishValue(crashed, at, 0, 2); err != nil {
		t.Fatalf("rescued job: %v", err)
	}

	for _, id := range []string{dropped, crashed} {
		if s := h.postState(id); s.Status != "error" || s.Error != "unconfirmed" {
			t.Fatalf("post %s = %+v", id, s)
		}
	}
	if sent := h.sentTo(canalA.ID); len(sent) != 0 {
		t.Fatalf("the crashed post was sent again: %+v", sent)
	}
}

// S05.10 Un comentario que falla no deshace el principal.
func TestAFailedCommentKeepsThePostPublished(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.scheduleValues(session, "schedule", at, channel, []value{
		{Content: "<p>Principal</p>"},
		{Content: "<p>Primero</p>"},
		{Content: "<p>Segundo</p>"},
	})
	h.clock.Set(at)
	h.mustPublishValue(id, at, 0)
	link := h.postState(id).ReleaseURL

	h.bot.FailNext(http.StatusBadRequest, "Bad Request: replied message not found")
	h.mustPublishValue(id, at, 1)

	if s := h.postState(id); s.Status != "published" || s.ReleaseURL != link {
		t.Fatalf("post after the failed comment = %+v", s)
	}
	d := h.deliveries(id)
	if len(d) != 2 || d[1].Index != 1 || d[1].State != "failed" || d[1].Error != "Bad Request: replied message not found" {
		t.Fatalf("deliveries = %+v", d)
	}
	for _, q := range h.queuedFor(id) {
		if q.Index == 2 {
			t.Fatalf("the second comment was queued: %+v", q)
		}
	}
}

// S05.11 Un trabajo repetido no publica dos veces.
func TestARepeatedJobDoesNotPublishTwice(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.schedule(session, "schedule", at, "Una vez", channel)[0]
	h.clock.Set(at)
	h.mustPublishValue(id, at, 0)
	link := h.postState(id).ReleaseURL

	h.mustPublishValue(id, at, 0)

	if sent := h.sentTo(canalA.ID); len(sent) != 1 {
		t.Fatalf("sent %d messages: %+v", len(sent), sent)
	}
	if s := h.postState(id); s.Status != "published" || s.ReleaseURL != link {
		t.Fatalf("post = %+v", s)
	}
}

// S05.12 El barrido relanza los vencidos de las últimas 48 horas, y solo esos.
func TestTheSweepRelaunchesOnlyRecentOverduePosts(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	off := h.connect(session, canalB)
	at := tomorrowAt(h.clock.Now(), 10)
	recent := h.schedule(session, "schedule", at, "Hace 3 horas", channel)[0]
	old := h.schedule(session, "schedule", at, "Hace 3 días", channel)[0]
	disabled := h.schedule(session, "schedule", at, "Canal apagado", off)[0]
	pending := h.schedule(session, "schedule", at, "Con trabajo", channel)[0]

	now := at.Add(72 * time.Hour)
	h.clock.Set(now)
	h.exec("UPDATE posts SET publish_at = ? WHERE id = ?", now.Add(-3*time.Hour), recent)
	h.exec("UPDATE posts SET publish_at = ? WHERE id = ?", now.Add(-72*time.Hour), old)
	h.exec("UPDATE posts SET publish_at = ? WHERE id = ?", now.Add(-time.Hour), disabled)
	h.exec("UPDATE channels SET disabled = true WHERE id = ?", off)
	h.exec("UPDATE posts SET publish_at = ? WHERE id = ?", now.Add(-time.Hour), pending)
	h.exec("DELETE FROM river.river_job")
	if err := h.jobs.SchedulePost(context.Background(), nil, uuid.MustParse(pending), now.Add(-time.Hour)); err != nil {
		t.Fatalf("queue the pending post: %v", err)
	}

	h.sweep()

	if q := h.queuedFor(recent); len(q) != 1 || !q[0].PublishAt.Equal(now.Add(-3*time.Hour)) {
		t.Fatalf("recent post jobs = %+v", q)
	}
	for name, id := range map[string]string{"old": old, "disabled": disabled} {
		if q := h.queuedFor(id); len(q) != 0 {
			t.Fatalf("%s post was relaunched: %+v", name, q)
		}
	}
	if q := h.queuedFor(pending); len(q) != 1 {
		t.Fatalf("pending post jobs = %+v", q)
	}
	if s := h.postState(old).Status; s != "scheduled" {
		t.Fatalf("old post = %s", s)
	}
}

// S05.13 Republicar envía de nuevo como publicación nueva.
func TestRepublishingSendsANewMessage(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.schedule(session, "schedule", at, "Otra vez", channel)[0]
	h.clock.Set(at)
	h.mustPublishValue(id, at, 0)
	first := h.postState(id).ReleaseURL

	again := at.Add(time.Minute)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/posts/"+id+"/date", map[string]any{"publishAt": rfc(again), "mode": "schedule", "republish": true}, session), http.StatusNoContent)
	h.clock.Set(again)
	h.mustPublishValue(id, again, 0)

	if sent := h.sentTo(canalA.ID); len(sent) != 2 {
		t.Fatalf("sent %+v", sent)
	}
	if s := h.postState(id); s.Status != "published" || s.ReleaseURL == first || s.ReleaseURL == "" {
		t.Fatalf("republished post = %+v, first link %q", s, first)
	}
}

// S05.14 Vincular una publicación sin enlace.
func TestLinkingAPublicationWithoutLink(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	missing := h.schedule(session, "schedule", at, "Sin enlace", channel)[0]
	linked := h.schedule(session, "schedule", at, "Con enlace", channel)[0]
	h.exec("UPDATE posts SET status = 'published', release_url = NULL WHERE id = ?", missing)
	h.exec("UPDATE posts SET status = 'published', release_url = 'https://t.me/postik_demo/1' WHERE id = ?", linked)

	link := func(id, url string) response {
		return h.do(http.MethodPut, "/api/v1/posts/"+id+"/release", map[string]string{"url": url}, session)
	}
	expectStatus(t, link(missing, "http://t.me/postik_demo/7"), http.StatusBadRequest)
	expectStatus(t, link(missing, "https://t.me/postik_demo/7"), http.StatusNoContent)
	if s := h.postState(missing); s.ReleaseURL != "https://t.me/postik_demo/7" {
		t.Fatalf("linked post = %+v", s)
	}
	res := link(linked, "https://t.me/postik_demo/8")
	expectStatus(t, res, http.StatusConflict)
	if code := res.json(t)["code"]; code != "release_not_missing" {
		t.Fatalf("code = %v", code)
	}
}
