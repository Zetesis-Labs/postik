package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/zetesis-labs/postik/internal/core/access"
	"github.com/zetesis-labs/postik/internal/postgres"
)

const CookieName = "postik_session"

type Principal struct {
	Kind        access.SessionKind
	UserID      *uuid.UUID
	sessionHash []byte
}

type principalKey struct{}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

type SessionStore interface {
	Create(ctx context.Context, session postgres.Session) error
	Get(ctx context.Context, idHash []byte) (*postgres.Session, error)
	Touch(ctx context.Context, idHash []byte, at time.Time) error
	Delete(ctx context.Context, idHash []byte) error
}

type Sessions struct {
	Store         SessionStore
	Now           func() time.Time
	SecureCookies bool
	Logger        *slog.Logger
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// Open creates a session and returns the Set-Cookie value that carries it.
func (s *Sessions) Open(ctx context.Context, kind access.SessionKind, userID *uuid.UUID) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	now := s.Now()
	times := access.SessionTimes{Kind: kind, CreatedAt: now, LastSeenAt: now}
	if err := s.Store.Create(ctx, postgres.Session{IDHash: hashToken(token), UserID: userID, SessionTimes: times}); err != nil {
		return "", err
	}
	cookie := s.cookie(token)
	cookie.Expires = access.CookieExpiresAt(times)
	return cookie.String(), nil
}

// Close deletes the session behind the request, if any, and returns the Set-Cookie value that clears it.
func (s *Sessions) Close(ctx context.Context) (string, error) {
	if p, ok := PrincipalFrom(ctx); ok {
		if err := s.Store.Delete(ctx, p.sessionHash); err != nil {
			return "", err
		}
	}
	cookie := s.cookie("")
	cookie.MaxAge = -1
	return cookie.String(), nil
}

func (s *Sessions) cookie(value string) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.SecureCookies,
		SameSite: http.SameSiteLaxMode,
	}
}

// Middleware attaches the Principal of a valid session to the request context.
func (s *Sessions) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if principal, ok := s.resolve(r); ok {
			r = r.WithContext(context.WithValue(r.Context(), principalKey{}, principal))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Sessions) resolve(r *http.Request) (Principal, bool) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return Principal{}, false
	}
	idHash := hashToken(cookie.Value)
	session, err := s.Store.Get(r.Context(), idHash)
	if err != nil {
		s.Logger.ErrorContext(r.Context(), "load session", "error", err)
		return Principal{}, false
	}
	if session == nil {
		return Principal{}, false
	}
	now := s.Now()
	verdict := access.EvaluateSession(session.SessionTimes, now)
	if !verdict.Valid {
		return Principal{}, false
	}
	if verdict.Touch {
		if err := s.Store.Touch(r.Context(), idHash, now); err != nil {
			s.Logger.ErrorContext(r.Context(), "touch session", "error", err)
		}
	}
	return Principal{Kind: session.Kind, UserID: session.UserID, sessionHash: idHash}, true
}
