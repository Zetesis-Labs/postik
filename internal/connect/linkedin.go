package connect

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/zetesis-labs/postik/internal/core/oauth"
	"github.com/zetesis-labs/postik/internal/linkedin"
	"github.com/zetesis-labs/postik/internal/tokens"
)

// LinkedIn connects the profile of the member who authorizes.
type LinkedIn struct {
	Client *linkedin.Client
	Now    func() time.Time
}

func (l *LinkedIn) scopes() []string { return linkedin.MemberScopes }

func (l *LinkedIn) Begin(_ context.Context, redirect string) (Begin, error) {
	state, err := randomState()
	if err != nil {
		return Begin{}, err
	}
	return Begin{URL: l.Client.AuthorizeURL(state, redirect, l.scopes()), State: state}, nil
}

func (l *LinkedIn) State(q url.Values) string { return q.Get("state") }

func (l *LinkedIn) Denied(q url.Values) bool { return q.Get("error") != "" }

func (l *LinkedIn) Finish(ctx context.Context, q url.Values, _ []byte, redirect string) (Account, error) {
	return l.finish(ctx, q, redirect, l.scopes())
}

func (l *LinkedIn) finish(ctx context.Context, q url.Values, redirect string, scopes []string) (Account, error) {
	token, err := l.Client.Exchange(ctx, q.Get("code"), redirect)
	if err != nil {
		return Account{}, err
	}
	if missing := oauth.MissingScopes(token.Scope, scopes); len(missing) > 0 {
		return Account{}, fmt.Errorf("%w: %v", ErrMissingPermissions, missing)
	}
	info, err := l.Client.UserInfo(ctx, token.AccessToken)
	if err != nil {
		return Account{}, err
	}
	return Account{ExternalID: info.Sub, Name: info.Name, PictureURL: info.Picture, Tokens: l.tokens(token)}, nil
}

func (l *LinkedIn) tokens(t linkedin.Token) tokens.Tokens {
	now := l.Now()
	return tokens.Tokens{
		Access:           t.AccessToken,
		Refresh:          t.RefreshToken,
		ExpiresAt:        oauth.ExpiresAt(now, t.ExpiresIn),
		RefreshExpiresAt: oauth.ExpiresAt(now, t.RefreshTokenExpiresIn),
	}
}

func (l *LinkedIn) Picture(ctx context.Context, target string) ([]byte, error) {
	return l.Client.Download(ctx, target)
}

// Renew asks LinkedIn for a new access token. A refused refresh token means
// the channel has to be reconnected.
func (l *LinkedIn) Renew(ctx context.Context, refresh string) (tokens.Tokens, error) {
	t, err := l.Client.Refresh(ctx, refresh)
	var apiErr *linkedin.APIError
	if errors.As(err, &apiErr) && (apiErr.Status == http.StatusBadRequest || apiErr.Status == http.StatusUnauthorized) {
		return tokens.Tokens{}, fmt.Errorf("%w: %s", tokens.ErrCannotRenew, apiErr.Message)
	}
	if err != nil {
		return tokens.Tokens{}, err
	}
	return l.tokens(t), nil
}

// LinkedInPage connects a page the member administers, chosen after
// authorizing (F5, step 5).
type LinkedInPage struct {
	LinkedIn
}

func (l *LinkedInPage) Begin(_ context.Context, redirect string) (Begin, error) {
	state, err := randomState()
	if err != nil {
		return Begin{}, err
	}
	return Begin{URL: l.Client.AuthorizeURL(state, redirect, linkedin.PageScopes), State: state}, nil
}

func (l *LinkedInPage) Finish(ctx context.Context, q url.Values, _ []byte, redirect string) (Account, error) {
	account, err := l.finish(ctx, q, redirect, linkedin.PageScopes)
	account.Pending = true
	return account, err
}

func (l *LinkedInPage) Administered(ctx context.Context, accessToken string) ([]string, error) {
	acls, err := l.Client.ACLs(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	roles := make([]oauth.Role, len(acls))
	for i, a := range acls {
		roles[i] = oauth.Role{Organization: a.Organization, Role: a.Role, State: a.State}
	}
	return oauth.AdministeredPages(roles), nil
}

func (l *LinkedInPage) Page(ctx context.Context, accessToken, id string) (Page, error) {
	org, err := l.Client.Organization(ctx, accessToken, id)
	if err != nil {
		return Page{}, err
	}
	return Page{ID: id, Name: org.Name, Username: org.VanityName, PictureURL: org.LogoURL}, nil
}

func randomState() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
