package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/riverqueue/river"

	"github.com/zetesis-labs/postik/internal/core/publish"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/telegram"
)

// PublishValueWorker sends one value of a post (§4).
type PublishValueWorker struct {
	river.WorkerDefaults[PublishValueArgs]
	deps Deps
	jobs *Jobs
}

func (w *PublishValueWorker) Work(ctx context.Context, job *river.Job[PublishValueArgs]) error {
	args := job.Args
	post, err := w.deps.Store.Post(ctx, args.PostID)
	if errors.Is(err, postgres.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	delivery, err := w.deps.Store.Delivery(ctx, args.PostID, args.PublishAt, args.Index)
	if err != nil {
		return err
	}
	turn := publish.Turn{
		Index:           args.Index,
		JobPublishAt:    args.PublishAt,
		PostFound:       true,
		PostStatus:      post.Status,
		PostPublishAt:   post.PublishAt,
		ChannelDisabled: post.Channel.Disabled,
		ChannelRefresh:  post.Channel.RefreshNeeded,
	}
	if delivery != nil {
		turn.Delivery = publish.Delivery(delivery.State)
	}

	switch publish.Next(turn) {
	case publish.Skip:
		return nil
	case publish.FailChannelDisabled:
		return w.settleFailure(ctx, post, args, publish.CodeChannelDisabled, "info", "channel_disabled")
	case publish.FailChannelRefresh:
		return w.settleFailure(ctx, post, args, publish.CodeChannelRefresh, "info", "channel_refresh")
	case publish.FailUnconfirmed:
		return w.settleFailure(ctx, post, args, publish.CodeUnconfirmed, "failure", "unconfirmed")
	case publish.AlreadySent:
		return w.queueNext(ctx, post, args)
	}
	if args.Index >= len(post.Values) {
		return nil
	}
	return w.send(ctx, job, post)
}

func (w *PublishValueWorker) send(ctx context.Context, job *river.Job[PublishValueArgs], post *postgres.Post) error {
	args := job.Args
	replyTo, err := w.replyTo(ctx, args)
	if err != nil {
		return err
	}
	if err := w.deps.Store.StartSending(ctx, args.PostID, args.PublishAt, args.Index, w.deps.Now()); err != nil {
		if errors.Is(err, postgres.ErrConflict) {
			return nil
		}
		return err
	}

	messageID, sendErr := w.sendValue(ctx, post, post.Values[args.Index], replyTo)
	if sendErr != nil {
		return w.afterSendError(ctx, job, post, sendErr)
	}

	now := w.deps.Now()
	url := publish.TelegramURL(post.Channel.ExternalID, post.Channel.Username, messageID)
	if err := w.deps.Store.MarkSent(ctx, args.PostID, args.PublishAt, args.Index, strconv.FormatInt(messageID, 10), url, now); err != nil {
		return err
	}
	if args.Index == 0 {
		if err := w.deps.Store.PublishPost(ctx, args.PostID, args.PublishAt, url, now); err != nil {
			return err
		}
		w.notify(ctx, post, "success", "published", map[string]string{"url": url})
	}
	return w.queueNext(ctx, post, args)
}

// replyTo is the message of the previous value, which a comment answers.
func (w *PublishValueWorker) replyTo(ctx context.Context, args PublishValueArgs) (int64, error) {
	if args.Index == 0 {
		return 0, nil
	}
	previous, err := w.deps.Store.Delivery(ctx, args.PostID, args.PublishAt, args.Index-1)
	if err != nil || previous == nil || previous.ExternalID == nil {
		return 0, err
	}
	return strconv.ParseInt(*previous.ExternalID, 10, 64)
}

// sendValue makes the calls of §5 and returns the ID of the first message.
func (w *PublishValueWorker) sendValue(ctx context.Context, post *postgres.Post, value postgres.PostValue, replyTo int64) (int64, error) {
	if w.deps.Telegram == nil {
		return 0, &telegram.NotSentError{Err: errors.New("telegram is not configured")}
	}
	chatID, err := strconv.ParseInt(post.Channel.ExternalID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("chat ID %q: %w", post.Channel.ExternalID, err)
	}
	media := make([]publish.Media, len(value.Media))
	for i, m := range value.Media {
		media[i] = publish.Media{Kind: m.Kind, Path: m.URL}
	}
	var first int64
	for i, call := range publish.TelegramCalls(publish.TelegramHTML(value.Content), media) {
		reply := int64(0)
		if i == 0 {
			reply = replyTo
		}
		id, err := w.call(ctx, chatID, call, reply)
		if err != nil {
			if i == 0 {
				return 0, err
			}
			w.deps.Logger.ErrorContext(ctx, "part of a value was not sent", "post", post.ID, "call", i, "error", err)
			return first, nil
		}
		if i == 0 {
			first = id
		}
	}
	return first, nil
}

func (w *PublishValueWorker) call(ctx context.Context, chatID int64, call publish.Call, replyTo int64) (int64, error) {
	client := w.deps.Telegram
	switch call.Method {
	case "sendMessage":
		m, err := client.SendMessage(ctx, chatID, call.Text, replyTo)
		return m.MessageID, err
	case "sendMediaGroup":
		files := make([]telegram.File, len(call.Media))
		for i, m := range call.Media {
			files[i] = w.file(m)
		}
		messages, err := client.SendMediaGroup(ctx, chatID, files, call.Text, replyTo)
		if err != nil || len(messages) == 0 {
			return 0, err
		}
		return messages[0].MessageID, nil
	default:
		m, err := client.SendFile(ctx, chatID, w.file(call.Media[0]), call.Text, replyTo)
		return m.MessageID, err
	}
}

func (w *PublishValueWorker) file(m publish.Media) telegram.File {
	kind := "photo"
	if m.Kind == "video" {
		kind = "video"
	}
	return telegram.File{
		Type: kind,
		Name: path.Base(m.Path),
		Open: func() (io.ReadCloser, error) { return w.deps.Files.Open(m.Path) },
	}
}

// afterSendError applies the retry rules: only a send that never started is
// tried again; anything else settles the value.
func (w *PublishValueWorker) afterSendError(ctx context.Context, job *river.Job[PublishValueArgs], post *postgres.Post, sendErr error) error {
	args := job.Args
	kind, reason := classify(sendErr)
	verdict := publish.AfterFailure(kind, reason, job.Attempt)
	if verdict.Retry {
		if err := w.deps.Store.ForgetSending(ctx, args.PostID, args.PublishAt, args.Index); err != nil {
			return err
		}
		return fmt.Errorf("attempt %d: %w", job.Attempt, sendErr)
	}
	w.deps.Logger.WarnContext(ctx, "a value was not published", "post", post.ID, "index", args.Index, "error", sendErr)
	template := "failed"
	if verdict.Error == publish.CodeUnconfirmed {
		template = "unconfirmed"
	}
	return w.settleFailure(ctx, post, args, verdict.Error, "failure", template)
}

// classify turns an error from the Bot API into a publish.Failure.
func classify(err error) (publish.Failure, string) {
	var notSent *telegram.NotSentError
	if errors.As(err, &notSent) {
		return publish.NotStarted, publish.CodeUnreachable
	}
	var apiErr *telegram.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Code == http.StatusTooManyRequests:
			return publish.RateLimited, apiErr.Description
		case apiErr.Code >= http.StatusInternalServerError:
			return publish.Unknown, apiErr.Description
		}
		return publish.Rejected, apiErr.Description
	}
	return publish.Unknown, err.Error()
}

