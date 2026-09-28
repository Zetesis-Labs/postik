package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/linkedin"
	"github.com/zetesis-labs/postik/internal/testsupport/fakelinkedin"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeoidc"
)

var (
	anaLinkedIn = fakelinkedin.Member{Sub: "ana-li", Name: "Ana García", Picture: []byte("\x89PNG\r\n\x1a\nana")}
	zetesis     = fakelinkedin.PageNamed("Zetesis")
	nexoLabs    = fakelinkedin.PageNamed("Nexo Labs")
	analytics   = func() fakelinkedin.Page {
		p := fakelinkedin.PageNamed("Solo analista")
		p.Role = "ANALYST"
		return p
	}()
	anaAdmin = fakelinkedin.Member{Sub: "ana-li", Name: "Ana García", Picture: []byte("\x89PNG\r\n\x1a\nana"), Pages: []fakelinkedin.Page{zetesis, nexoLabs, analytics}}
)

func withLinkedIn() harnessOption {
	return func(c *config.Config) {
		c.LinkedIn = &config.LinkedIn{ClientID: fakelinkedin.ClientID, ClientSecret: fakelinkedin.ClientSecret, Version: "202609"}
		c.EncryptionKey = bytes.Repeat([]byte{9}, 32)
	}
}

// startLinkedIn is called by newHarness when the configuration asks for LinkedIn.
func startLinkedIn(h *harness, cfg *config.Config) {
	h.linkedIn = fakelinkedin.New("")
	server := httptest.NewServer(h.linkedIn.Handler())
	h.t.Cleanup(server.Close)
	h.linkedIn.Base = server.URL
	cfg.LinkedIn.AuthURL = server.URL
	cfg.LinkedIn.APIURL = server.URL
}

// memberBrowser signs identity in and keeps the browser, which follows the
// OAuth flows with its cookies.
func (h *harness) memberBrowser(identity fakeoidc.Identity) *browser {
	h.t.Helper()
	b := h.newBrowser()
	if location := b.signIn(identity); location != "/launches" {
		h.t.Fatalf("%s landed on %q", identity.Subject, location)
	}
	return b
}

func (b *browser) session() requestOption {
	return withCookie(b.sessionCookie())
}

// startAuthorization asks postik where to send the browser for provider;
// with channelID it reconnects that channel.
func (b *browser) startAuthorization(provider, channelID string) string {
	b.h.t.Helper()
	body := map[string]any{}
	if channelID != "" {
		body["channelId"] = channelID
	}
	res := b.h.do(http.MethodPost, "/api/v1/channels/"+provider+"/authorizations", body, b.session())
	expectStatus(b.h.t, res, http.StatusCreated)
	return res.json(b.h.t)["url"].(string)
}

// authorize runs the whole flow and returns where postik sends the browser.
func (b *browser) authorize(provider, channelID string) string {
	b.h.t.Helper()
	_, location := b.get(b.startAuthorization(provider, channelID))
	return location
}

// connectLinkedIn connects member's profile and returns the channel ID.
func (b *browser) connectLinkedIn(member fakelinkedin.Member) string {
	b.h.t.Helper()
	b.h.linkedIn.SignInNext(member)
	location := b.authorize("linkedin", "")
	id, ok := strings.CutPrefix(location, "/launches?added=")
	if !ok {
		b.h.t.Fatalf("connecting LinkedIn ended at %q", location)
	}
	return id
}

// connectPage connects LinkedIn Page as member, chooses page and returns the channel ID.
func (b *browser) connectPage(member fakelinkedin.Member, page fakelinkedin.Page) string {
	b.h.t.Helper()
	b.h.linkedIn.SignInNext(member)
	location := b.authorize("linkedin-page", "")
	pending, ok := strings.CutPrefix(location, "/launches?continue=")
	if !ok {
		b.h.t.Fatalf("connecting LinkedIn Page ended at %q", location)
	}
	res := b.h.do(http.MethodPut, "/api/v1/channels/"+pending+"/page", map[string]string{"pageId": page.ID}, b.session())
	expectStatus(b.h.t, res, http.StatusOK)
	return res.json(b.h.t)["id"].(string)
}

