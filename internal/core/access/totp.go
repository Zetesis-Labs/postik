package access

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"time"
)

const totpStep = 30 * time.Second

func hotp(secret []byte, counter uint64) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], counter)
	mac := hmac.New(sha1.New, secret)
	mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := uint32(sum[offset]&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000)
}

func totpCounter(t time.Time) int64 {
	return t.Unix() / int64(totpStep/time.Second)
}

// TOTPCode returns the RFC 6238 code (SHA-1, 6 digits, 30 s) for the instant t.
func TOTPCode(secret []byte, t time.Time) string {
	return hotp(secret, uint64(totpCounter(t)))
}

// VerifyTOTP accepts the code of the current step and of the steps right before and after it.
func VerifyTOTP(secret []byte, code string, now time.Time) bool {
	if len(code) != 6 {
		return false
	}
	counter := totpCounter(now)
	matched := 0
	for drift := int64(-1); drift <= 1; drift++ {
		matched |= subtle.ConstantTimeCompare([]byte(hotp(secret, uint64(counter+drift))), []byte(code))
	}
	return matched == 1
}
