// Package oauth holds the pure rules of connecting channels with OAuth and of
// keeping their tokens alive (S06 §3 and §4).
package oauth

import (
	"math"
	"slices"
	"strings"
	"time"
)

// AuthorizationTTL is how long a started authorization can come back (F5).
const AuthorizationTTL = time.Hour

// Callback errors, sent to the screen in oauth_error.
const (
	ErrDenied             = "denied"
	ErrInvalidState       = "invalid_state"
	ErrExpired            = "expired"
	ErrExchangeFailed     = "exchange_failed"
	ErrMissingPermissions = "missing_permissions"
	ErrWrongAccount       = "wrong_account"
)

func Expired(createdAt, now time.Time) bool {
	return !now.Before(createdAt.Add(AuthorizationTTL))
}

// MissingScopes lists the required scopes that the network did not grant.
// LinkedIn separates the granted ones with commas; others use spaces.
func MissingScopes(granted string, required []string) []string {
	have := strings.FieldsFunc(granted, func(r rune) bool { return r == ',' || r == ' ' })
	var missing []string
	for _, scope := range required {
		if !slices.Contains(have, scope) {
			missing = append(missing, scope)
		}
	}
	return missing
}

// RenewalMargin renews a token that is about to expire before using it.
const RenewalMargin = 5 * time.Minute

// NeedsRenewal reports whether a token should be renewed before a call.
func NeedsRenewal(expiresAt *time.Time, hasRefresh bool, now time.Time) bool {
	return hasRefresh && expiresAt != nil && !now.Add(RenewalMargin).Before(*expiresAt)
}

// ExpiresAt turns a lifetime in seconds into a date; zero means it does not expire.
func ExpiresAt(now time.Time, seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	at := now.Add(time.Duration(seconds) * time.Second)
	return &at
}

// ExpiryNotice is how long before its token expires a channel is warned about.
const ExpiryNotice = 7 * 24 * time.Hour

// Token is what token_expiry knows about a channel.
type Token struct {
	ExpiresAt     *time.Time
	HasRefresh    bool
	WarnedFor     *time.Time
	RefreshNeeded bool
}

type ExpiryAction int

const (
	NoExpiryAction ExpiryAction = iota
	WarnExpiry
	MarkExpired
)

// Expiry decides what token_expiry does with a channel: warn once, 7 days
// before, about a token that cannot be renewed, and ask for reconnection once
// it has expired. It returns the whole days left for the warning.
func Expiry(t Token, now time.Time) (ExpiryAction, int) {
	if t.HasRefresh || t.ExpiresAt == nil || t.RefreshNeeded {
		return NoExpiryAction, 0
	}
	left := t.ExpiresAt.Sub(now)
	switch {
	case left <= 0:
		return MarkExpired, 0
	case left <= ExpiryNotice && (t.WarnedFor == nil || !t.WarnedFor.Equal(*t.ExpiresAt)):
		return WarnExpiry, int(math.Ceil(left.Hours() / 24))
	}
	return NoExpiryAction, 0
}

// PendingPage is the external ID of a LinkedIn Page channel whose page is
// still to be chosen, one per member.
func PendingPage(sub string) string {
	return "pending:" + sub
}

// Role is a member's role in a LinkedIn organization.
type Role struct {
	Organization string
	Role         string
	State        string
}

const organizationPrefix = "urn:li:organization:"

// AdministeredPages lists the IDs of the pages a member can publish as.
func AdministeredPages(roles []Role) []string {
	var ids []string
	for _, r := range roles {
		id, ok := strings.CutPrefix(r.Organization, organizationPrefix)
		if !ok || r.State != "APPROVED" || (r.Role != "ADMINISTRATOR" && r.Role != "CONTENT_ADMINISTRATOR") || slices.Contains(ids, id) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}
