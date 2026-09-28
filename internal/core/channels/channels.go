// Package channels holds the pure rules about channels and how they connect.
package channels

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
)

const ConnectionTTL = 30 * time.Minute

var DefaultPostingTimes = []int{120, 400, 700}

var connectCommand = regexp.MustCompile(`^/connect(?:@([A-Za-z0-9_]+))? ([A-Za-z0-9]{4})$`)

// ParseConnectCommand recognises "/connect CODE" and "/connect@bot CODE"
// addressed to botUsername.
func ParseConnectCommand(text, botUsername string) (string, bool) {
	match := connectCommand.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return "", false
	}
	if match[1] != "" && !strings.EqualFold(match[1], botUsername) {
		return "", false
	}
	return match[2], true
}

type ConnectionStatus string

const (
	Pending   ConnectionStatus = "pending"
	Connected ConnectionStatus = "connected"
	Expired   ConnectionStatus = "expired"
)

func ConnectionState(createdAt time.Time, connected bool, now time.Time) ConnectionStatus {
	switch {
	case connected:
		return Connected
	case now.Sub(createdAt) >= ConnectionTTL:
		return Expired
	default:
		return Pending
	}
}

type Chat struct {
	Type      string
	Title     string
	FirstName string
	LastName  string
}

func ChatName(c Chat) string {
	if c.Title != "" {
		return c.Title
	}
	return strings.TrimSpace(c.FirstName + " " + c.LastName)
}

var ErrPostingTimeOutOfRange = errors.New("posting times must be between 0 and 1439 minutes")

// NormalizePostingTimes sorts the slots, drops repeated ones and rejects any
// outside the day.
func NormalizePostingTimes(times []int) ([]int, error) {
	normalized := make([]int, 0, len(times))
	for _, minute := range times {
		if minute < 0 || minute > 1439 {
			return nil, ErrPostingTimeOutOfRange
		}
		normalized = append(normalized, minute)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

// CustomerName trims the name; an empty result means "no customer".
func CustomerName(name string) string {
	return strings.TrimSpace(name)
}
