package jobs

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/zetesis-labs/postik/internal/core/oauth"
)

type TokenExpiryArgs struct{}

func (TokenExpiryArgs) Kind() string { return "token_expiry" }

// ExpiryWorker warns, once an hour, about the tokens that will expire and
// cannot be renewed, and asks for reconnection when they have (S06 §4).
type ExpiryWorker struct {
	river.WorkerDefaults[TokenExpiryArgs]
	deps Deps
}

func (w *ExpiryWorker) Work(ctx context.Context, _ *river.Job[TokenExpiryArgs]) error {
	if w.deps.Tokens == nil {
		return nil
	}
	expiring, err := w.deps.Tokens.Store.Expiring(ctx)
	if err != nil {
		return err
	}
	now := w.deps.Now()
	for _, c := range expiring {
		action, days := oauth.Expiry(oauth.Token{ExpiresAt: c.ExpiresAt, WarnedFor: c.ExpiryWarnedFor, RefreshNeeded: c.RefreshNeeded}, now)
		params := map[string]string{"channel": c.Name, "provider": c.Provider, "channelId": c.ChannelID.String()}
		switch action {
		case oauth.WarnExpiry:
			if err := w.deps.Tokens.Store.MarkWarned(ctx, c.ChannelID, *c.ExpiresAt); err != nil {
				return err
			}
			params["days"] = strconv.Itoa(days)
			w.notify(ctx, c.OrganizationID, "channel_expiring", params)
		case oauth.MarkExpired:
			if err := w.deps.Channels.MarkRefreshNeeded(ctx, c.ChannelID, now); err != nil {
				return err
			}
			w.notify(ctx, c.OrganizationID, "refresh_failed", params)
		}
	}
	return nil
}

func (w *ExpiryWorker) notify(ctx context.Context, orgID uuid.UUID, template string, params map[string]string) {
	if err := w.deps.Notifier.Notify(ctx, orgID, Notification{Kind: "info", Template: template, Params: params}); err != nil {
		w.deps.Logger.ErrorContext(ctx, "notify", "template", template, "channel", params["channelId"], "error", err)
	}
}
