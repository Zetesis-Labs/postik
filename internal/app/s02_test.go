package app_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/testsupport/fakeoidc"
)

var ana = fakeoidc.Identity{Subject: "ana-1", Email: "ana@example.com", Name: "Ana Pérez"}

type meOrganization struct {
	id   string
	name string
	role string
}

type meView struct {
	kind          string
	name          string
	email         string
	organizations []meOrganization
	active        string
}

func (h *harness) me(options ...requestOption) (int, meView) {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/me", nil, options...)
	if res.status != http.StatusOK {
		return res.status, meView{}
	}
	body := res.json(h.t)
	view := meView{kind: body["kind"].(string)}
	if user, ok := body["user"].(map[string]any); ok {
		view.name, _ = user["name"].(string)
		view.email, _ = user["email"].(string)
	}
	if orgs, ok := body["organizations"].([]any); ok {
		for _, raw := range orgs {
			org := raw.(map[string]any)
			view.organizations = append(view.organizations, meOrganization{
				id: org["id"].(string), name: org["name"].(string), role: org["role"].(string),
			})
		}
	}
	view.active, _ = body["activeOrganizationId"].(string)
	return res.status, view
}

func (h *harness) count(table string) int {
	h.t.Helper()
	var n int
	if err := h.db.QueryRowContext(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		h.t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// addOrganization creates an organization with userID as a member who joined at joinedAt.
func (h *harness) addOrganization(name string, userID string, joinedAt time.Time) string {
	h.t.Helper()
	ctx := context.Background()
	orgID := uuid.NewString()
	if _, err := h.db.ExecContext(ctx, "INSERT INTO organizations (id, name, created_at) VALUES (?, ?, ?)", orgID, name, joinedAt); err != nil {
		h.t.Fatalf("insert organization: %v", err)
	}
	if userID != "" {
		if _, err := h.db.ExecContext(ctx, "INSERT INTO memberships (organization_id, user_id, role, created_at) VALUES (?, ?, 'USER', ?)", orgID, userID, joinedAt); err != nil {
			h.t.Fatalf("insert membership: %v", err)
		}
	}
	return orgID
}

func (h *harness) userID(subject string) string {
	h.t.Helper()
	var id string
	if err := h.db.QueryRowContext(context.Background(), "SELECT id FROM users WHERE subject = ?", subject).Scan(&id); err != nil {
		h.t.Fatalf("user %s: %v", subject, err)
	}
	return id
}

func orgCookie(id string) requestOption {
	return withCookie(&http.Cookie{Name: "postik_org", Value: id})
}

// S02.1 La pantalla de acceso ofrece el proveedor configurado.
func TestInstanceAdvertisesTheProvider(t *testing.T) {
	with := newHarness(t, withOIDC("Zetesis"))
	body := with.do(http.MethodGet, "/api/v1/instance", nil).json(t)
	oidc, _ := body["oidc"].(map[string]any)
	if oidc["name"] != "Zetesis" {
		t.Fatalf("instance = %s", with.do(http.MethodGet, "/api/v1/instance", nil).body)
	}

	without := newHarness(t)
	if _, present := without.do(http.MethodGet, "/api/v1/instance", nil).json(t)["oidc"]; present {
		t.Fatal("without a provider the instance must not advertise one")
	}
}

// S02.2 La primera entrada crea a la persona y su organización.
func TestFirstAccessCreatesThePersonAndTheirOrganization(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	b := h.newBrowser()

	if location := b.signIn(ana); location != "/launches" {
		t.Fatalf("landed on %q, want /launches", location)
	}
	status, me := h.me(withCookie(b.sessionCookie()))
	if status != http.StatusOK || me.kind != "member" {
		t.Fatalf("me: %d %+v", status, me)
	}
	if me.name != "Ana Pérez" || me.email != "ana@example.com" {
		t.Errorf("person = %q <%s>", me.name, me.email)
	}
	if len(me.organizations) != 1 || me.organizations[0].name != "Ana Pérez" || me.organizations[0].role != "OWNER" {
		t.Fatalf("organizations = %+v", me.organizations)
	}
	if me.active != me.organizations[0].id {
		t.Errorf("active = %s, want %s", me.active, me.organizations[0].id)
	}
}

// S02.3 Volver a entrar no duplica nada ni cambia los datos.
func TestSignInAgainKeepsThePerson(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	h.newBrowser().signIn(ana)

	changed := fakeoidc.Identity{Subject: ana.Subject, Email: "ana@otra.example", Name: "Ana P."}
	b := h.newBrowser()
	if location := b.signIn(changed); location != "/launches" {
		t.Fatalf("landed on %q", location)
	}
	if users, orgs := h.count("users"), h.count("organizations"); users != 1 || orgs != 1 {
		t.Fatalf("users=%d organizations=%d, want 1 and 1", users, orgs)
	}
	_, me := h.me(withCookie(b.sessionCookie()))
	if me.name != "Ana Pérez" || me.email != "ana@example.com" {
		t.Errorf("the second access changed the person: %q <%s>", me.name, me.email)
	}
}

// S02.4 Sin email, la primera entrada se deniega.
func TestFirstAccessWithoutEmailIsDenied(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	b := h.newBrowser()

	location := b.signIn(fakeoidc.Identity{Subject: "sin-email", Name: "Sin Email"})
	if location != "/auth/login?error=email_required" {
		t.Fatalf("landed on %q", location)
	}
	if b.sessionCookie() != nil {
		t.Error("a denied access opened a session")
	}
	if users, orgs := h.count("users"), h.count("organizations"); users != 0 || orgs != 0 {
		t.Fatalf("users=%d organizations=%d, want nothing created", users, orgs)
	}
}

// S02.5 Con «exigir invitación», quien no tiene cuenta no entra.
func TestRequiredInvitationKeepsNewPeopleOut(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), requireInvitation())
	if _, err := h.db.ExecContext(context.Background(),
		"INSERT INTO users (id, issuer, subject, email, name, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.NewString(), h.config.OIDC.Issuer, ana.Subject, ana.Email, ana.Name, h.clock.Now()); err != nil {
		t.Fatalf("seed Ana: %v", err)
	}

	newcomer := h.newBrowser()
	if location := newcomer.signIn(fakeoidc.Identity{Subject: "nuevo", Email: "nuevo@example.com"}); location != "/auth/login?error=invitation_required" {
		t.Fatalf("newcomer landed on %q", location)
	}
	if users := h.count("users"); users != 1 {
		t.Fatalf("users = %d, want only Ana", users)
	}
	if location := h.newBrowser().signIn(ana); location != "/launches" {
		t.Fatalf("Ana landed on %q", location)
	}
}

