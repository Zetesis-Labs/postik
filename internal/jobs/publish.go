package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/riverqueue/river"

	"github.com/zetesis-labs/postik/internal/core/publish"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/tokens"
)

// Sent is a value the network published.
type Sent struct {
	ID  string
	URL string
}

// Publisher sends the values of posts to one network (S06 §5).
type Publisher interface {
	// Prepare does what can be repeated without publishing anything, like
	// renewing the token or uploading the media.
	Prepare(ctx context.Context, post *postgres.Post, index int) (any, error)
	// Create publishes a prepared value: it is the point of no return.
	// replyTo is the delivery a comment answers.
	Create(ctx context.Context, post *postgres.Post, index int, prepared any, replyTo *postgres.Delivery) (Sent, error)
	// ReplyTo is the index of the value a comment answers.
	ReplyTo(index int) int
	// Classify says what a failure of the network means.
	Classify(err error) (publish.Failure, string)
}

// PublishValueWorker sends one value of a post (S05 §4).
type PublishValueWorker struct {
	river.WorkerDefaults[PublishValueArgs]
	deps       Deps
	jobs       *Jobs
	publishers map[string]Publisher
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
	publisher, ok := w.publishers[post.Channel.Provider]
	if !ok {
		return fmt.Errorf("no publisher for %s", post.Channel.Provider)
	}
	return w.send(ctx, job, post, publisher)
}

func (w *PublishValueWorker) send(ctx context.Context, job *river.Job[PublishValueArgs], post *postgres.Post, publisher Publisher) error {
	args := job.Args
	prepared, err := publisher.Prepare(ctx, post, args.Index)
	if err != nil {
		return w.afterPrepareError(ctx, job, post, publisher, err)
	}
	var replyTo *postgres.Delivery
	if args.Index > 0 {
		if replyTo, err = w.deps.Store.Delivery(ctx, args.PostID, args.PublishAt, publisher.ReplyTo(args.Index)); err != nil {
			return err
		}
	}
	if err := w.deps.Store.StartSending(ctx, args.PostID, args.PublishAt, args.Index, w.deps.Now()); err != nil {
		if errors.Is(err, postgres.ErrConflict) {
			return nil
		}
		return err
	}

	sent, sendErr := publisher.Create(ctx, post, args.Index, prepared, replyTo)
	if sendErr != nil {
		return w.afterSendError(ctx, job, post, publisher, sendErr)
	}

	now := w.deps.Now()
	if err := w.deps.Store.MarkSent(ctx, args.PostID, args.PublishAt, args.Index, sent.ID, sent.URL, now); err != nil {
		return err
	}
	if args.Index == 0 {
		if err := w.deps.Store.PublishPost(ctx, args.PostID, args.PublishAt, sent.URL, now); err != nil {
			return err
		}
		w.notify(ctx, post, "success", "published", map[string]string{"url": sent.URL})
	}
	return w.queueNext(ctx, post, args)
}

// afterPrepareError handles a failure before the point of no return: it is
// retried unless the network refused the value or the channel lost its tokens.
func (w *PublishValueWorker) afterPrepareError(ctx context.Context, job *river.Job[PublishValueArgs], post *postgres.Post, publisher Publisher, err error) error {
	if lostTokens(err) {
		return w.channelRefresh(ctx, post, job.Args, err)
	}
	kind, reason := publisher.Classify(err)
	if kind == publish.Unknown || kind == publish.RateLimited {
		kind = publish.Interrupted
	}
	verdict := publish.AfterFailure(kind, reason, job.Attempt)
	if verdict.Retry {
		return fmt.Errorf("attempt %d: %w", job.Attempt, err)
	}
	w.deps.Logger.WarnContext(ctx, "a value could not be prepared", "post", post.ID, "index", job.Args.Index, "error", err)
	return w.settleFailure(ctx, post, job.Args, verdict.Error, "failure", "failed")
}

// afterSendError applies the retry rules: only a send that never started is
// tried again; anything else settles the value.
func (w *PublishValueWorker) afterSendError(ctx context.Context, job *river.Job[PublishValueArgs], post *postgres.Post, publisher Publisher, sendErr error) error {
	args := job.Args
	if lostTokens(sendErr) {
		return w.channelRefresh(ctx, post, args, sendErr)
	}
	kind, reason := publisher.Classify(sendErr)
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

func lostTokens(err error) bool {
	return errors.Is(err, tokens.ErrCannotRenew) || errors.Is(err, tokens.ErrNoCredentials)
}

// channelRefresh handles a channel whose tokens cannot be renewed: it needs
// reconnection, the value fails and everyone is told (F12).
func (w *PublishValueWorker) channelRefresh(ctx context.Context, post *postgres.Post, args PublishValueArgs, cause error) error {
	w.deps.Logger.WarnContext(ctx, "the channel needs reconnection", "channel", post.ChannelID, "error", cause)
	if err := w.deps.Channels.MarkRefreshNeeded(ctx, post.ChannelID, w.deps.Now()); err != nil {
		return err
	}
	if err := w.settle(ctx, post, args, publish.CodeChannelRefresh); err != nil {
		return err
	}
	w.notify(ctx, post, "info", "refresh_failed", map[string]string{})
	return nil
}

// settleFailure records a value that will not be sent and tells the
// organization.
func (w *PublishValueWorker) settleFailure(ctx context.Context, post *postgres.Post, args PublishValueArgs, reason, kind, template string) error {
	if err := w.settle(ctx, post, args, reason); err != nil {
		return err
	}
	if args.Index > 0 && template == "failed" {
		template = "comment_failed"
	}
	w.notify(ctx, post, kind, template, map[string]string{"reason": reason})
	return nil
}

// settle records a value that will not be sent. The principal value takes
// the post to error; a comment leaves the post published.
func (w *PublishValueWorker) settle(ctx context.Context, post *postgres.Post, args PublishValueArgs, reason string) error {
	now := w.deps.Now()
	if err := w.deps.Store.MarkFailed(ctx, args.PostID, args.PublishAt, args.Index, reason, now); err != nil {
		return err
	}
	if args.Index == 0 {
		return w.deps.Store.FailPost(ctx, args.PostID, args.PublishAt, reason, now)
	}
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
