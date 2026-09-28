package app_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zetesis-labs/postik/internal/testsupport/faketelegram"
)

var (
	canalA = faketelegram.Chat{ID: -5001, Type: "supergroup", Title: "Canal A", BotAdmin: true}
	canalB = faketelegram.Chat{ID: -5002, Type: "supergroup", Title: "Canal B", BotAdmin: true}
	canalC = faketelegram.Chat{ID: -5003, Type: "supergroup", Title: "Canal C", BotAdmin: true}
)

type value struct {
	Content      string           `json:"content"`
	DelayMinutes int              `json:"delayMinutes"`
	Media        []map[string]any `json:"media"`
}

type channelPost struct {
	ChannelID string         `json:"channelId"`
	Values    []value        `json:"values"`
	Settings  map[string]any `json:"settings"`
}

type newPosts struct {
	Type      string        `json:"type"`
	PublishAt string        `json:"publishAt"`
	Tags      []string      `json:"tags"`
	Posts     []channelPost `json:"posts"`
}

func text(content string) []value {
	return []value{{Content: "<p>" + content + "</p>"}}
}

func rfc(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func (h *harness) createPosts(session requestOption, body newPosts) response {
	h.t.Helper()
	for i := range body.Posts {
		if body.Posts[i].Settings == nil {
			body.Posts[i].Settings = map[string]any{}
		}
	}
	if body.Tags == nil {
		body.Tags = []string{}
	}
	return h.do(http.MethodPost, "/api/v1/posts", body, session)
}

// schedule creates one post per channel with the same text and returns their IDs.
func (h *harness) schedule(session requestOption, kind string, at time.Time, content string, channelIDs ...string) []string {
	h.t.Helper()
	body := newPosts{Type: kind, PublishAt: rfc(at)}
	for _, id := range channelIDs {
		body.Posts = append(body.Posts, channelPost{ChannelID: id, Values: text(content)})
	}
	res := h.createPosts(session, body)
	expectStatus(h.t, res, http.StatusCreated)
	var ids []string
	for _, p := range res.json(h.t)["posts"].([]any) {
		ids = append(ids, p.(map[string]any)["id"].(string))
	}
	return ids
}

type postView struct {
	ID        string
	GroupID   string
	ChannelID string
	Status    string
	PublishAt time.Time
	Values    []map[string]any
	Tags      []map[string]any
}

func (h *harness) post(session requestOption, id string) (int, postView) {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/posts/"+id, nil, session)
	if res.status != http.StatusOK {
		return res.status, postView{}
	}
	body := res.json(h.t)
	at, _ := time.Parse(time.RFC3339, body["publishAt"].(string))
	view := postView{
		ID:        body["id"].(string),
		GroupID:   body["groupId"].(string),
		ChannelID: body["channel"].(map[string]any)["id"].(string),
		Status:    body["status"].(string),
		PublishAt: at,
	}
	for _, v := range body["values"].([]any) {
		view.Values = append(view.Values, v.(map[string]any))
	}
	for _, tag := range body["tags"].([]any) {
		view.Tags = append(view.Tags, tag.(map[string]any))
	}
	return res.status, view
}

func (h *harness) mustPost(session requestOption, id string) postView {
	h.t.Helper()
	status, view := h.post(session, id)
	if status != http.StatusOK {
		h.t.Fatalf("GET post %s: %d", id, status)
	}
	return view
}

func (h *harness) calendar(session requestOption, from, to time.Time, customer string) []string {
	h.t.Helper()
	query := url.Values{"from": {rfc(from)}, "to": {rfc(to)}}
	if customer != "" {
		query.Set("customer", customer)
	}
	res := h.do(http.MethodGet, "/api/v1/posts?"+query.Encode(), nil, session)
	expectStatus(h.t, res, http.StatusOK)
	var items []map[string]any
	decodeJSON(h.t, res.body, &items)
	var ids []string
	for _, item := range items {
		ids = append(ids, item["id"].(string))
	}
	return ids
}

func (h *harness) createTag(session requestOption, name, color string) string {
	h.t.Helper()
	res := h.do(http.MethodPost, "/api/v1/tags", map[string]string{"name": name, "color": color}, session)
	expectStatus(h.t, res, http.StatusCreated)
	return res.json(h.t)["id"].(string)
}

func problemCodes(t *testing.T, res response) []string {
	t.Helper()
	body := res.json(t)
	if body["code"] != "invalid_post" {
		t.Fatalf("body = %s", res.body)
	}
	var codes []string
	for _, raw := range body["problems"].([]any) {
		p := raw.(map[string]any)
		code := p["code"].(string)
		if index, ok := p["valueIndex"].(float64); ok {
			code += "@" + string(rune('0'+int(index)))
		}
		codes = append(codes, code)
	}
	return codes
}

func sameIDs(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, id := range got {
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			return false
		}
	}
	return true
}