// S02.6 Si el proveedor rechaza la autorización, se vuelve al acceso.
func TestDeniedAuthorizationReturnsToSignIn(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	b := h.newBrowser()
	h.oidc.DenyNext()

	_, location := b.get(h.server.URL + "/api/v1/auth/oidc/login")
	if location != "/auth/login?error=oidc_denied" {
		t.Fatalf("landed on %q", location)
	}
	if b.sessionCookie() != nil {
		t.Error("a denied authorization opened a session")
	}
}

// callbackURL starts a login in b and returns the callback URL the provider
// sends back, without following it.
func callbackURL(t *testing.T, h *harness, b *browser) string {
	t.Helper()
	h.oidc.SignInNext(ana)
	stop := b.client.CheckRedirect
	b.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if strings.HasSuffix(req.URL.Path, "/api/v1/auth/oidc/callback") {
			return http.ErrUseLastResponse
		}
		return stop(req, via)
	}
	defer func() { b.client.CheckRedirect = stop }()
	res, err := b.client.Get(h.server.URL + "/api/v1/auth/oidc/login")
	if err != nil {
		t.Fatalf("start login: %v", err)
	}
	res.Body.Close()
	return res.Header.Get("Location")
}

// S02.7 Un retorno que no corresponde al navegador o ha caducado se rechaza.
func TestCallbackRejectsForeignOrExpiredState(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))

	t.Run("without the state cookie", func(t *testing.T) {
		callback := callbackURL(t, h, h.newBrowser())
		stranger := h.newBrowser()
		if _, location := stranger.get(callback); location != "/auth/login?error=oidc_state" {
			t.Fatalf("landed on %q", location)
		}
		if stranger.sessionCookie() != nil {
			t.Error("opened a session")
		}
	})

	t.Run("with another state", func(t *testing.T) {
		b := h.newBrowser()
		callback, _ := url.Parse(callbackURL(t, h, b))
		q := callback.Query()
		q.Set("state", "not-the-state")
		callback.RawQuery = q.Encode()
		if _, location := b.get(callback.String()); location != "/auth/login?error=oidc_state" {
			t.Fatalf("landed on %q", location)
		}
	})

	t.Run("after ten minutes", func(t *testing.T) {
		b := h.newBrowser()
		callback := callbackURL(t, h, b)
		h.clock.Advance(10*time.Minute + time.Second)
		defer h.clock.Advance(-10*time.Minute - time.Second)
		if _, location := b.get(callback); location != "/auth/login?error=oidc_state" {
			t.Fatalf("landed on %q", location)
		}
	})

	t.Run("used twice", func(t *testing.T) {
		b := h.newBrowser()
		callback := callbackURL(t, h, b)
		app, _ := url.Parse(h.server.URL)
		stateCookies := b.client.Jar.Cookies(app)
		if _, location := b.get(callback); location != "/launches" {
			t.Fatalf("first use landed on %q", location)
		}
		replay := h.newBrowser()
		replay.client.Jar.SetCookies(app, stateCookies)
		if _, location := replay.get(callback); location != "/auth/login?error=oidc_state" {
			t.Fatalf("replay landed on %q", location)
		}
		if replay.sessionCookie() != nil {
			t.Error("the replay opened a session")
		}
	})
}

