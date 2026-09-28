package access

import (
	"testing"
	"time"
)

func TestTOTPMatchesRFC6238Vector(t *testing.T) {
	secret := []byte("12345678901234567890")
	cases := map[int64]string{
		59:         "287082",
		1111111109: "081804",
		1234567890: "005924",
		2000000000: "279037",
	}
	for unix, want := range cases {
		if got := TOTPCode(secret, time.Unix(unix, 0)); got != want {
			t.Errorf("TOTPCode(%d) = %s, want %s", unix, got, want)
		}
	}
}

func TestVerifyTOTPWindow(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Date(2026, 9, 28, 10, 0, 15, 0, time.UTC)
	for _, offset := range []time.Duration{-totpStep, 0, totpStep} {
		if !VerifyTOTP(secret, TOTPCode(secret, now.Add(offset)), now) {
			t.Errorf("code at offset %s was rejected", offset)
		}
	}
	for _, offset := range []time.Duration{-2 * totpStep, 2 * totpStep} {
		if VerifyTOTP(secret, TOTPCode(secret, now.Add(offset)), now) {
			t.Errorf("code at offset %s was accepted", offset)
		}
	}
	if VerifyTOTP(secret, "", now) || VerifyTOTP(secret, "12345", now) {
		t.Error("malformed codes must be rejected")
	}
}

func TestMemberSessionExpiresOnIdleOrMaximum(t *testing.T) {
	created := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	idle := SessionTimes{Kind: SessionMember, CreatedAt: created, LastSeenAt: created}
	if got := SessionExpiresAt(idle); !got.Equal(created.Add(MemberSessionIdleTTL)) {
		t.Errorf("idle expiry = %s", got)
	}
	busy := SessionTimes{Kind: SessionMember, CreatedAt: created, LastSeenAt: created.Add(29 * 24 * time.Hour)}
	if got := SessionExpiresAt(busy); !got.Equal(created.Add(MemberSessionMaxTTL)) {
		t.Errorf("busy expiry = %s, want the 30-day cap", got)
	}
	verdict := EvaluateSession(idle, created.Add(10*time.Minute))
	if !verdict.Valid || !verdict.Touch {
		t.Errorf("a member session in use must stay valid and be touched: %+v", verdict)
	}
}

func TestSuperadminSessionNeverRenews(t *testing.T) {
	created := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	s := SessionTimes{Kind: SessionSuperadmin, CreatedAt: created, LastSeenAt: created.Add(11 * time.Hour)}
	if verdict := EvaluateSession(s, created.Add(11*time.Hour+59*time.Minute)); !verdict.Valid || verdict.Touch {
		t.Errorf("before 12 h: %+v", verdict)
	}
	if verdict := EvaluateSession(s, created.Add(12*time.Hour)); verdict.Valid {
		t.Error("the superadmin session must expire at 12 h")
	}
}