type channelRow struct {
	Provider       string
	ExternalID     string
	Name           string
	Username       string
	Picture        string
	Disabled       bool
	RefreshNeeded  bool
	InBetweenSteps bool
}

func (h *harness) channelRow(id string) (channelRow, bool) {
	h.t.Helper()
	var c channelRow
	err := h.db.QueryRowContext(context.Background(), `
		SELECT provider, external_id, name, username, coalesce(picture, ''), disabled, refresh_needed, in_between_steps
		FROM channels WHERE id = ?`, id).
		Scan(&c.Provider, &c.ExternalID, &c.Name, &c.Username, &c.Picture, &c.Disabled, &c.RefreshNeeded, &c.InBetweenSteps)
	if err != nil {
		return channelRow{}, false
	}
	return c, true
}

func (h *harness) mustChannelRow(id string) channelRow {
	h.t.Helper()
	c, ok := h.channelRow(id)
	if !ok {
		h.t.Fatalf("channel %s does not exist", id)
	}
	return c
}

func (h *harness) credentialsUpdatedAt(channelID string) time.Time {
	h.t.Helper()
	var at time.Time
	if err := h.db.QueryRowContext(context.Background(), "SELECT updated_at FROM channel_credentials WHERE channel_id = ?", channelID).Scan(&at); err != nil {
		h.t.Fatalf("credentials of %s: %v", channelID, err)
	}
	return at
}

func providerIDs(t *testing.T, h *harness, session requestOption) []string {
	t.Helper()
	res := h.do(http.MethodGet, "/api/v1/channels/providers", nil, session)
	expectStatus(t, res, http.StatusOK)
	var providers []struct{ Identifier string }
	decodeJSON(t, res.body, &providers)
	ids := make([]string, len(providers))
	for i, p := range providers {
		ids[i] = p.Identifier
	}
	return ids
}

// S06.1 La rejilla ofrece solo las redes con credenciales.
func TestOAuthNetworksAreOfferedWithCredentials(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	if ids := providerIDs(t, h, h.member(ana)); !slices.Equal(ids, []string{"linkedin", "linkedin-page"}) {
		t.Fatalf("providers = %v", ids)
	}

	withBot := newHarness(t, withOIDC("Fake"), withLinkedIn(), withTelegram())
	if ids := providerIDs(t, withBot, withBot.member(ana)); !slices.Equal(ids, []string{"linkedin", "linkedin-page", "telegram"}) {
		t.Fatalf("providers with a bot = %v", ids)
	}
}

// S06.2 Conectar LinkedIn crea el canal con su identidad y los tokens cifrados.
func TestConnectingLinkedInCreatesTheChannelWithSealedTokens(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	h.linkedIn.SignInNext(anaLinkedIn)

	start, err := url.Parse(b.startAuthorization("linkedin", ""))
	if err != nil {
		t.Fatal(err)
	}
	q := start.Query()
	if q.Get("state") == "" || q.Get("redirect_uri") != h.server.URL+"/api/v1/channels/linkedin/callback" || q.Get("scope") != strings.Join(linkedin.MemberScopes, " ") {
		t.Fatalf("authorization URL = %s", start)
	}
	_, location := b.get(start.String())
	id, ok := strings.CutPrefix(location, "/launches?added=")
	if !ok {
		t.Fatalf("callback ended at %q", location)
	}

	c := h.mustChannelRow(id)
	if c.Provider != "linkedin" || c.ExternalID != "ana-li" || c.Name != "Ana García" || !strings.HasPrefix(c.Picture, "/uploads/avatars/") {
		t.Fatalf("channel = %+v", c)
	}
	if views := h.channels(b.session()); len(views) != 1 || !slices.Equal(views[0].PostingTimes, []int{120, 400, 700}) {
		t.Fatalf("channels = %+v", views)
	}
	var sealed []byte
	if err := h.db.QueryRowContext(context.Background(), "SELECT access_token FROM channel_credentials WHERE channel_id = ?", id).Scan(&sealed); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, []byte("access-ana-li")) {
		t.Fatal("the access token is stored in the clear")
	}
	if n := h.count("oauth_authorizations"); n != 0 {
		t.Fatalf("%d authorizations left", n)
	}

	legacy := newHarness(t, withOIDC("Fake"), withLinkedIn(), func(c *config.Config) { c.LegacyCallbacks = true })
	lb := legacy.memberBrowser(ana)
	legacy.linkedIn.SignInNext(anaLinkedIn)
	start, _ = url.Parse(lb.startAuthorization("linkedin", ""))
	if got := start.Query().Get("redirect_uri"); got != legacy.server.URL+"/integrations/social/linkedin" {
		t.Fatalf("legacy redirect_uri = %s", got)
	}
	if _, location := lb.get(start.String()); !strings.HasPrefix(location, "/launches?added=") {
		t.Fatalf("legacy callback ended at %q", location)
	}
}

