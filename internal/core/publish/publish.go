// Package publish holds the pure rules of sending a post: what goes to
// Telegram, whether a job still has work to do and what a failure means.
package publish

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MaxAttempts bounds the tries of a value whose call never reached the network.
const MaxAttempts = 5

// Codes stored in a post's error instead of a message from the network.
const (
	CodeChannelDisabled = "channel_disabled"
	CodeChannelRefresh  = "channel_refresh"
	CodeUnconfirmed     = "unconfirmed"
	CodeUnreachable     = "unreachable"
)

var (
	strongTag    = regexp.MustCompile(`(?i)<(/?)strong\b[^>]*>`)
	keptTag      = regexp.MustCompile(`(?i)<(/?)(b|u)\b[^>]*>`)
	paragraphEnd = regexp.MustCompile(`(?i)</p>\s*</li>`)
	lineBreak    = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>`)
	listItem     = regexp.MustCompile(`(?i)<li\b[^>]*>`)
	anyTag       = regexp.MustCompile(`<[^>]*>`)
)

// TelegramHTML turns the editor's HTML into what Telegram's HTML parse mode
// accepts, like Postiz: bold and underline stay, paragraphs and list items
// become lines and every other tag goes. Entities stay escaped.
func TelegramHTML(content string) string {
	text := strongTag.ReplaceAllString(content, "<${1}b>")
	text = paragraphEnd.ReplaceAllString(text, "</li>")
	text = listItem.ReplaceAllString(text, "• ")
	text = lineBreak.ReplaceAllString(text, "\n")
	text = keptTag.ReplaceAllStringFunc(text, func(tag string) string {
		return "\x00" + strings.ToLower(strings.TrimSpace(strings.Trim(tag, "<>"))) + "\x01"
	})
	text = anyTag.ReplaceAllString(text, "")
	text = strings.NewReplacer("\x00", "<", "\x01", ">").Replace(text)
	return strings.TrimRight(text, "\n")
}

// TelegramURL is the link to a message: by the chat's public name when it
// has one, and by its ID without the -100 prefix otherwise.
func TelegramURL(chatID, username string, messageID int64) string {
	id := strconv.FormatInt(messageID, 10)
	if username != "" {
		return "https://t.me/" + username + "/" + id
	}
	chat := strings.TrimPrefix(strings.TrimPrefix(chatID, "-100"), "-")
	return "https://t.me/c/" + chat + "/" + id
}

// Media is a file of a value: Kind is "image" or "video".
type Media struct {
	Kind string
	Path string
}

// Call is one request to the Bot API. Text is the message or its caption.
type Call struct {
	Method string
	Text   string
	Media  []Media
}

const mediaGroupSize = 10

// TelegramCalls plans the requests that send a value, in order. Only the first
// one carries the text.
func TelegramCalls(text string, media []Media) []Call {
	switch len(media) {
	case 0:
		return []Call{{Method: "sendMessage", Text: text}}
	case 1:
		return []Call{single(text, media[0])}
	}
	var calls []Call
	for start := 0; start < len(media); start += mediaGroupSize {
		chunk := media[start:min(start+mediaGroupSize, len(media))]
		caption := ""
		if start == 0 {
			caption = text
		}
		if len(chunk) == 1 {
			calls = append(calls, single(caption, chunk[0]))
			continue
		}
		calls = append(calls, Call{Method: "sendMediaGroup", Text: caption, Media: chunk})
	}
	return calls
}

func single(text string, m Media) Call {
	method := "sendPhoto"
	if m.Kind == "video" {
		method = "sendVideo"
	}
	return Call{Method: method, Text: text, Media: []Media{m}}
}

// Delivery is what a previous attempt left recorded for a value.
type Delivery string

const (
	DeliveryNone    Delivery = ""
	DeliverySending Delivery = "sending"
	DeliverySent    Delivery = "sent"
	DeliveryFailed  Delivery = "failed"
)

// Turn is what publish_value knows when it runs.
type Turn struct {
	Index           int
	JobPublishAt    time.Time
	PostFound       bool
	PostStatus      string
	PostPublishAt   time.Time
	ChannelDisabled bool
	ChannelRefresh  bool
	Delivery        Delivery
}

type Action int

const (
	Skip Action = iota
	Send
	AlreadySent
	FailChannelDisabled
	FailChannelRefresh
	FailUnconfirmed
)

func (a Action) String() string {
	return [...]string{"skip", "send", "already sent", "fail: channel disabled", "fail: channel refresh", "fail: unconfirmed"}[a]
}

// Next decides what publish_value does (§4, steps 1 to 3).
func Next(t Turn) Action {
	wantStatus := "scheduled"
	if t.Index > 0 {
		wantStatus = "published"
	}
	if !t.PostFound || t.PostStatus != wantStatus || !t.PostPublishAt.Equal(t.JobPublishAt) {
		return Skip
	}
	switch {
	case t.ChannelDisabled:
		return FailChannelDisabled
	case t.ChannelRefresh:
		return FailChannelRefresh
	}
	switch t.Delivery {
	case DeliverySent:
		return AlreadySent
	case DeliverySending:
		return FailUnconfirmed
	case DeliveryFailed:
		return Skip
	}
	return Send
}

// Failure is how a call to the network went wrong.
type Failure int

const (
	// NotStarted: the request never reached the network (it refused the connection).
	NotStarted Failure = iota
	// RateLimited: the network answered 429 without doing anything.
	RateLimited
	// Rejected: the network answered that it will not publish this.
	Rejected
	// Unknown: the request went out and no answer came back.
	Unknown
)

// Verdict says whether to try again or, if not, the error the post keeps.
type Verdict struct {
	Retry bool
	Error string
}

// AfterFailure applies the retry rules of F11: only what never started is
// retried, and never beyond MaxAttempts.
func AfterFailure(kind Failure, reason string, attempt int) Verdict {
	switch kind {
	case Rejected:
		return Verdict{Error: reason}
	case Unknown:
		return Verdict{Error: CodeUnconfirmed}
	}
	if attempt < MaxAttempts {
		return Verdict{Retry: true}
	}
	if kind == NotStarted {
		return Verdict{Error: CodeUnreachable}
	}
	return Verdict{Error: reason}
}
