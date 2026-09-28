package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/jobs"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeoidc"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeresend"
)

func withResend(server **fakeresend.Server) harnessOption {
	return func(c *config.Config) {
		fake := fakeresend.New()
		api := httptest.NewServer(fake.Handler())
		*server = fake
		c.Email = &config.Email{APIKey: fakeresend.APIKey, From: "postik <postik@example.com>", APIURL: api.URL}
	}
}

// joinAs signs a person in, makes them a member of an organization and
// switches their session to it.
func (h *harness) joinAs(identity fakeoidc.Identity, orgID string) requestOption {
	h.t.Helper()
	session := h.member(identity)
	h.exec("INSERT INTO memberships (organization_id, user_id, role, created_at) VALUES (?, ?, 'USER', ?)", orgID, h.userID(identity.Subject), h.clock.Now())
	res := h.do(http.MethodPost, "/api/v1/me/active-organization", map[string]string{"organizationId": orgID}, session)
	expectStatus(h.t, res, http.StatusNoContent)
	var org requestOption
	for _, c := range res.cookies {
		if c.Name == "postik_org" {
			org = withCookie(c)
		}
	}
	return func(r *http.Request) {
		session(r)
		org(r)
	}
}

func (h *harness) activeOrganization(session requestOption) string {
	h.t.Helper()
	_, me := h.me(session)
	return me.active
}

type notificationView struct {
	Kind     string
	Template string
	Params   map[string]any
	Unread   bool
}

func (h *harness) notifications(session requestOption) ([]notificationView, int) {
	h.t.Helper()
	res := h.do(http.MethodGet, "/api/v1/notifications", nil, session)
	expectStatus(h.t, res, http.StatusOK)
	body := res.json(h.t)
	var out []notificationView
	for _, raw := range body["items"].([]any) {
		item := raw.(map[string]any)
		out = append(out, notificationView{
			Kind:     item["kind"].(string),
			Template: item["template"].(string),
			Params:   item["params"].(map[string]any),
			Unread:   item["unread"].(bool),
		})
	}
	return out, int(body["unread"].(float64))
}

func (h *harness) insertNotification(orgID string, at time.Time) {
	h.t.Helper()
	h.exec(`INSERT INTO notifications (id, organization_id, kind, template, params, created_at)
		VALUES (?, ?, 'success', 'published', '{"url": "https://t.me/postik_demo/1"}', ?)`, uuid.NewString(), orgID, at)
}

func (h *harness) setEmailPreferences(subject string, success, failure bool) {
	h.t.Helper()
	h.exec("UPDATE users SET email_success = ?, email_failure = ? WHERE subject = ?", success, failure, subject)
}

func (h *harness) digest() {
	h.t.Helper()
	job := &river.Job[jobs.SuccessDigestArgs]{JobRow: &rivertype.JobRow{Attempt: 1}}
	if err := h.jobs.SuccessDigest.Work(context.Background(), job); err != nil {
		h.t.Fatalf("digest: %v", err)
	}
}