// S06.3 Conectar la misma cuenta otra vez actualiza el canal.
func TestConnectingTheSameAccountAgainUpdatesTheChannel(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	id := b.connectLinkedIn(anaLinkedIn)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/customer", map[string]string{"name": "Cliente A"}, b.session()), http.StatusNoContent)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/posting-times", map[string][]int{"times": {60}}, b.session()), http.StatusNoContent)
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+id+"/disabled", map[string]bool{"disabled": true}, b.session()), http.StatusNoContent)

	h.clock.Advance(time.Minute)
	renamed := anaLinkedIn
	renamed.Name = "Ana G."
	again := b.connectLinkedIn(renamed)

	views := h.channels(b.session())
	if again != id || len(views) != 1 {
		t.Fatalf("reconnected as %s, channels %+v", again, views)
	}
	v := views[0]
	if v.Name != "Ana G." || v.Customer != "Cliente A" || !slices.Equal(v.PostingTimes, []int{60}) || !v.Disabled {
		t.Fatalf("channel = %+v", v)
	}
	if at := h.credentialsUpdatedAt(id); !at.Equal(h.clock.Now()) {
		t.Fatalf("tokens saved at %s, want %s", at, h.clock.Now())
	}
}

// S06.4 Una vuelta inválida no crea canal y explica por qué.
func TestAnInvalidCallbackCreatesNoChannel(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	other := h.memberBrowser(bruno)

	h.linkedIn.DenyNext()
	denied := b.authorize("linkedin", "")

	_, unknown := b.get(h.server.URL + "/api/v1/channels/linkedin/callback?code=x&state=nope")
	h.linkedIn.SignInNext(anaLinkedIn)
	_, foreign := b.get(other.startAuthorization("linkedin", ""))

	late := b.startAuthorization("linkedin", "")
	h.clock.Advance(61 * time.Minute)
	h.linkedIn.SignInNext(anaLinkedIn)
	_, expired := b.get(late)

	h.linkedIn.SignInNext(anaLinkedIn)
	h.linkedIn.GrantFewerNext()
	missing := b.authorize("linkedin", "")

	want := []string{"denied", "invalid_state", "invalid_state", "expired", "missing_permissions"}
	for i, location := range []string{denied, unknown, foreign, expired, missing} {
		if location != "/launches?oauth_error="+want[i] {
			t.Errorf("case %d ended at %q, want %s", i, location, want[i])
		}
	}
	if n := h.count("channels"); n != 0 {
		t.Fatalf("%d channels created", n)
	}
}

