package access

import "time"

type SessionKind string

const (
	SessionSuperadmin SessionKind = "superadmin"
	SessionMember     SessionKind = "member"
)

const (
	SuperadminSessionTTL = 12 * time.Hour
	MemberSessionIdleTTL = 7 * 24 * time.Hour
	MemberSessionMaxTTL  = 30 * 24 * time.Hour
	sessionTouchInterval = 5 * time.Minute
)

type SessionTimes struct {
	Kind       SessionKind
	CreatedAt  time.Time
	LastSeenAt time.Time
}

// SessionExpiresAt is when the session stops being valid if nobody uses it again.
func SessionExpiresAt(s SessionTimes) time.Time {
	if s.Kind == SessionSuperadmin {
		return s.CreatedAt.Add(SuperadminSessionTTL)
	}
	idle := s.LastSeenAt.Add(MemberSessionIdleTTL)
	maximum := s.CreatedAt.Add(MemberSessionMaxTTL)
	if idle.Before(maximum) {
		return idle
	}
	return maximum
}

// CookieExpiresAt is the latest instant the session could still be valid.
func CookieExpiresAt(s SessionTimes) time.Time {
	if s.Kind == SessionSuperadmin {
		return s.CreatedAt.Add(SuperadminSessionTTL)
	}
	return s.CreatedAt.Add(MemberSessionMaxTTL)
}

type SessionVerdict struct {
	Valid bool
	Touch bool
}

// EvaluateSession decides whether a session is still valid at now and whether
// its last use must be recorded. Superadmin sessions never renew.
func EvaluateSession(s SessionTimes, now time.Time) SessionVerdict {
	valid := now.Before(SessionExpiresAt(s))
	touch := valid && s.Kind == SessionMember && now.Sub(s.LastSeenAt) >= sessionTouchInterval
	return SessionVerdict{Valid: valid, Touch: touch}
}
