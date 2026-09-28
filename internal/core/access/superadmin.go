package access

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"
	"time"
)

type Superadmin struct {
	Username      string
	Password      string
	TOTPSecret    []byte
	RecoveryCodes []string
}

func (s Superadmin) HasTOTP() bool {
	return len(s.TOTPSecret) > 0
}

type SuperadminAttempt struct {
	Username string
	Password string
	Code     string
}

// CheckSuperadmin evaluates every factor even after one fails, so the answer
// takes the same time whichever part is wrong.
func CheckSuperadmin(cfg Superadmin, attempt SuperadminAttempt, now time.Time) bool {
	userOK := sameSecret(attempt.Username, cfg.Username)
	passwordOK := sameSecret(attempt.Password, cfg.Password)
	secondFactorOK := true
	if cfg.HasTOTP() {
		code := strings.TrimSpace(attempt.Code)
		totpOK := VerifyTOTP(cfg.TOTPSecret, code, now)
		recoveryOK := matchesRecoveryCode(cfg.RecoveryCodes, code)
		secondFactorOK = totpOK || recoveryOK
	}
	return userOK && passwordOK && secondFactorOK
}

func matchesRecoveryCode(codes []string, code string) bool {
	matched := false
	for _, candidate := range codes {
		if sameSecret(code, candidate) {
			matched = true
		}
	}
	return matched && code != ""
}

func sameSecret(given, expected string) bool {
	a := sha256.Sum256([]byte(given))
	b := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(a[:], b[:]) == 1
}