// S06.5 Reconectar el perfil deja el canal activo, y solo con la misma cuenta.
func TestReconnectingAProfileNeedsTheSameAccount(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	id := b.connectLinkedIn(anaLinkedIn)
	post := h.scheduleValues(b.session(), "schedule", tomorrowAt(h.clock.Now(), 10), id, []value{{Content: "<p>Programado</p>"}})
	h.exec("UPDATE channels SET refresh_needed = true WHERE id = ?", id)

	h.clock.Advance(time.Minute)
	h.linkedIn.SignInNext(anaLinkedIn)
	if location := b.authorize("linkedin", id); location != "/launches?added="+id {
		t.Fatalf("reconnect ended at %q", location)
	}
	if c := h.mustChannelRow(id); c.RefreshNeeded {
		t.Fatalf("channel still needs reconnection: %+v", c)
	}
	if at := h.credentialsUpdatedAt(id); !at.Equal(h.clock.Now()) {
		t.Fatalf("tokens saved at %s", at)
	}
	if s := h.postState(post); s.Status != "scheduled" {
		t.Fatalf("post = %+v", s)
	}

	other := h.memberBrowser(bruno)
	brunoLinkedIn := fakelinkedin.Member{Sub: "bruno-li", Name: "Bruno"}
	otherID := other.connectLinkedIn(brunoLinkedIn)
	h.exec("UPDATE channels SET refresh_needed = true WHERE id = ?", otherID)
	before := h.credentialsUpdatedAt(otherID)
	h.clock.Advance(time.Minute)
	h.linkedIn.SignInNext(fakelinkedin.Member{Sub: "carla-li", Name: "Carla"})
	if location := other.authorize("linkedin", otherID); location != "/launches?oauth_error=wrong_account" {
		t.Fatalf("reconnect with another account ended at %q", location)
	}
	if c := h.mustChannelRow(otherID); !c.RefreshNeeded || c.Name != "Bruno" || c.ExternalID != "bruno-li" {
		t.Fatalf("channel changed: %+v", c)
	}
	if at := h.credentialsUpdatedAt(otherID); !at.Equal(before) {
		t.Fatal("the tokens changed")
	}
}

type pageView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

func (h *harness) pages(session requestOption, channelID string) []pageView {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/channels/"+channelID+"/pages", nil, session)
	expectStatus(h.t, res, http.StatusOK)
	var pages []pageView
	if err := json.Unmarshal(res.body, &pages); err != nil {
		h.t.Fatal(err)
	}
	return pages
}

// S06.6 Conectar LinkedIn Page deja el canal en paso intermedio y lista las páginas.
func TestConnectingALinkedInPageWaitsForThePage(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	h.linkedIn.SignInNext(anaAdmin)

	start, _ := url.Parse(b.startAuthorization("linkedin-page", ""))
	if start.Query().Get("scope") != strings.Join(linkedin.PageScopes, " ") {
		t.Fatalf("scopes = %q", start.Query().Get("scope"))
	}
	_, location := b.get(start.String())
	id, ok := strings.CutPrefix(location, "/launches?continue=")
	if !ok {
		t.Fatalf("callback ended at %q", location)
	}

	if c := h.mustChannelRow(id); c.Provider != "linkedin-page" || !c.InBetweenSteps {
		t.Fatalf("channel = %+v", c)
	}
	res := h.createPosts(b.session(), newPosts{Type: "schedule", PublishAt: rfc(tomorrowAt(h.clock.Now(), 10)), Posts: []channelPost{{ChannelID: id, Values: []value{{Content: "<p>Hola</p>"}}}}})
	expectStatus(t, res, http.StatusBadRequest)
	if !strings.Contains(string(res.body), "channel_unavailable") {
		t.Fatalf("post on a channel in between steps: %s", res.body)
	}
	pages := h.pages(b.session(), id)
	if len(pages) != 2 || pages[0].ID != zetesis.ID || pages[0].Name != "Zetesis" || pages[1].Name != "Nexo Labs" || pages[0].Picture != h.linkedIn.Base+"/logos/"+zetesis.ID {
		t.Fatalf("pages = %+v", pages)
	}
}

