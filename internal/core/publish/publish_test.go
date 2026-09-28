package publish

import (
	"reflect"
	"testing"
	"time"
)

func TestTelegramHTML(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"bold becomes b", "<p>Hola <strong>mundo</strong></p>", "Hola <b>mundo</b>"},
		{"underline stays", "<p><u>subrayado</u></p>", "<u>subrayado</u>"},
		{"paragraphs end in a line break", "<p>uno</p><p>dos</p>", "uno\ndos"},
		{"an empty paragraph is a blank line", "<p>uno</p><p></p><p>dos</p>", "uno\n\ndos"},
		{"list items become bullets", "<p>Lista</p><ul><li><p>uno</p></li><li><p>dos</p></li></ul>", "Lista\n• uno\n• dos"},
		{"bare list items too", "<ul><li>uno</li><li>dos</li></ul>", "• uno\n• dos"},
		{"line breaks", "<p>uno<br>dos</p>", "uno\ndos"},
		{"other tags go away", `<p><span class="x">Hola</span> <a href="https://x.test">enlace</a></p>`, "Hola enlace"},
		{"entities stay escaped", "<p>a &lt; b &amp;&amp; c</p>", "a &lt; b &amp;&amp; c"},
	}
	for _, c := range cases {
		if got := TelegramHTML(c.in); got != c.want {
			t.Errorf("%s: TelegramHTML(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}

func TestTelegramURL(t *testing.T) {
	if got := TelegramURL("-1009876543210", "postik_demo", 42); got != "https://t.me/postik_demo/42" {
		t.Errorf("public = %q", got)
	}
	if got := TelegramURL("-1001234567890", "", 42); got != "https://t.me/c/1234567890/42" {
		t.Errorf("private = %q", got)
	}
	if got := TelegramURL("-5001", "", 7); got != "https://t.me/c/5001/7" {
		t.Errorf("group = %q", got)
	}
}

func media(kinds ...string) []Media {
	out := make([]Media, len(kinds))
	for i, k := range kinds {
		out[i] = Media{Kind: k, Path: string(rune('a' + i))}
	}
	return out
}

func TestTelegramCalls(t *testing.T) {
	if got := TelegramCalls("hola", nil); !reflect.DeepEqual(got, []Call{{Method: "sendMessage", Text: "hola"}}) {
		t.Errorf("text only = %+v", got)
	}
	if got := TelegramCalls("foto", media("image")); !reflect.DeepEqual(got, []Call{{Method: "sendPhoto", Text: "foto", Media: media("image")}}) {
		t.Errorf("one image = %+v", got)
	}
	if got := TelegramCalls("vídeo", media("video")); !reflect.DeepEqual(got, []Call{{Method: "sendVideo", Text: "vídeo", Media: media("video")}}) {
		t.Errorf("one video = %+v", got)
	}

	twelve := make([]string, 12)
	for i := range twelve {
		twelve[i] = "image"
	}
	calls := TelegramCalls("álbum", media(twelve...))
	if len(calls) != 2 || calls[0].Method != "sendMediaGroup" || len(calls[0].Media) != 10 || calls[0].Text != "álbum" ||
		calls[1].Method != "sendMediaGroup" || len(calls[1].Media) != 2 || calls[1].Text != "" {
		t.Errorf("twelve images = %+v", calls)
	}

	eleven := twelve[:11]
	calls = TelegramCalls("once", media(eleven...))
	if len(calls) != 2 || calls[0].Method != "sendMediaGroup" || calls[1].Method != "sendPhoto" || calls[1].Text != "" {
		t.Errorf("a lone last item cannot be a media group: %+v", calls)
	}
}

func TestNext(t *testing.T) {
	at := time.Date(2026, 9, 29, 10, 0, 0, 0, time.UTC)
	base := Turn{Index: 0, JobPublishAt: at, PostFound: true, PostStatus: "scheduled", PostPublishAt: at}
	with := func(change func(*Turn)) Turn {
		t := base
		change(&t)
		return t
	}
	cases := []struct {
		name string
		turn Turn
		want Action
	}{
		{"a due post is sent", base, Send},
		{"a deleted post is skipped", with(func(t *Turn) { t.PostFound = false }), Skip},
		{"a draft is skipped", with(func(t *Turn) { t.PostStatus = "draft" }), Skip},
		{"a moved post is skipped", with(func(t *Turn) { t.PostPublishAt = at.Add(time.Hour) }), Skip},
		{"a published post is not sent again", with(func(t *Turn) { t.PostStatus = "published" }), Skip},
		{"a disabled channel fails", with(func(t *Turn) { t.ChannelDisabled = true }), FailChannelDisabled},
		{"a channel to reconnect fails", with(func(t *Turn) { t.ChannelRefresh = true }), FailChannelRefresh},
		{"a value already sent moves on", with(func(t *Turn) { t.Delivery = DeliverySent }), AlreadySent},
		{"a value left sending is unconfirmed", with(func(t *Turn) { t.Delivery = DeliverySending }), FailUnconfirmed},
		{"a failed value is done", with(func(t *Turn) { t.Delivery = DeliveryFailed }), Skip},
		{"a comment needs the post published", with(func(t *Turn) { t.Index = 1 }), Skip},
		{"a comment of a published post is sent", with(func(t *Turn) { t.Index, t.PostStatus = 1, "published" }), Send},
	}
	for _, c := range cases {
		if got := Next(c.turn); got != c.want {
			t.Errorf("%s: Next = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAfterFailure(t *testing.T) {
	cases := []struct {
		name    string
		kind    Failure
		attempt int
		want    Verdict
	}{
		{"a rejection is final", Rejected, 1, Verdict{Error: "Bad Request: nope"}},
		{"an unknown outcome is never retried", Unknown, 1, Verdict{Error: CodeUnconfirmed}},
		{"a call that never started is retried", NotStarted, 1, Verdict{Retry: true}},
		{"until the last attempt", NotStarted, MaxAttempts, Verdict{Error: CodeUnreachable}},
		{"a rate limit is retried", RateLimited, 4, Verdict{Retry: true}},
		{"and settles on the last attempt with Telegram's words", RateLimited, MaxAttempts, Verdict{Error: "Bad Request: nope"}},
	}
	for _, c := range cases {
		if got := AfterFailure(c.kind, "Bad Request: nope", c.attempt); got != c.want {
			t.Errorf("%s: AfterFailure = %+v, want %+v", c.name, got, c.want)
		}
	}
}
