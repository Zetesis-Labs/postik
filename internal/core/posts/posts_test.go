package posts

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPlainTextAndLength(t *testing.T) {
	if got := PlainText("<p>Hola <strong>mundo</strong> &amp; más</p>"); got != "Hola mundo & más" {
		t.Errorf("PlainText = %q", got)
	}
	if got := Length("<p>😀</p>"); got != 2 {
		t.Errorf("an emoji counts as two UTF-16 units, got %d", got)
	}
}

func TestValidate(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	ok := ChannelSubmission{ChannelID: uuid.New(), Provider: "telegram", Available: true, Values: []Value{{Content: "<p>Hola</p>"}}}
	if p := Validate(Submission{CheckDate: true, PublishAt: now.Add(time.Hour), Channels: []ChannelSubmission{ok}}, now); len(p) != 0 {
		t.Fatalf("valid submission: %+v", p)
	}
	long := ok
	long.Values = []Value{{Content: strings.Repeat("a", 4097)}}
	if p := Validate(Submission{CheckDate: true, PublishAt: now.Add(time.Hour), Channels: []ChannelSubmission{long}}, now); len(p) != 1 || p[0].Code != CodeTooLong {
		t.Fatalf("too long: %+v", p)
	}
	if p := Validate(Submission{Draft: true, PublishAt: now.Add(-time.Hour), Channels: []ChannelSubmission{long}}, now); len(p) != 0 {
		t.Fatalf("a draft is not checked for length or date: %+v", p)
	}
	unavailable := ok
	unavailable.Available = false
	if p := Validate(Submission{CheckDate: true, PublishAt: now.Add(time.Hour), Channels: []ChannelSubmission{unavailable}}, now); len(p) != 1 || p[0].Code != CodeChannelUnavailable {
		t.Fatalf("unavailable: %+v", p)
	}
	if p := Validate(Submission{CheckDate: true, PublishAt: now.Add(-30 * time.Second), Channels: []ChannelSubmission{ok}}, now); len(p) != 0 {
		t.Fatalf("within the minute of margin: %+v", p)
	}
	withMedia := ok
	withMedia.Values = []Value{{Content: "", MediaCount: 1}}
	if p := Validate(Submission{CheckDate: true, PublishAt: now.Add(time.Hour), Channels: []ChannelSubmission{withMedia}}, now); len(p) != 0 {
		t.Fatalf("an image alone is content: %+v", p)
	}
}

func TestTransitions(t *testing.T) {
	cases := []struct {
		name      string
		move      bool
		current   Status
		mode      Mode
		republish bool
		want      Status
		err       bool
	}{
		{"edit update keeps", false, Draft, ModeUpdate, false, Draft, false},
		{"edit schedule a draft", false, Draft, ModeSchedule, false, Scheduled, false},
		{"edit published needs confirmation", false, Published, ModeSchedule, false, Published, true},
		{"edit published confirmed", false, Published, ModeSchedule, true, Scheduled, false},
		{"edit an error reschedules", false, Error, ModeSchedule, false, Scheduled, false},
		{"move draft stays draft", true, Draft, ModeSchedule, false, Draft, false},
		{"move published update", true, Published, ModeUpdate, false, Published, false},
		{"move published schedule", true, Published, ModeSchedule, false, Published, true},
	}
	for _, tc := range cases {
		transition := AfterEdit
		if tc.move {
			transition = AfterMove
		}
		got, err := transition(tc.current, tc.mode, tc.republish)
		if got != tc.want || (err != nil) != tc.err {
			t.Errorf("%s: %v, %v", tc.name, got, err)
		}
	}
}

func TestNextFreeSlot(t *testing.T) {
	now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
	day := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	taken := map[time.Time]bool{day.Add(400 * time.Minute): true}
	got, ok := NextFreeSlot([]int{700, 120, 400}, taken, now)
	if !ok || !got.Equal(day.Add(700*time.Minute)) {
		t.Fatalf("got %s", got)
	}
	taken[day.Add(700*time.Minute)] = true
	if got, _ := NextFreeSlot([]int{120, 400, 700}, taken, now); !got.Equal(day.AddDate(0, 0, 1).Add(120 * time.Minute)) {
		t.Fatalf("next day: %s", got)
	}
	if _, ok := NextFreeSlot(nil, taken, now); ok {
		t.Fatal("without slots there is no free slot")
	}
}

func TestPublishAtKeepsMinutes(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 34, 56, 0, time.UTC)
	if got := PublishAt(TypeNow, now.AddDate(-1, 0, 0), now); !got.Equal(time.Date(2026, 9, 28, 12, 34, 0, 0, time.UTC)) {
		t.Fatalf("now = %s", got)
	}
	requested := time.Date(2026, 10, 1, 9, 30, 45, 0, time.UTC)
	if got := PublishAt(TypeSchedule, requested, now); !got.Equal(time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)) {
		t.Fatalf("schedule = %s", got)
	}
}