// S06.7 Elegir la página deja el canal listo, y solo si la persona la administra.
func TestChoosingThePageRequiresAdministeringIt(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	h.linkedIn.SignInNext(anaAdmin)
	id, _ := strings.CutPrefix(b.authorize("linkedin-page", ""), "/launches?continue=")

	res := h.do(http.MethodPut, "/api/v1/channels/"+id+"/page", map[string]string{"pageId": analytics.ID}, b.session())
	expectStatus(t, res, http.StatusForbidden)
	if res.json(t)["code"] != "page_not_administered" || !h.mustChannelRow(id).InBetweenSteps {
		t.Fatalf("choosing a page not administered: %s", res.body)
	}

	res = h.do(http.MethodPut, "/api/v1/channels/"+id+"/page", map[string]string{"pageId": zetesis.ID}, b.session())
	expectStatus(t, res, http.StatusOK)
	if res.json(t)["id"] != id {
		t.Fatalf("answered %s", res.body)
	}
	c := h.mustChannelRow(id)
	if c.InBetweenSteps || c.ExternalID != zetesis.ID || c.Name != "Zetesis" || c.Username != "zetesis" || !strings.HasPrefix(c.Picture, "/uploads/avatars/") {
		t.Fatalf("channel = %+v", c)
	}
}

// S06.8 Elegir una página que ya estaba conectada actualiza ese canal.
func TestChoosingAConnectedPageUpdatesItsChannel(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	b := h.memberBrowser(ana)
	existing := b.connectPage(anaAdmin, zetesis)
	post := h.scheduleValues(b.session(), "schedule", tomorrowAt(h.clock.Now(), 10), existing, []value{{Content: "<p>Programado</p>"}})
	h.exec("UPDATE channels SET refresh_needed = true WHERE id = ?", existing)
	h.clock.Advance(time.Minute)

	chosen := b.connectPage(anaAdmin, zetesis)

	if chosen != existing {
		t.Fatalf("answered channel %s, want %s", chosen, existing)
	}
	if c := h.mustChannelRow(existing); c.RefreshNeeded || c.InBetweenSteps {
		t.Fatalf("channel = %+v", c)
	}
	if at := h.credentialsUpdatedAt(existing); !at.Equal(h.clock.Now()) {
		t.Fatalf("tokens saved at %s", at)
	}
	if n := h.count("channels"); n != 1 {
		t.Fatalf("%d channels, the one in between steps should be gone", n)
	}
	if s := h.postState(post); s.Status != "scheduled" {
		t.Fatalf("post = %+v", s)
	}
}

// S06.9 Reconectar la página pide seguir administrándola.
func TestReconnectingAPageRequiresAdministeringIt(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withLinkedIn())
	anaBrowser := h.memberBrowser(ana)
	brunoBrowser := h.memberBrowser(bruno)
	brunoAdmin := fakelinkedin.Member{Sub: "bruno-li", Name: "Bruno", Pages: []fakelinkedin.Page{zetesis}}
	anaPage := anaBrowser.connectPage(anaAdmin, zetesis)
	brunoPage := brunoBrowser.connectPage(brunoAdmin, zetesis)
	h.exec("UPDATE channels SET refresh_needed = true")

	h.linkedIn.SignInNext(anaAdmin)
	if location := anaBrowser.authorize("linkedin-page", anaPage); location != "/launches?added="+anaPage {
		t.Fatalf("reconnect ended at %q", location)
	}
	if c := h.mustChannelRow(anaPage); c.RefreshNeeded || c.InBetweenSteps || c.ExternalID != zetesis.ID {
		t.Fatalf("channel = %+v", c)
	}

	h.linkedIn.SignInNext(fakelinkedin.Member{Sub: "bruno-li", Name: "Bruno"})
	if location := brunoBrowser.authorize("linkedin-page", brunoPage); location != "/launches?oauth_error=wrong_account" {
		t.Fatalf("reconnect without the page ended at %q", location)
	}
	if c := h.mustChannelRow(brunoPage); !c.RefreshNeeded {
		t.Fatalf("channel changed: %+v", c)
	}
}