// S05.17 Cada resultado crea su notificación.
func TestEveryOutcomeCreatesItsNotification(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"), withTelegram())
	session := h.member(ana)
	good := h.connect(session, canalPublico)
	bad := h.connect(session, canalA)
	off := h.connect(session, canalB)
	at := tomorrowAt(h.clock.Now(), 10)
	ok := h.schedule(session, "schedule", at, "Sale", good)[0]
	rejected := h.schedule(session, "schedule", at, "No sale", bad)[0]
	disabled := h.schedule(session, "schedule", at, "Apagado", off)[0]
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+off+"/disabled", map[string]bool{"disabled": true}, session), http.StatusNoContent)

	h.clock.Set(at)
	h.mustPublishValue(ok, at, 0)
	h.bot.FailNext(http.StatusBadRequest, "Bad Request: chat not found")
	h.mustPublishValue(rejected, at, 0)
	h.mustPublishValue(disabled, at, 0)

	items, unread := h.notifications(session)
	if unread != 3 || len(items) != 3 {
		t.Fatalf("notifications = %+v, unread %d", items, unread)
	}
	byTemplate := map[string]notificationView{}
	for _, n := range items {
		byTemplate[n.Template] = n
	}
	if n := byTemplate["published"]; n.Kind != "success" || n.Params["url"] != h.postState(ok).ReleaseURL || n.Params["channel"] != "Demo" {
		t.Fatalf("published = %+v", n)
	}
	if n := byTemplate["failed"]; n.Kind != "failure" || n.Params["reason"] != "Bad Request: chat not found" || n.Params["channel"] != "Canal A" {
		t.Fatalf("failed = %+v", n)
	}
	if n := byTemplate["channel_disabled"]; n.Kind != "info" || n.Params["channel"] != "Canal B" {
		t.Fatalf("channel_disabled = %+v", n)
	}
}

// S05.18 La campana cuenta las no leídas de cada miembro.
func TestTheBellCountsWhatEachMemberHasNotRead(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	anaSession := h.member(ana)
	org := h.activeOrganization(anaSession)
	brunoSession := h.joinAs(bruno, org)
	for i := range 12 {
		h.insertNotification(org, h.clock.Now().Add(time.Duration(i-12)*time.Minute))
	}

	expectStatus(t, h.do(http.MethodPost, "/api/v1/notifications/read", nil, anaSession), http.StatusNoContent)
	h.clock.Advance(time.Minute)
	h.insertNotification(org, h.clock.Now())

	items, unread := h.notifications(anaSession)
	if len(items) != 10 || unread != 1 || !items[0].Unread || items[1].Unread {
		t.Fatalf("Ana sees %d items, %d unread: %+v", len(items), unread, items)
	}
	if _, unread := h.notifications(brunoSession); unread != 13 {
		t.Fatalf("Bruno has %d unread, want 13", unread)
	}
}

// S05.19 El correo de fallo sale al momento, a quien lo quiere.
func TestFailureEmailsGoOutAtOnceToWhoeverWantsThem(t *testing.T) {
	var resend *fakeresend.Server
	h := newHarness(t, withOIDC("Fake"), withTelegram(), withResend(&resend))
	session := h.member(ana)
	org := h.activeOrganization(session)
	h.joinAs(bruno, org)
	h.setEmailPreferences(bruno.Subject, true, false)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.schedule(session, "schedule", at, "No sale", channel)[0]

	h.bot.FailNext(http.StatusBadRequest, "Bad Request: chat not found")
	h.clock.Set(at)
	h.mustPublishValue(id, at, 0)

	emails := resend.To(ana.Email)
	if len(emails) != 1 || !strings.Contains(emails[0].HTML, "Bad Request: chat not found") || !strings.Contains(emails[0].HTML, h.server.URL+"/settings") {
		t.Fatalf("emails to Ana = %+v", emails)
	}
	if emails[0].From != "postik <postik@example.com>" || emails[0].Subject == "" {
		t.Fatalf("email = %+v", emails[0])
	}
	if got := resend.To(bruno.Email); len(got) != 0 {
		t.Fatalf("Bruno got %+v", got)
	}

	quiet := newHarness(t, withOIDC("Fake"), withTelegram())
	quietSession := quiet.member(ana)
	quietChannel := quiet.connect(quietSession, canalA)
	failing := quiet.schedule(quietSession, "schedule", at, "No sale", quietChannel)[0]
	quiet.bot.FailNext(http.StatusBadRequest, "Bad Request: chat not found")
	quiet.clock.Set(at)
	quiet.mustPublishValue(failing, at, 0)
	if items, _ := quiet.notifications(quietSession); len(items) != 1 || items[0].Template != "failed" {
		t.Fatalf("without Resend the notification is still created: %+v", items)
	}
}

