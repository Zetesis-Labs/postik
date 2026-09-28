package jobs

import (
	"context"
	"errors"
	"net/http"
	"os"

	"github.com/zetesis-labs/postik/internal/core/publish"
	"github.com/zetesis-labs/postik/internal/linkedin"
	"github.com/zetesis-labs/postik/internal/postgres"
	"github.com/zetesis-labs/postik/internal/storage"
	"github.com/zetesis-labs/postik/internal/tokens"
)

// linkedInPublisher publishes as a member or as a page (S06 §6). Preparing
// renews the token when needed and uploads the media.
type linkedInPublisher struct {
	client *linkedin.Client
	tokens *tokens.Keeper
	files  storage.Files
}

type linkedInValue struct {
	token string
	media []string
}

// ReplyTo: comments on LinkedIn do not nest, they all answer the post.
func (p *linkedInPublisher) ReplyTo(int) int { return 0 }

func author(channel *postgres.Channel) string {
	if channel.Provider == "linkedin-page" {
		return "urn:li:organization:" + channel.ExternalID
	}
	return "urn:li:person:" + channel.ExternalID
}

func (p *linkedInPublisher) Prepare(ctx context.Context, post *postgres.Post, index int) (any, error) {
	t, err := p.tokens.Current(ctx, *post.Channel)
	if err != nil {
		return nil, err
	}
	prepared := &linkedInValue{token: t.Access}
	owner := author(post.Channel)
	for _, m := range post.Values[index].Media {
		file := linkedin.File{Open: func() (*os.File, error) { return p.files.Open(m.URL) }}
		var urn string
		err := p.withRenewal(ctx, post.Channel, &prepared.token, func(token string) (err error) {
			if m.Kind == "video" {
				urn, err = p.client.UploadVideo(ctx, token, owner, file)
			} else {
				urn, err = p.client.UploadImage(ctx, token, owner, file)
			}
			return err
		})
		if err != nil {
			return nil, err
		}
		prepared.media = append(prepared.media, urn)
	}
	return prepared, nil
}

func (p *linkedInPublisher) Create(ctx context.Context, post *postgres.Post, index int, prepared any, replyTo *postgres.Delivery) (Sent, error) {
	value := prepared.(*linkedInValue)
	commentary := publish.LinkedInCommentary(post.Values[index].Content)
	actor := author(post.Channel)
	if index == 0 {
		var urn string
		err := p.withRenewal(ctx, post.Channel, &value.token, func(token string) (err error) {
			urn, err = p.client.CreatePost(ctx, token, linkedin.Post{Author: actor, Commentary: commentary, Content: linkedin.Content{Media: value.media}})
			return err
		})
		return Sent{ID: urn, URL: publish.LinkedInURL(urn)}, err
	}
	if replyTo == nil || replyTo.ExternalID == nil {
		return Sent{}, errors.New("the post to comment on was not published")
	}
	root := *replyTo.ExternalID
	var id string
	err := p.withRenewal(ctx, post.Channel, &value.token, func(token string) (err error) {
		id, err = p.client.CreateComment(ctx, token, root, actor, commentary)
		return err
	})
	return Sent{ID: id, URL: publish.LinkedInURL(root)}, err
}

// withRenewal runs call and, if LinkedIn rejects the token, renews it and
// runs call once more (F11, step 4). A 401 means LinkedIn did nothing.
func (p *linkedInPublisher) withRenewal(ctx context.Context, channel *postgres.Channel, token *string, call func(token string) error) error {
	err := call(*token)
	var apiErr *linkedin.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		return err
	}
	renewed, err := p.tokens.Renew(ctx, *channel, *token)
	if err != nil {
		return err
	}
	*token = renewed.Access
	err = call(*token)
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
		return tokens.ErrCannotRenew
	}
	return err
}

func (p *linkedInPublisher) Classify(err error) (publish.Failure, string) {
	var notSent *linkedin.NotSentError
	if errors.As(err, &notSent) {
		return publish.NotStarted, publish.CodeUnreachable
	}
	var apiErr *linkedin.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.Status == http.StatusTooManyRequests:
			return publish.RateLimited, apiErr.Message
		case apiErr.Status >= http.StatusInternalServerError:
			return publish.Unknown, apiErr.Message
		}
		return publish.Rejected, apiErr.Message
	}
	return publish.Unknown, err.Error()
}