// settleFailure records a value that will not be sent. The principal value
// takes the post to error; a comment leaves the post published.
func (w *PublishValueWorker) settleFailure(ctx context.Context, post *postgres.Post, args PublishValueArgs, reason, kind, template string) error {
	now := w.deps.Now()
	if err := w.deps.Store.MarkFailed(ctx, args.PostID, args.PublishAt, args.Index, reason, now); err != nil {
		return err
	}
	if args.Index == 0 {
		if err := w.deps.Store.FailPost(ctx, args.PostID, args.PublishAt, reason, now); err != nil {
			return err
		}
	} else if template == "failed" {
		template = "comment_failed"
	}
	w.notify(ctx, post, kind, template, map[string]string{"reason": reason})
	return nil
}

func (w *PublishValueWorker) queueNext(ctx context.Context, post *postgres.Post, args PublishValueArgs) error {
	next := args.Index + 1
	if next >= len(post.Values) {
		return nil
	}
	at := w.deps.Now().Add(time.Duration(post.Values[next].DelayMinutes) * time.Minute)
	return w.jobs.enqueue(ctx, nil, PublishValueArgs{PostID: args.PostID, PublishAt: args.PublishAt, Index: next}, at)
}

func (w *PublishValueWorker) notify(ctx context.Context, post *postgres.Post, kind, template string, params map[string]string) {
	params["channel"] = post.Channel.Name
	params["provider"] = post.Channel.Provider
	params["postId"] = post.ID.String()
	if err := w.deps.Notifier.Notify(ctx, post.OrganizationID, Notification{Kind: kind, Template: template, Params: params}); err != nil {
		w.deps.Logger.ErrorContext(ctx, "notify", "post", post.ID, "template", template, "error", err)
	}
}