// S05.20 Los avisos informativos llegan a todos.
func TestInformationalNoticesReachEveryone(t *testing.T) {
	var resend *fakeresend.Server
	h := newHarness(t, withOIDC("Fake"), withTelegram(), withResend(&resend))
	session := h.member(ana)
	org := h.activeOrganization(session)
	h.joinAs(bruno, org)
	h.setEmailPreferences(ana.Subject, false, false)
	h.setEmailPreferences(bruno.Subject, false, false)
	channel := h.connect(session, canalA)
	at := tomorrowAt(h.clock.Now(), 10)
	id := h.schedule(session, "schedule", at, "Apagado", channel)[0]
	expectStatus(t, h.do(http.MethodPut, "/api/v1/channels/"+channel+"/disabled", map[string]bool{"disabled": true}, session), http.StatusNoContent)

	h.clock.Set(at)
	h.mustPublishValue(id, at, 0)

	for _, who := range []fakeoidc.Identity{ana, bruno} {
		if got := resend.To(who.Email); len(got) != 1 || !strings.Contains(got[0].HTML, "Canal A") {
			t.Fatalf("emails to %s = %+v", who.Name, got)
		}
	}
}

// S05.21 Los éxitos llegan en un resumen por hora, sin repetirse.
func TestSuccessesArriveInAnHourlyDigestOnce(t *testing.T) {
	var resend *fakeresend.Server
	h := newHarness(t, withOIDC("Fake"), withTelegram(), withResend(&resend))
	session := h.member(ana)
	org := h.activeOrganization(session)
	h.joinAs(bruno, org)
	h.setEmailPreferences(bruno.Subject, false, true)
	channel := h.connect(session, canalPublico)
	at := tomorrowAt(h.clock.Now(), 10)
	var ids []string
	for _, text := range []string{"Uno", "Dos", "Tres"} {
		ids = append(ids, h.schedule(session, "schedule", at, text, channel)[0])
	}
	h.clock.Set(at)
	for _, id := range ids {
		h.mustPublishValue(id, at, 0)
	}
	if got := resend.Emails(); len(got) != 0 {
		t.Fatalf("successes were mailed at once: %+v", got)
	}

	h.clock.Advance(30 * time.Minute)
	h.digest()
	h.digest()

	emails := resend.To(ana.Email)
	if len(emails) != 1 {
		t.Fatalf("digests to Ana = %d, want 1", len(emails))
	}
	for _, id := range ids {
		if link := h.postState(id).ReleaseURL; !strings.Contains(emails[0].HTML, link) {
			t.Fatalf("the digest misses %s: %s", link, emails[0].HTML)
		}
	}
	if got := resend.To(bruno.Email); len(got) != 0 {
		t.Fatalf("Bruno got %+v", got)
	}
}

// S05.22 Las preferencias se guardan por persona.
func TestEmailPreferencesArePerPerson(t *testing.T) {
	h := newHarness(t, withOIDC("Fake"))
	anaSession := h.member(ana)
	brunoSession := h.joinAs(bruno, h.activeOrganization(anaSession))

	preferences := func(session requestOption) (bool, bool) {
		user := h.do(http.MethodGet, "/api/v1/me", nil, session).json(t)["user"].(map[string]any)
		return user["emailSuccess"].(bool), user["emailFailure"].(bool)
	}
	if success, failure := preferences(anaSession); !success || !failure {
		t.Fatalf("defaults = %v, %v", success, failure)
	}
	expectStatus(t, h.do(http.MethodPut, "/api/v1/me/preferences", map[string]bool{"emailSuccess": false, "emailFailure": true}, anaSession), http.StatusNoContent)

	if success, failure := preferences(anaSession); success || !failure {
		t.Fatalf("Ana after the change = %v, %v", success, failure)
	}
	if success, failure := preferences(brunoSession); !success || !failure {
		t.Fatalf("Bruno = %v, %v", success, failure)
	}
}
