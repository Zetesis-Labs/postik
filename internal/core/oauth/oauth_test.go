package oauth

import (
	"slices"
	"testing"
	"time"
)

var now = time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)

func at(d time.Duration) *time.Time {
	t := now.Add(d)
	return &t
}

func TestAnAuthorizationExpiresAfterAnHour(t *testing.T) {
	if Expired(now.Add(-59*time.Minute), now) || !Expired(now.Add(-time.Hour), now) {
		t.Fatal("an authorization lasts one hour")
	}
}

func TestMissingScopesAcceptsCommasAndSpaces(t *testing.T) {
	required := []string{"openid", "profile", "w_member_social"}
	if missing := MissingScopes("openid,profile,w_member_social,email", required); missing != nil {
		t.Fatalf("missing = %v", missing)
	}
	if missing := MissingScopes("openid profile", required); !slices.Equal(missing, []string{"w_member_social"}) {
		t.Fatalf("missing = %v", missing)
	}
}

func TestNeedsRenewal(t *testing.T) {
	cases := []struct {
		expires    *time.Time
		hasRefresh bool
		want       bool
	}{
		{at(time.Hour), true, false},
		{at(4 * time.Minute), true, true},
		{at(-time.Hour), true, true},
		{at(-time.Hour), false, false},
		{nil, true, false},
	}
	for _, c := range cases {
		if got := NeedsRenewal(c.expires, c.hasRefresh, now); got != c.want {
			t.Errorf("NeedsRenewal(%v, %v) = %v", c.expires, c.hasRefresh, got)
		}
	}
}

func TestExpiry(t *testing.T) {
	expires := at(6 * 24 * time.Hour)
	cases := []struct {
		name   string
		token  Token
		action ExpiryAction
		days   int
	}{
		{"far away", Token{ExpiresAt: at(30 * 24 * time.Hour)}, NoExpiryAction, 0},
		{"within a week", Token{ExpiresAt: expires}, WarnExpiry, 6},
		{"already warned", Token{ExpiresAt: expires, WarnedFor: expires}, NoExpiryAction, 0},
		{"warned for an older token", Token{ExpiresAt: expires, WarnedFor: at(-time.Hour)}, WarnExpiry, 6},
		{"renewable", Token{ExpiresAt: expires, HasRefresh: true}, NoExpiryAction, 0},
		{"expired", Token{ExpiresAt: at(-time.Minute)}, MarkExpired, 0},
		{"expired and marked", Token{ExpiresAt: at(-time.Minute), RefreshNeeded: true}, NoExpiryAction, 0},
		{"never expires", Token{}, NoExpiryAction, 0},
	}
	for _, c := range cases {
		if action, days := Expiry(c.token, now); action != c.action || days != c.days {
			t.Errorf("%s: Expiry = %v, %d", c.name, action, days)
		}
	}
}

func TestAdministeredPages(t *testing.T) {
	roles := []Role{
		{Organization: "urn:li:organization:1", Role: "ADMINISTRATOR", State: "APPROVED"},
		{Organization: "urn:li:organization:2", Role: "CONTENT_ADMINISTRATOR", State: "APPROVED"},
		{Organization: "urn:li:organization:3", Role: "ANALYST", State: "APPROVED"},
		{Organization: "urn:li:organization:4", Role: "ADMINISTRATOR", State: "REQUESTED"},
		{Organization: "urn:li:organization:1", Role: "CONTENT_ADMINISTRATOR", State: "APPROVED"},
	}
	if got := AdministeredPages(roles); !slices.Equal(got, []string{"1", "2"}) {
		t.Fatalf("AdministeredPages = %v", got)
	}
}
