package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/uptrace/bun"

	"github.com/zetesis-labs/postik/internal/app"
	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/core/access"
	"github.com/zetesis-labs/postik/internal/jobs"
	"github.com/zetesis-labs/postik/internal/testsupport"
	"github.com/zetesis-labs/postik/internal/testsupport/fakelinkedin"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeoidc"
	"github.com/zetesis-labs/postik/internal/testsupport/faketelegram"
)

var testTOTPSecret = []byte("postik-test-totp-secret!")

type harness struct {
	t        *testing.T
	server   *httptest.Server
	handler  http.Handler
	clock    *testsupport.Clock
	db       *bun.DB
	config   config.Config
	oidc     *fakeoidc.Provider
	bot      *faketelegram.Bot
	botAPI   *httptest.Server
	linkedIn *fakelinkedin.Server
	jobs     *jobs.Jobs
}

type harnessOption func(*config.Config)

func withTOTP(recoveryCodes ...string) harnessOption {
	return func(c *config.Config) {
		c.Superadmin.TOTPSecret = testTOTPSecret
		c.Superadmin.RecoveryCodes = recoveryCodes
	}
}

func testWebUI() fs.FS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>postik-spa</title>")},
		"assets/app.js": {Data: []byte("console.log('postik')")},
	}
}

func newHarness(t *testing.T, options ...harnessOption) *harness {
	t.Helper()
	h := &harness{
		t:     t,
		clock: testsupport.NewClock(time.Date(2026, 9, 28, 10, 0, 15, 0, time.UTC)),
		db:    testsupport.NewMigratedDB(t),
	}
	h.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.handler.ServeHTTP(w, r)
	}))
	t.Cleanup(h.server.Close)
	publicURL, _ := url.Parse(h.server.URL)
	cfg := config.Config{
		DatabaseURL: "unused",
		PublicURL:   publicURL,
		ListenAddr:  ":0",
		Superadmin: access.Superadmin{
			Username: "admin",
			Password: "correct horse battery staple",
		},
		StorageDir: t.TempDir(),
	}
	for _, option := range options {
		option(&cfg)
	}
	if cfg.OIDC != nil {
		startOIDC(h, &cfg)
	}
	if cfg.Telegram != nil {
		startTelegram(h, &cfg)
	}
	if cfg.LinkedIn != nil {
		startLinkedIn(h, &cfg)
	}
	h.config = cfg
	a, err := app.New(app.Deps{
		Config: cfg,
		DB:     h.db,
		Now:    h.clock.Now,
		WebUI:  testWebUI(),
		Logger: slog.New(slog.NewTextHandler(testLog{t}, &slog.HandlerOptions{Level: slog.LevelWarn})),
	})
	if err != nil {
		t.Fatalf("new app: %v", err)
	}
	h.handler, h.jobs = a.Handler, a.Jobs
	return h
}

// testLog sends the application's warnings and errors to the test output.
type testLog struct{ t *testing.T }

func (l testLog) Write(p []byte) (int, error) {
	l.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}

type response struct {
	status  int
	header  http.Header
	body    []byte
	cookies []*http.Cookie
}

func decodeJSON(t *testing.T, body []byte, into any) {
	t.Helper()
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
}

func (r response) json(t *testing.T) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(r.body, &decoded); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
	return decoded
}

func (r response) sessionCookie() *http.Cookie {
	for _, c := range r.cookies {
		if c.Name == "postik_session" {
			return c
		}
	}
	return nil
}

type requestOption func(*http.Request)

func withCookie(c *http.Cookie) requestOption {
	return func(r *http.Request) {
		if c != nil {
			r.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
		}
	}
}

func withHeader(key, value string) requestOption {
	return func(r *http.Request) { r.Header.Set(key, value) }
}

func (h *harness) do(method, path string, body any, options ...requestOption) response {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			h.t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, option := range options {
		option(req)
	}
	res, err := h.server.Client().Do(req)
	if err != nil {
		h.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		h.t.Fatalf("read body: %v", err)
	}
	return response{status: res.StatusCode, header: res.Header, body: data, cookies: res.Cookies()}
}

func (h *harness) loginSuperadmin(code string) response {
	h.t.Helper()
	body := map[string]string{
		"username": h.config.Superadmin.Username,
		"password": h.config.Superadmin.Password,
	}
	if code != "" {
		body["code"] = code
	}
	return h.do(http.MethodPost, "/api/v1/auth/superadmin", body)
}

func (h *harness) mustLoginSuperadmin(code string) *http.Cookie {
	h.t.Helper()
	res := h.loginSuperadmin(code)
	if res.status != http.StatusOK || res.sessionCookie() == nil {
		h.t.Fatalf("superadmin login: status %d, body %s", res.status, res.body)
	}
	return res.sessionCookie()
}

func expectStatus(t *testing.T, res response, want int) {
	t.Helper()
	if res.status != want {
		t.Fatalf("status = %d, want %d; body %s", res.status, want, strings.TrimSpace(string(res.body)))
	}
}