func tomorrowAt(now time.Time, hour int) time.Time {
	day := now.UTC().Truncate(24*time.Hour).AddDate(0, 0, 1)
	return day.Add(time.Duration(hour) * time.Hour)
}

func todayAt(now time.Time, hour, minute int) time.Time {
	return now.UTC().Truncate(24 * time.Hour).Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

// S04.8 Programar un post en dos canales crea un grupo con un post por canal.
func TestSchedulingInTwoChannelsCreatesAGroup(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	a, b := h.connect(session, canalA), h.connect(session, canalB)
	tag := h.createTag(session, "Lanzamiento", "#ff0000")
	image := h.mustUpload(session, "foto.png", pngBytes)
	at := tomorrowAt(h.clock.Now(), 10)

	values := []value{
		{Content: "<p>Hola <strong>mundo</strong></p>", Media: []map[string]any{{"id": image.ID}}},
		{Content: "<p>Comentario</p>", DelayMinutes: 5},
	}
	res := h.createPosts(session, newPosts{
		Type: "schedule", PublishAt: rfc(at), Tags: []string{tag},
		Posts: []channelPost{{ChannelID: a, Values: values}, {ChannelID: b, Values: values}},
	})
	expectStatus(t, res, http.StatusCreated)
	body := res.json(t)
	groupID := body["groupId"].(string)
	var ids []string
	for _, p := range body["posts"].([]any) {
		ids = append(ids, p.(map[string]any)["id"].(string))
	}
	if len(ids) != 2 {
		t.Fatalf("posts = %s", res.body)
	}
	channels := map[string]bool{}
	for _, id := range ids {
		p := h.mustPost(session, id)
		channels[p.ChannelID] = true
		if p.GroupID != groupID || p.Status != "scheduled" || !p.PublishAt.Equal(at) {
			t.Errorf("post %s = %+v", id, p)
		}
		if len(p.Values) != 2 || p.Values[1]["delayMinutes"].(float64) != 5 {
			t.Fatalf("values = %+v", p.Values)
		}
		media := p.Values[0]["media"].([]any)
		if len(media) != 1 || media[0].(map[string]any)["url"] != image.URL {
			t.Errorf("media = %+v", media)
		}
		if len(p.Tags) != 1 || p.Tags[0]["name"] != "Lanzamiento" {
			t.Errorf("tags = %+v", p.Tags)
		}
	}
	if !channels[a] || !channels[b] {
		t.Errorf("channels = %v", channels)
	}
	if got := h.calendar(session, tomorrowAt(h.clock.Now(), 0), tomorrowAt(h.clock.Now(), 24), ""); !sameIDs(got, ids...) {
		t.Errorf("calendar = %v, want %v", got, ids)
	}
}

// S04.9 La validación rechaza contenido vacío, demasiado largo o con fecha pasada.
func TestValidationRejectsEmptyLongOrPastPosts(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	one := func(values []value, when time.Time) response {
		return h.createPosts(session, newPosts{Type: "schedule", PublishAt: rfc(when), Posts: []channelPost{{ChannelID: channel, Values: values}}})
	}

	res := one([]value{{Content: "<p></p>"}}, at)
	expectStatus(t, res, http.StatusBadRequest)
	if codes := problemCodes(t, res); strings.Join(codes, ",") != "empty@0" {
		t.Errorf("empty post: %v", codes)
	}
	res = one([]value{{Content: "<p>Hola</p>"}, {Content: "<p> </p>"}}, at)
	expectStatus(t, res, http.StatusBadRequest)
	if codes := problemCodes(t, res); strings.Join(codes, ",") != "empty@1" {
		t.Errorf("empty comment: %v", codes)
	}
	res = one(text(strings.Repeat("a", 4097)), at)
	expectStatus(t, res, http.StatusBadRequest)
	if codes := problemCodes(t, res); strings.Join(codes, ",") != "too_long@0" {
		t.Errorf("too long: %v", codes)
	}
	res = one([]value{{Content: "<p><strong>" + strings.Repeat("a", 4096) + "</strong></p>"}}, at)
	expectStatus(t, res, http.StatusCreated)

	res = one(text("Hola"), h.clock.Now().Add(-time.Hour))
	expectStatus(t, res, http.StatusBadRequest)
	if codes := problemCodes(t, res); strings.Join(codes, ",") != "past_date" {
		t.Errorf("past date: %v", codes)
	}
	var posts int
	if err := h.db.QueryRowContext(context.Background(), "SELECT count(*) FROM posts").Scan(&posts); err != nil || posts != 1 {
		t.Fatalf("posts stored = %d (%v), want only the valid one", posts, err)
	}
}

// S04.10 Un borrador solo exige contenido y no se programa.
func TestDraftsOnlyNeedContent(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	yesterday := h.clock.Now().Add(-24 * time.Hour).Truncate(time.Minute)

	ids := h.schedule(session, "draft", yesterday, "Idea", channel)
	if p := h.mustPost(session, ids[0]); p.Status != "draft" || !p.PublishAt.Equal(yesterday) {
		t.Fatalf("draft = %+v", p)
	}
	res := h.createPosts(session, newPosts{Type: "draft", PublishAt: rfc(yesterday), Posts: []channelPost{{ChannelID: channel, Values: text("")}}})
	expectStatus(t, res, http.StatusBadRequest)
	if codes := problemCodes(t, res); strings.Join(codes, ",") != "empty@0" {
		t.Errorf("empty draft: %v", codes)
	}
}

// S04.11 «Publicar ya» usa la hora del servidor.
func TestPublishNowUsesTheServerClock(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	h.clock.Set(time.Date(2026, 9, 28, 12, 34, 56, 0, time.UTC))

	ids := h.schedule(session, "now", h.clock.Now().AddDate(-1, 0, 0), "Ya", channel)
	p := h.mustPost(session, ids[0])
	if p.Status != "scheduled" || !p.PublishAt.Equal(time.Date(2026, 9, 28, 12, 34, 0, 0, time.UTC)) {
		t.Fatalf("post = %+v", p)
	}
}

func (h *harness) nextSlot(session requestOption, channelID string) time.Time {
	h.t.Helper()
	path := "/api/v1/posts/next-slot"
	if channelID != "" {
		path += "?channelId=" + channelID
	}
	res := h.do(http.MethodGet, path, nil, session)
	expectStatus(h.t, res, http.StatusOK)
	at, err := time.Parse(time.RFC3339, res.json(h.t)["date"].(string))
	if err != nil {
		h.t.Fatalf("slot: %v", err)
	}
	return at
}

// S04.12 El siguiente hueco libre salta lo ocupado y cambia de día.
func TestNextFreeSlotSkipsTakenMinutesAndDays(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	h.clock.Set(time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC))
	a, b := h.connect(session, canalA), h.connect(session, canalB)
	h.schedule(session, "schedule", todayAt(h.clock.Now(), 6, 40), "Ocupa", b)

	if got := h.nextSlot(session, a); !got.Equal(todayAt(h.clock.Now(), 11, 40)) {
		t.Fatalf("first slot = %s", got)
	}
	h.schedule(session, "schedule", todayAt(h.clock.Now(), 11, 40), "Ocupa", b)
	if got := h.nextSlot(session, a); !got.Equal(tomorrowAt(h.clock.Now(), 2)) {
		t.Fatalf("after filling the day = %s", got)
	}
	c := h.connect(session, canalC)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+c+"/posting-times", map[string][]int{"times": {900}}, session), http.StatusNoContent)
	if got := h.nextSlot(session, ""); !got.Equal(todayAt(h.clock.Now(), 15, 0)) {
		t.Fatalf("without channel = %s", got)
	}
}

