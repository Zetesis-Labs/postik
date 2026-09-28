package app_test

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zetesis-labs/postik/internal/core/access"
	"github.com/zetesis-labs/postik/internal/database/migrations"
	"github.com/zetesis-labs/postik/internal/testsupport"
)

// S01.1 El proceso informa de su salud y de la de la base de datos.
func TestHealthAndReadiness(t *testing.T) {
	h := newHarness(t)

	expectStatus(t, h.do(http.MethodGet, "/healthz", nil), http.StatusOK)
	expectStatus(t, h.do(http.MethodGet, "/readyz", nil), http.StatusOK)

	if err := h.db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
	expectStatus(t, h.do(http.MethodGet, "/readyz", nil), http.StatusServiceUnavailable)
	expectStatus(t, h.do(http.MethodGet, "/healthz", nil), http.StatusOK)
}

// S01.2 Las migraciones dejan el esquema al día y se pueden repetir.
func TestMigrationsAreRepeatable(t *testing.T) {
	db := testsupport.NewEmptyDB(t)
	ctx := context.Background()

	if err := migrations.Run(ctx, db.DB); err != nil {
		t.Fatalf("first run: %v", err)
	}
	var appliedAfterFirst int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&appliedAfterFirst); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if appliedAfterFirst == 0 {
		t.Fatal("the first run applied nothing")
	}

	if err := migrations.Run(ctx, db.DB); err != nil {
		t.Fatalf("second run: %v", err)
	}
	var appliedAfterSecond int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&appliedAfterSecond); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if appliedAfterSecond != appliedAfterFirst {
		t.Fatalf("the second run changed the schema: %d → %d migrations", appliedAfterFirst, appliedAfterSecond)
	}
	var sessionsTable bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.sessions') IS NOT NULL").Scan(&sessionsTable); err != nil || !sessionsTable {
		t.Fatalf("the sessions table is missing (err %v)", err)
	}
}

// S01.4 El superadmin sin 2FA entra con usuario y contraseña.
func TestSuperadminWithoutTOTPLogsIn(t *testing.T) {
	h := newHarness(t)

	res := h.loginSuperadmin("")
	expectStatus(t, res, http.StatusOK)
	cookie := res.sessionCookie()
	if cookie == nil {
		t.Fatal("the login did not set postik_session")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie must be HttpOnly and SameSite=Lax: %+v", cookie)
	}
	if cookie.Secure {
		t.Error("an http public URL must not mark the cookie as secure")
	}

	me := h.do(http.MethodGet, "/api/v1/me", nil, withCookie(cookie))
	expectStatus(t, me, http.StatusOK)
	if kind := me.json(t)["kind"]; kind != "superadmin" {
		t.Fatalf("kind = %v, want superadmin", kind)
	}
}

// S01.5 Con 2FA hace falta un código TOTP vigente.
func TestSuperadminNeedsACurrentTOTPCode(t *testing.T) {
	h := newHarness(t, withTOTP())
	now := h.clock.Now()

	for _, offset := range []time.Duration{0, -30 * time.Second, 30 * time.Second} {
		code := access.TOTPCode(testTOTPSecret, now.Add(offset))
		expectStatus(t, h.loginSuperadmin(code), http.StatusOK)
	}
	stale := access.TOTPCode(testTOTPSecret, now.Add(-60*time.Second))
	expectStatus(t, h.loginSuperadmin(stale), http.StatusUnauthorized)
	expectStatus(t, h.loginSuperadmin(""), http.StatusUnauthorized)
}

// S01.6 Un código de recuperación sustituye al TOTP y no se gasta.
func TestRecoveryCodeReplacesTOTPAndIsNotConsumed(t *testing.T) {
	h := newHarness(t, withTOTP("alfa-1234", "beta-5678"))

	expectStatus(t, h.loginSuperadmin("alfa-1234"), http.StatusOK)
	expectStatus(t, h.loginSuperadmin("alfa-1234"), http.StatusOK)
	expectStatus(t, h.loginSuperadmin("gamma-0000"), http.StatusUnauthorized)
}

