package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strconv"

	"github.com/zetesis-labs/postik/internal/core/publish"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/storage"
	"github.com/zetesis-labs/postik/internal/telegram"
)

// telegramPublisher sends values with the Bot API (S05 §5). Nothing needs
// preparing: the first call is the point of no return.
type telegramPublisher struct {
	client *telegram.Client
	files  storage.Files
	logger *slog.Logger
}

func (p *telegramPublisher) Prepare(context.Context, *postgres.Post, int) (any, error) {
	return nil, nil
}

// ReplyTo: a comment answers the message of the previous value.
func (p *telegramPublisher) ReplyTo(index int) int { return index - 1 }

func (p *telegramPublisher) Create(ctx context.Context, post *postgres.Post, index int, _ any, replyTo *postgres.Delivery) (Sent, error) {
	var reply int64
	if replyTo != nil && replyTo.ExternalID != nil {
		id, err := strconv.ParseInt(*replyTo.ExternalID, 10, 64)
		if err != nil {
			return Sent{}, err
		}
		reply = id
	}
	messageID, err := p.sendValue(ctx, post, post.Values[index], reply)
	if err != nil {
		return Sent{}, err
	}
	return Sent{
		ID:  strconv.FormatInt(messageID, 10),
		URL: publish.TelegramURL(post.Channel.ExternalID, post.Channel.Username, messageID),
	}, nil
}

// sendValue makes the calls of S05 §5 and returns the ID of the first message.
func (p *telegramPublisher) sendValue(ctx context.Context, post *postgres.Post, value postgres.PostValue, replyTo int64) (int64, error) {
	if p.client == nil {
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
		id, err := p.call(ctx, chatID, call, reply)
		if err != nil {
			if i == 0 {
				return 0, err
			}
			p.logger.ErrorContext(ctx, "part of a value was not sent", "post", post.ID, "call", i, "error", err)
			return first, nil
		}
		if i == 0 {
			first = id
		}
	}
	return first, nil
}

func (p *telegramPublisher) call(ctx context.Context, chatID int64, call publish.Call, replyTo int64) (int64, error) {
	switch call.Method {
	case "sendMessage":
		m, err := p.client.SendMessage(ctx, chatID, call.Text, replyTo)
		return m.MessageID, err
	case "sendMediaGroup":
		files := make([]telegram.File, len(call.Media))
		for i, m := range call.Media {
			files[i] = p.file(m)
		}
		messages, err := p.client.SendMediaGroup(ctx, chatID, files, call.Text, replyTo)
		if err != nil || len(messages) == 0 {
			return 0, err
		}
		return messages[0].MessageID, nil
	default:
		m, err := p.client.SendFile(ctx, chatID, p.file(call.Media[0]), call.Text, replyTo)
		return m.MessageID, err
	}
}

func (p *telegramPublisher) file(m publish.Media) telegram.File {
	kind := "photo"
	if m.Kind == "video" {
		kind = "video"
	}
	return telegram.File{
		Type: kind,
		Name: path.Base(m.Path),
		Open: func() (io.ReadCloser, error) { return p.files.Open(m.Path) },
	}
}

// Classify turns an error from the Bot API into a publish.Failure.
func (p *telegramPublisher) Classify(err error) (publish.Failure, string) {
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