// S04.13 El calendario trae el rango pedido, filtra por cliente y la lista pagina.
func TestCalendarRangeCustomerAndList(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	anaSession, brunoSession := h.member(ana), h.member(bruno)
	a, b := h.connect(anaSession, canalA), h.connect(anaSession, canalB)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+a+"/customer", map[string]string{"name": "Acme"}, anaSession), http.StatusNoContent)
	acme := h.customers(anaSession)["Acme"]
	now := h.clock.Now()

	draft := h.schedule(anaSession, "draft", now.Add(-24*time.Hour), "Ayer", a)[0]
	todayA := h.schedule(anaSession, "schedule", todayAt(now, 12, 0), "Hoy A", a)[0]
	todayB := h.schedule(anaSession, "schedule", todayAt(now, 14, 0), "Hoy B", b)[0]
	h.schedule(anaSession, "schedule", now.AddDate(0, 0, 10), "Lejos", b)
	brunoChannel := h.connect(brunoSession, grupoBruno)
	h.schedule(brunoSession, "schedule", todayAt(now, 12, 0), "Bruno", brunoChannel)

	weekStart := todayAt(now, 0, 0)
	weekEnd := weekStart.AddDate(0, 0, 7)
	if got := h.calendar(anaSession, weekStart, weekEnd, ""); !sameIDs(got, todayA, todayB) {
		t.Errorf("week = %v, want %v", got, []string{todayA, todayB})
	}
	if got := h.calendar(anaSession, weekStart, weekEnd, acme); !sameIDs(got, todayA) {
		t.Errorf("week for Acme = %v", got)
	}
	res := h.do(http.MethodGet, "/api/v1/posts/list?status=draft", nil, anaSession)
	expectStatus(t, res, http.StatusOK)
	var ids []string
	for _, item := range res.json(t)["items"].([]any) {
		ids = append(ids, item.(map[string]any)["id"].(string))
	}
	if !sameIDs(ids, draft) {
		t.Errorf("drafts = %v", ids)
	}
}