// S01.7 Cualquier dato incorrecto da el mismo error, sin decir cuál.
func TestWrongSuperadminDataGivesTheSameError(t *testing.T) {
	h := newHarness(t, withTOTP())
	code := access.TOTPCode(testTOTPSecret, h.clock.Now())
	attempts := []map[string]string{
		{"username": "root", "password": h.config.Superadmin.Password, "code": code},
		{"username": h.config.Superadmin.Username, "password": "wrong", "code": code},
		{"username": h.config.Superadmin.Username, "password": h.config.Superadmin.Password, "code": "000000"},
	}

	var first []byte
	for i, attempt := range attempts {
		res := h.do(http.MethodPost, "/api/v1/auth/superadmin", attempt)
		expectStatus(t, res, http.StatusUnauthorized)
		if res.sessionCookie() != nil {
			t.Errorf("attempt %d set a session cookie", i)
		}
		if res.json(t)["code"] != "invalid_credentials" {
			t.Errorf("attempt %d: body %s", i, res.body)
		}
		if first == nil {
			first = res.body
		} else if !bytes.Equal(first, res.body) {
			t.Errorf("attempt %d answered %s, attempt 0 answered %s", i, res.body, first)
		}
	}
}

// S01.8 La sesión del superadmin caduca a las 12 horas aunque se use.
func TestSuperadminSessionExpiresAfterTwelveHours(t *testing.T) {
	h := newHarness(t)
	start := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	h.clock.Set(start)
	cookie := h.mustLoginSuperadmin("")

	for hour := 1; hour <= 11; hour++ {
		h.clock.Set(start.Add(time.Duration(hour) * time.Hour))
		expectStatus(t, h.do(http.MethodGet, "/api/v1/me", nil, withCookie(cookie)), http.StatusOK)
	}
	h.clock.Set(start.Add(11*time.Hour + 59*time.Minute))
	expectStatus(t, h.do(http.MethodGet, "/api/v1/me", nil, withCookie(cookie)), http.StatusOK)
	h.clock.Set(start.Add(12 * time.Hour))
	expectStatus(t, h.do(http.MethodGet, "/api/v1/me", nil, withCookie(cookie)), http.StatusUnauthorized)
}

// S01.9 Cerrar sesión la invalida en el servidor.
func TestLogoutInvalidatesTheSession(t *testing.T) {
	h := newHarness(t)
	cookie := h.mustLoginSuperadmin("")

	res := h.do(http.MethodPost, "/api/v1/auth/logout", nil, withCookie(cookie))
	expectStatus(t, res, http.StatusNoContent)
	cleared := res.sessionCookie()
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Errorf("logout must expire the cookie, got %+v", cleared)
	}
	expectStatus(t, h.do(http.MethodGet, "/api/v1/me", nil, withCookie(cookie)), http.StatusUnauthorized)
}

// S01.10 Sin sesión, la API responde 401.
func TestAPIRequiresASession(t *testing.T) {
	h := newHarness(t)

	anonymous := h.do(http.MethodGet, "/api/v1/me", nil)
	expectStatus(t, anonymous, http.StatusUnauthorized)
	if anonymous.json(t)["code"] != "unauthenticated" {
		t.Errorf("body %s", anonymous.body)
	}
	unknown := h.do(http.MethodGet, "/api/v1/me", nil, withCookie(&http.Cookie{Name: "postik_session", Value: "bm90LWEtc2Vzc2lvbg"}))
	expectStatus(t, unknown, http.StatusUnauthorized)
}

// S01.11 Una petición que cambia estado desde otro origen se rechaza.
func TestCrossOriginMutationIsRejected(t *testing.T) {
	h := newHarness(t)
	cookie := h.mustLoginSuperadmin("")

	res := h.do(http.MethodPost, "/api/v1/auth/logout", nil, withCookie(cookie), withHeader("Origin", "https://otro.example"))
	expectStatus(t, res, http.StatusForbidden)
	expectStatus(t, h.do(http.MethodGet, "/api/v1/me", nil, withCookie(cookie)), http.StatusOK)
}

// S01.12 El binario sirve la SPA y la API por separado.
func TestBinaryServesSPAAndAPISeparately(t *testing.T) {
	h := newHarness(t)

	for _, path := range []string{"/", "/launches", "/auth/login"} {
		res := h.do(http.MethodGet, path, nil)
		expectStatus(t, res, http.StatusOK)
		if !strings.Contains(string(res.body), "postik-spa") {
			t.Errorf("%s did not return the SPA: %s", path, res.body)
		}
	}
	asset := h.do(http.MethodGet, "/assets/app.js", nil)
	expectStatus(t, asset, http.StatusOK)
	if !strings.Contains(string(asset.body), "console.log") {
		t.Errorf("asset body %s", asset.body)
	}

	missing := h.do(http.MethodGet, "/api/v1/no-existe", nil)
	expectStatus(t, missing, http.StatusNotFound)
	if missing.json(t)["code"] != "not_found" {
		t.Errorf("body %s", missing.body)
	}
}