// S02.8 La sesión de un miembro caduca tras 7 días sin uso y como mucho a los 30.
func TestMemberSessionExpiresWhenIdleOrOld(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	day1 := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	on := func(day int) time.Time { return day1.AddDate(0, 0, day-1) }

	h.clock.Set(on(1))
	idle := h.newBrowser()
	idle.signIn(ana)
	idleCookie := withCookie(idle.sessionCookie())
	for _, day := range []int{7, 13} {
		h.clock.Set(on(day))
		if status, _ := h.me(idleCookie); status != http.StatusOK {
			t.Fatalf("day %d: status %d, want the session alive", day, status)
		}
	}
	h.clock.Set(on(21))
	if status, _ := h.me(idleCookie); status != http.StatusUnauthorized {
		t.Fatalf("day 21: status %d, want expired after 8 idle days", status)
	}

	h.clock.Set(on(1))
	busy := h.newBrowser()
	busy.signIn(ana)
	busyCookie := withCookie(busy.sessionCookie())
	for day := 2; day <= 30; day++ {
		h.clock.Set(on(day))
		if status, _ := h.me(busyCookie); status != http.StatusOK {
			t.Fatalf("day %d: status %d", day, status)
		}
	}
	h.clock.Set(on(31))
	if status, _ := h.me(busyCookie); status != http.StatusUnauthorized {
		t.Fatalf("day 31: status %d, want expired at 30 days", status)
	}
}

// S02.9 La organización activa es la recordada si sigue siendo suya y, si no, la primera.
func TestActiveOrganizationFallsBackToTheFirstOwnOne(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	b := h.newBrowser()
	b.signIn(ana)
	session := withCookie(b.sessionCookie())
	anaID := h.userID(ana.Subject)
	norte := h.addOrganization("Agencia Norte", anaID, h.clock.Now().Add(time.Hour))
	foreign := h.addOrganization("Ajena", "", h.clock.Now())

	_, me := h.me(session)
	own := me.organizations[0].id
	if me.active != own || me.organizations[0].name != "Ana Pérez" {
		t.Fatalf("without cookie: active %s, organizations %+v", me.active, me.organizations)
	}
	if _, me = h.me(session, orgCookie(norte)); me.active != norte {
		t.Fatalf("remembered Agencia Norte: active %s", me.active)
	}
	_, me = h.me(session, orgCookie(foreign))
	if me.active != own {
		t.Fatalf("remembered foreign organization: active %s", me.active)
	}
	for _, org := range me.organizations {
		if org.id == foreign {
			t.Fatal("the foreign organization is listed")
		}
	}
	if len(me.organizations) != 2 {
		t.Fatalf("organizations = %+v", me.organizations)
	}
}

// S02.10 Solo se puede cambiar a una organización propia.
func TestSwitchingOnlyToOwnOrganizations(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	b := h.newBrowser()
	b.signIn(ana)
	session := withCookie(b.sessionCookie())
	norte := h.addOrganization("Agencia Norte", h.userID(ana.Subject), h.clock.Now().Add(time.Hour))
	foreign := h.addOrganization("Ajena", "", h.clock.Now())

	res := h.do(http.MethodPost, "/api/v1/me/active-organization", map[string]string{"organizationId": norte}, session)
	expectStatus(t, res, http.StatusNoContent)
	var remembered *http.Cookie
	for _, c := range res.cookies {
		if c.Name == "postik_org" {
			remembered = c
		}
	}
	if remembered == nil || remembered.Value != norte {
		t.Fatalf("postik_org cookie = %+v", remembered)
	}
	if _, me := h.me(session, withCookie(remembered)); me.active != norte {
		t.Fatalf("active %s, want Agencia Norte", me.active)
	}

	denied := h.do(http.MethodPost, "/api/v1/me/active-organization", map[string]string{"organizationId": foreign}, session, withCookie(remembered))
	expectStatus(t, denied, http.StatusNotFound)
	for _, c := range denied.cookies {
		if c.Name == "postik_org" {
			t.Fatalf("a foreign switch changed the cookie: %+v", c)
		}
	}
	if _, me := h.me(session, withCookie(remembered)); me.active != norte {
		t.Fatalf("active %s after the rejected switch", me.active)
	}
}