func (h *harness) edit(session requestOption, id string, body map[string]any) response {
	h.t.Helper()
	if _, ok := body["values"]; !ok {
		body["values"] = text("Texto")
	}
	if _, ok := body["settings"]; !ok {
		body["settings"] = map[string]any{}
	}
	if _, ok := body["tags"]; !ok {
		body["tags"] = []string{}
	}
	return h.do(http.MethodPut, "/api/v1/posts/"+id, body, session)
}

// S04.14 Editar actualiza sin tocar el estado, o reprograma.
func TestEditingUpdatesOrReschedules(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	draft := h.schedule(session, "draft", at, "Borrador", channel)[0]
	scheduled := h.schedule(session, "schedule", at, "Programado", channel)[0]

	expectStatus(t, h.edit(session, draft, map[string]any{"mode": "update", "publishAt": rfc(at), "values": text("Nuevo texto")}), http.StatusOK)
	p := h.mustPost(session, draft)
	if p.Status != "draft" || !strings.Contains(p.Values[0]["content"].(string), "Nuevo texto") {
		t.Fatalf("draft after update = %+v", p)
	}
	later := at.Add(3 * time.Hour)
	expectStatus(t, h.edit(session, scheduled, map[string]any{"mode": "schedule", "publishAt": rfc(later)}), http.StatusOK)
	if p := h.mustPost(session, scheduled); p.Status != "scheduled" || !p.PublishAt.Equal(later) {
		t.Fatalf("rescheduled = %+v", p)
	}
}

