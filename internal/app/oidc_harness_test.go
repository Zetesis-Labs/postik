package app_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/testsupport/fakeoidc"
)

// withOIDC starts a fake provider that shares the harness clock.
func withOIDC(displayName string) harnessOption {
	return func(c *config.Config) {
		c.OIDC = &config.OIDC{DisplayName: displayName, ClientID: fakeoidc.ClientID, ClientSecret: fakeoidc.ClientSecret}
	}
}

func requireInvitation() harnessOption {
	return func(c *config.Config) { c.RequireInvitation = true }
}

// browser follows redirects between postik's API and the provider, and stops
// when postik sends it to a page of the SPA.
type browser struct {
	h      *harness
	client *http.Client
}

func (h *harness) newBrowser() *browser {
	jar, _ := cookiejar.New(nil)
	app, _ := url.Parse(h.server.URL)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if req.URL.Host == app.Host && !strings.HasPrefix(req.URL.Path, "/api/") && !strings.HasPrefix(req.URL.Path, "/integrations/social/") {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return &browser{h: h, client: client}
}

// get returns the status and, for redirects, the Location relative to postik.
func (b *browser) get(target string) (int, string) {
	b.h.t.Helper()
	res, err := b.client.Get(target)
	if err != nil {
		b.h.t.Fatalf("GET %s: %v", target, err)
	}
	defer res.Body.Close()
	return res.StatusCode, strings.TrimPrefix(res.Header.Get("Location"), b.h.server.URL)
}

// signIn runs the whole authorization code flow as identity and returns
// where postik sends the browser at the end.
func (b *browser) signIn(identity fakeoidc.Identity) string {
	b.h.t.Helper()
	b.h.oidc.SignInNext(identity)
	status, location := b.get(b.h.server.URL + "/api/v1/auth/oidc/login")
	if status != http.StatusFound {
		b.h.t.Fatalf("the flow ended with status %d", status)
	}
	return location
}

func (b *browser) sessionCookie() *http.Cookie {
	app, _ := url.Parse(b.h.server.URL)
	for _, c := range b.client.Jar.Cookies(app) {
		if c.Name == "postik_session" {
			return c
		}
	}
	return nil
}

// startOIDC is called by newHarness when the configuration asks for OIDC.
func startOIDC(h *harness, cfg *config.Config) {
	var server *httptest.Server
	provider, err := fakeoidc.New("", h.clock.Now)
	if err != nil {
		h.t.Fatalf("fake provider: %v", err)
	}
	server = httptest.NewServer(provider.Handler())
	h.t.Cleanup(server.Close)
	provider.Issuer = server.URL
	cfg.OIDC.Issuer = server.URL
	h.oidc = provider
}
