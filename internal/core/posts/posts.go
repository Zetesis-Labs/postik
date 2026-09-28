// Package posts holds the pure rules about posts: validation, state changes
// and the next free slot.
package posts

import (
	"errors"
	"html"
	"sort"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"
)

type Status string

const (
	Draft     Status = "draft"
	Scheduled Status = "scheduled"
	Published Status = "published"
	Error     Status = "error"
)

type Type string

const (
	TypeSchedule Type = "schedule"
	TypeDraft    Type = "draft"
	TypeNow      Type = "now"
)

// limits are the character limits of §6.4, counted on the plain text.
var limits = map[string]int{
	"telegram": 4096,
}

// Limit returns the character limit of a provider, or 0 when it has none.
func Limit(provider string) int {
	return limits[provider]
}

// PlainText strips the editor's HTML and decodes its entities, like Postiz
// does before counting characters.
func PlainText(content string) string {
	var b strings.Builder
	inTag := false
	for _, r := range content {
		switch {
		case r == '<':
			inTag = true
		case r == '>' && inTag:
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return html.UnescapeString(b.String())
}

// Length counts like a browser does: UTF-16 code units.
func Length(content string) int {
	return len(utf16.Encode([]rune(PlainText(content))))
}

type Value struct {
	Content    string
	MediaCount int
}

func (v Value) Empty() bool {
	return strings.TrimSpace(PlainText(v.Content)) == "" && v.MediaCount == 0
}

type ChannelSubmission struct {
	ChannelID uuid.UUID
	Provider  string
	Available bool
	Values    []Value
}

type Submission struct {
	// Draft only asks for a principal value with content.
	Draft bool
	// CheckDate rejects a date in the past: creating or rescheduling does,
	// updating the details of a post does not.
	CheckDate bool
	PublishAt time.Time
	Channels  []ChannelSubmission
}

type Problem struct {
	ChannelID  *uuid.UUID
	ValueIndex *int
	Code       string
}

const (
	CodeEmpty              = "empty"
	CodeTooLong            = "too_long"
	CodePastDate           = "past_date"
	CodeChannelUnavailable = "channel_unavailable"
	CodeNoChannels         = "no_channels"
)

// PastMargin tolerates the minute that passes between choosing a time and saving.
const PastMargin = time.Minute

func IsPast(at, now time.Time) bool {
	return at.Before(now.Add(-PastMargin))
}

// PublishAt is the date a new post gets: "now" uses the server clock, and
// every date is kept to the minute.
func PublishAt(kind Type, requested, now time.Time) time.Time {
	if kind == TypeNow {
		return now.UTC().Truncate(time.Minute)
	}
	return requested.UTC().Truncate(time.Minute)
}

// Validate applies §6.5. Drafts only need a principal value with content.
func Validate(s Submission, now time.Time) []Problem {
	var problems []Problem
	if len(s.Channels) == 0 {
		problems = append(problems, Problem{Code: CodeNoChannels})
	}
	for _, c := range s.Channels {
		channelID := c.ChannelID
		if !c.Available {
			problems = append(problems, Problem{ChannelID: &channelID, Code: CodeChannelUnavailable})
			continue
		}
		problems = append(problems, validateValues(c, s.Draft)...)
	}
	if s.CheckDate && IsPast(s.PublishAt, now) {
		problems = append(problems, Problem{Code: CodePastDate})
	}
	return problems
}

func validateValues(c ChannelSubmission, draft bool) []Problem {
	channelID := c.ChannelID
	at := func(i int, code string) Problem {
		index := i
		return Problem{ChannelID: &channelID, ValueIndex: &index, Code: code}
	}
	if len(c.Values) == 0 {
		return []Problem{at(0, CodeEmpty)}
	}
	var problems []Problem
	for i, v := range c.Values {
		if draft && i > 0 {
			break
		}
		if v.Empty() {
			problems = append(problems, at(i, CodeEmpty))
			continue
		}
		if limit := Limit(c.Provider); !draft && limit > 0 && Length(v.Content) > limit {
			problems = append(problems, at(i, CodeTooLong))
		}
	}
	return problems
}

type Mode string

const (
	ModeUpdate   Mode = "update"
	ModeSchedule Mode = "schedule"
)

var ErrRepublishRequired = errors.New("the post was already published")

// AfterEdit is the status of a post saved from the editor.
func AfterEdit(current Status, mode Mode, republish bool) (Status, error) {
	if mode == ModeUpdate {
		return current, nil
	}
	if current == Published && !republish {
		return current, ErrRepublishRequired
	}
	return Scheduled, nil
}

// AfterMove is the status of a post dragged to another slot: like editing,
// except that a draft stays a draft.
func AfterMove(current Status, mode Mode, republish bool) (Status, error) {
	if current == Draft && mode == ModeSchedule {
		return Draft, nil
	}
	return AfterEdit(current, mode, republish)
}

// SlotSearchDays bounds the search for a free slot.
const SlotSearchDays = 365

// NextFreeSlot walks day by day from today (UTC) and returns the first future
// slot minute that no post of the organization takes.
func NextFreeSlot(times []int, taken map[time.Time]bool, now time.Time) (time.Time, bool) {
	sorted := append([]int(nil), times...)
	sort.Ints(sorted)
	day := now.UTC().Truncate(24 * time.Hour)
	for range SlotSearchDays {
		for _, minute := range sorted {
			candidate := day.Add(time.Duration(minute) * time.Minute)
			if candidate.After(now) && !taken[candidate] {
				return candidate, true
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return time.Time{}, false
}

// Excerpt is the start of the plain text shown on calendar cards.
func Excerpt(content string, size int) string {
	runes := []rune(strings.Join(strings.Fields(PlainText(content)), " "))
	if len(runes) <= size {
		return string(runes)
	}
	return string(runes[:size])
}