func (h *harness) markPublished(id string) {
	h.t.Helper()
	if _, err := h.db.ExecContext(context.Background(), "UPDATE posts SET status = 'published' WHERE id = ?", id); err != nil {
		h.t.Fatalf("mark published: %v", err)
	}
}

// S04.15 Reprogramar un post publicado exige confirmarlo.
func TestReschedulingAPublishedPostNeedsConfirmation(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	first := h.schedule(session, "schedule", at, "Uno", channel)[0]
	second := h.schedule(session, "schedule", at, "Dos", channel)[0]
	h.markPublished(first)
	h.markPublished(second)
	later := at.Add(time.Hour)

	res := h.edit(session, first, map[string]any{"mode": "schedule", "publishAt": rfc(later)})
	expectStatus(t, res, http.StatusConflict)
	if res.json(t)["code"] != "republish_required" {
		t.Errorf("body = %s", res.body)
	}
	if p := h.mustPost(session, first); p.Status != "published" || !p.PublishAt.Equal(at) {
		t.Fatalf("post changed without confirmation: %+v", p)
	}
	expectStatus(t, h.edit(session, first, map[string]any{"mode": "schedule", "republish": true, "publishAt": rfc(later)}), http.StatusOK)
	if p := h.mustPost(session, first); p.Status != "scheduled" {
		t.Fatalf("republished = %+v", p)
	}

	res = h.do(http.MethodPut, "/api/v1/posts/"+second+"/date", map[string]any{"publishAt": rfc(later), "mode": "update"}, session)
	expectStatus(t, res, http.StatusNoContent)
	if p := h.mustPost(session, second); p.Status != "published" || !p.PublishAt.Equal(later) {
		t.Fatalf("date-only update = %+v", p)
	}
}

// S04.16 Mover a un hueco pasado se rechaza y un borrador sigue siendo borrador.
func TestMovingRules(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	draft := h.schedule(session, "draft", at, "Borrador", channel)[0]
	scheduled := h.schedule(session, "schedule", at, "Programado", channel)[0]

	res := h.do(http.MethodPut, "/api/v1/posts/"+scheduled+"/date", map[string]any{"publishAt": rfc(h.clock.Now().Add(-24 * time.Hour)), "mode": "schedule"}, session)
	expectStatus(t, res, http.StatusBadRequest)
	if res.json(t)["code"] != "past_date" {
		t.Errorf("body = %s", res.body)
	}
	if p := h.mustPost(session, scheduled); !p.PublishAt.Equal(at) {
		t.Fatalf("the rejected move changed the date: %+v", p)
	}
	later := at.Add(24 * time.Hour)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/posts/"+draft+"/date", map[string]any{"publishAt": rfc(later), "mode": "schedule"}, session), http.StatusNoContent)
	if p := h.mustPost(session, draft); p.Status != "draft" || !p.PublishAt.Equal(later) {
		t.Fatalf("moved draft = %+v", p)
	}
}

// S04.17 Borrar un post borra su grupo en todos los canales.
func TestDeletingAPostDeletesItsGroup(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	a, b := h.connect(session, canalA), h.connect(session, canalB)
	ids := h.schedule(session, "schedule", tomorrowAt(h.clock.Now(), 10), "Grupo", a, b)

	expectStatus(t, h.do(http.MethodDelete, "/api/v1/posts/"+ids[0]+"/group", nil, session), http.StatusNoContent)
	for _, id := range ids {
		if status, _ := h.post(session, id); status != http.StatusNotFound {
			t.Errorf("post %s: status %d", id, status)
		}
	}
	if groups := h.count("post_groups"); groups != 0 {
		t.Errorf("groups left = %d", groups)
	}
}

// S04.18 Borrar un canal se lleva sus posts.
func TestDeletingAChannelTakesItsPosts(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	a, b := h.connect(session, canalA), h.connect(session, canalB)
	ids := h.schedule(session, "schedule", tomorrowAt(h.clock.Now(), 10), "Grupo", a, b)

	expectStatus(t, h.do(http.MethodDelete, "/api/v1/channels/"+a, nil, session), http.StatusNoContent)
	remaining := 0
	for _, id := range ids {
		if status, p := h.post(session, id); status == http.StatusOK {
			remaining++
			if p.ChannelID != b {
				t.Errorf("the post left is in %s", p.ChannelID)
			}
		}
	}
	if remaining != 1 {
		t.Fatalf("posts left = %d, want 1", remaining)
	}
}

