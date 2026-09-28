package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/zetesis-labs/postik/internal/config"
	"github.com/zetesis-labs/postik/internal/core/access"
	"github.com/zetesis-labs/postik/internal/core/identity"
	"github.com/zetesis-labs/postik/internal/postgres"
)

const (
	oidcStateCookie = "postik_oidc_state"
	oidcCookiePath  = "/api/v1/auth/oidc"
	oidcLoginTTL    = 10 * time.Minute
	CallbackPath    = "/api/v1/auth/oidc/callback"
	landingPath     = "/launches"
)

type OIDCLoginStore interface {
	Create(ctx context.Context, login postgres.OIDCLogin, staleBefore time.Time) error
	Take(ctx context.Context, stateHash []byte) (*postgres.OIDCLogin, error)
}

type IdentityStore interface {
	FindUser(ctx context.Context, issuer, subject string) (*postgres.User, error)
	CreateUserWithOrganization(ctx context.Context, user postgres.User, org postgres.Organization) error
}

type OIDC struct {
	Provider          config.OIDC
	RedirectURL       string
	RequireInvitation bool
	SecureCookies     bool
	Now               func() time.Time
	Logins            OIDCLoginStore
	Identity          IdentityStore
	Sessions          *Sessions
	Logger            *slog.Logger

	mu       sync.Mutex
	provider *oidc.Provider
}

// discover loads the provider on first use, so postik starts even while the
// provider is down.
func (o *OIDC) discover(ctx context.Context) (*oidc.Provider, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.provider != nil {
		return o.provider, nil
	}
	client := &http.Client{Timeout: 10 * time.Second}
	provider, err := oidc.NewProvider(oidc.ClientContext(context.WithoutCancel(ctx), client), o.Provider.Issuer)
	if err != nil {
		return nil, err
	}
	o.provider = provider
	return provider, nil
}

func (o *OIDC) oauth(provider *oidc.Provider) oauth2.Config {
	return oauth2.Config{
		ClientID:     o.Provider.ClientID,
		ClientSecret: o.Provider.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  o.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
}

func (o *OIDC) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	provider, err := o.discover(ctx)
	if err != nil {
		o.fail(w, r, "oidc_unavailable", err)
		return
	}
	state, nonce := randomToken(), randomToken()
	verifier := oauth2.GenerateVerifier()
	now := o.Now()
	login := postgres.OIDCLogin{StateHash: hashToken(state), Nonce: nonce, CodeVerifier: verifier, CreatedAt: now}
	if err := o.Logins.Create(ctx, login, now.Add(-oidcLoginTTL)); err != nil {
		o.fail(w, r, "oidc_unavailable", err)
		return
	}
	http.SetCookie(w, o.stateCookie(state, int(oidcLoginTTL.Seconds())))
	cfg := o.oauth(provider)
	target := cfg.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, target, http.StatusFound)
}

func (o *OIDC) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	http.SetCookie(w, o.stateCookie("", -1))

	state := query.Get("state")
	cookie, err := r.Cookie(oidcStateCookie)
	if err != nil || state == "" || cookie.Value != state {
		o.fail(w, r, "oidc_state", errors.New("the state does not belong to this browser"))
		return
	}
	login, err := o.Logins.Take(ctx, hashToken(state))
	if err != nil {
		o.fail(w, r, "oidc_unavailable", err)
		return
	}
	if login == nil || o.Now().Sub(login.CreatedAt) > oidcLoginTTL {
		o.fail(w, r, "oidc_state", errors.New("unknown or expired login attempt"))
		return
	}
	if query.Get("error") != "" {
		o.fail(w, r, "oidc_denied", fmt.Errorf("provider answered %s", query.Get("error")))
		return
	}

	claims, err := o.exchange(ctx, query.Get("code"), login)
	if err != nil {
		o.fail(w, r, "oidc_unavailable", err)
		return
	}
	userID, outcome, err := o.admit(ctx, claims)
	if err != nil {
		o.fail(w, r, "oidc_unavailable", err)
		return
	}
	if outcome == identity.DenyEmailRequired || outcome == identity.DenyInvitationRequired {
		o.fail(w, r, outcome.String(), nil)
		return
	}
	session, err := o.Sessions.Open(ctx, access.SessionMember, &userID)
	if err != nil {
		o.fail(w, r, "oidc_unavailable", err)
		return
	}
	w.Header().Add("Set-Cookie", session)
	http.Redirect(w, r, landingPath, http.StatusFound)
}

type verifiedClaims struct {
	issuer string
	identity.Claims
}

func (o *OIDC) exchange(ctx context.Context, code string, login *postgres.OIDCLogin) (verifiedClaims, error) {
	provider, err := o.discover(ctx)
	if err != nil {
		return verifiedClaims{}, err
	}
	cfg := o.oauth(provider)
	token, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(login.CodeVerifier))
	if err != nil {
		return verifiedClaims{}, fmt.Errorf("exchange code: %w", err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return verifiedClaims{}, errors.New("the token response has no id_token")
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: o.Provider.ClientID, Now: o.Now}).Verify(ctx, rawIDToken)
	if err != nil {
		return verifiedClaims{}, fmt.Errorf("verify id_token: %w", err)
	}
	if idToken.Nonce != login.Nonce {
		return verifiedClaims{}, errors.New("the id_token nonce does not match")
	}
	var extra struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&extra); err != nil {
		return verifiedClaims{}, fmt.Errorf("read claims: %w", err)
	}
	return verifiedClaims{
		issuer: idToken.Issuer,
		Claims: identity.Claims{
			Subject:           idToken.Subject,
			Email:             extra.Email,
			Name:              extra.Name,
			PreferredUsername: extra.PreferredUsername,
		},
	}, nil
}

// admit decides with the core whether the person enters and creates them when needed.
func (o *OIDC) admit(ctx context.Context, claims verifiedClaims) (uuid.UUID, identity.AccessOutcome, error) {
	existing, err := o.Identity.FindUser(ctx, claims.issuer, claims.Subject)
	if err != nil {
		return uuid.Nil, 0, err
	}
	outcome := identity.DecideAccess(identity.AccessInput{
		Existing:          existing != nil,
		Claims:            claims.Claims,
		RequireInvitation: o.RequireInvitation,
	})
	switch outcome {
	case identity.Enter:
		return existing.ID, outcome, nil
	case identity.CreateWithOrganization:
		now := o.Now()
		name := identity.DisplayName(claims.Claims)
		user := postgres.User{ID: uuid.New(), Issuer: claims.issuer, Subject: claims.Subject, Email: claims.Email, Name: name, CreatedAt: now}
		org := postgres.Organization{ID: uuid.New(), Name: name, CreatedAt: now}
		if err := o.Identity.CreateUserWithOrganization(ctx, user, org); err != nil {
			return uuid.Nil, 0, err
		}
		return user.ID, outcome, nil
	default:
		return uuid.Nil, outcome, nil
	}
}

func (o *OIDC) stateCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     oidcStateCookie,
		Value:    value,
		Path:     oidcCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   o.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
}

func (o *OIDC) fail(w http.ResponseWriter, r *http.Request, code string, err error) {
	if err != nil {
		o.Logger.WarnContext(r.Context(), "oidc sign-in failed", "code", code, "error", err)
	}
	http.Redirect(w, r, "/auth/login?error="+url.QueryEscape(code), http.StatusFound)
}

func randomToken() string {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}