// S04.19 Las etiquetas tienen nombre único y al borrarlas se quitan de los grupos.
func TestTagsAreUniqueAndLeaveGroupsWhenDeleted(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	channel := h.connect(session, canalA)
	tag := h.createTag(session, "Lanzamiento", "#ff0000")
	res := h.createPosts(session, newPosts{Type: "schedule", PublishAt: rfc(tomorrowAt(h.clock.Now(), 10)), Tags: []string{tag},
		Posts: []channelPost{{ChannelID: channel, Values: text("Con etiqueta")}}})
	expectStatus(t, res, http.StatusCreated)
	id := res.json(t)["posts"].([]any)[0].(map[string]any)["id"].(string)

	expectStatus(t, h.do(http.MethodPost, "/api/v1/tags", map[string]string{"name": "Lanzamiento", "color": "#000000"}, session), http.StatusConflict)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/tags/"+tag, map[string]string{"name": "Lanzamiento", "color": "#00ff00"}, session), http.StatusOK)
	if p := h.mustPost(session, id); len(p.Tags) != 1 || p.Tags[0]["color"] != "#00ff00" {
		t.Fatalf("tags = %+v", p.Tags)
	}
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/tags/"+tag, nil, session), http.StatusNoContent)
	if p := h.mustPost(session, id); len(p.Tags) != 0 {
		t.Fatalf("tags after delete = %+v", p.Tags)
	}
}

// S04.20 Los posts, canales, etiquetas y medios de otra organización no se usan ni se ven.
func TestPostsOfAnotherOrganizationAreOutOfReach(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	anaSession, brunoSession := h.member(ana), h.member(bruno)
	own := h.connect(anaSession, canalA)
	theirs := h.connect(brunoSession, grupoBruno)
	theirTag := h.createTag(brunoSession, "Suya", "#123456")
	theirMedia := h.mustUpload(brunoSession, "suya.png", pngBytes)
	at := tomorrowAt(h.clock.Now(), 10)
	theirPost := h.schedule(brunoSession, "schedule", at, "De Bruno", theirs)[0]

	res := h.createPosts(anaSession, newPosts{Type: "schedule", PublishAt: rfc(at), Posts: []channelPost{{ChannelID: theirs, Values: text("Robado")}}})
	expectStatus(t, res, http.StatusBadRequest)
	res = h.createPosts(anaSession, newPosts{Type: "schedule", PublishAt: rfc(at), Tags: []string{theirTag}, Posts: []channelPost{{ChannelID: own, Values: text("Etiqueta ajena")}}})
	expectStatus(t, res, http.StatusBadRequest)
	res = h.createPosts(anaSession, newPosts{Type: "schedule", PublishAt: rfc(at), Posts: []channelPost{{ChannelID: own, Values: []value{{Content: "<p>Medio ajeno</p>", Media: []map[string]any{{"id": theirMedia.ID}}}}}}})
	expectStatus(t, res, http.StatusBadRequest)

	if status, _ := h.post(anaSession, theirPost); status != http.StatusNotFound {
		t.Errorf("GET: %d", status)
	}
	expectStatus(t, h.edit(anaSession, theirPost, map[string]any{"mode": "update", "publishAt": rfc(at)}), http.StatusNotFound)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/posts/"+theirPost+"/date", map[string]any{"publishAt": rfc(at.Add(time.Hour)), "mode": "update"}, anaSession), http.StatusNotFound)
	expectStatus(t, h.do(http.MethodDelete, "/api/v1/posts/"+theirPost+"/group", nil, anaSession), http.StatusNotFound)
	if p := h.mustPost(brunoSession, theirPost); p.Status != "scheduled" || !p.PublishAt.Equal(at) {
		t.Fatalf("Bruno's post changed: %+v", p)
	}
}
