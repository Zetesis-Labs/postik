package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Delivery records one attempt at sending a value of a post, keyed by the
// post's publication date so a republication starts afresh.
type Delivery struct {
	bun.BaseModel `bun:"table:post_deliveries"`

	PostID     uuid.UUID `bun:"post_id,pk"`
	PublishAt  time.Time `bun:"publish_at,pk"`
	ValueIndex int       `bun:"value_index,pk"`
	State      string    `bun:"state,notnull"`
	ExternalID *string   `bun:"external_id"`
	URL        *string   `bun:"url"`
	Error      *string   `bun:"error"`
	CreatedAt  time.Time `bun:"created_at,notnull"`
	UpdatedAt  time.Time `bun:"updated_at,notnull"`
}

// Publications is what the publishing jobs read and write.
type Publications struct {
	db *bun.DB
}

func NewPublications(db *bun.DB) *Publications {
	return &Publications{db: db}
}

// Post returns a post with its channel, whatever its organization.
func (s *Publications) Post(ctx context.Context, id uuid.UUID) (*Post, error) {
	var post Post
	err := s.db.NewSelect().Model(&post).Relation("Channel").Where("post.id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &post, err
}

// Delivery returns nil when the value has no recorded attempt.
func (s *Publications) Delivery(ctx context.Context, postID uuid.UUID, publishAt time.Time, index int) (*Delivery, error) {
	var d Delivery
	err := s.db.NewSelect().Model(&d).
		Where("post_id = ? AND publish_at = ? AND value_index = ?", postID, publishAt, index).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &d, err
}

// StartSending records that a value is about to be sent. It reports
// ErrConflict when another attempt already recorded it.
func (s *Publications) StartSending(ctx context.Context, postID uuid.UUID, publishAt time.Time, index int, now time.Time) error {
	d := Delivery{PostID: postID, PublishAt: publishAt, ValueIndex: index, State: "sending", CreatedAt: now, UpdatedAt: now}
	_, err := s.db.NewInsert().Model(&d).Exec(ctx)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// ForgetSending removes the mark of a send that never started, so a retry can send it.
func (s *Publications) ForgetSending(ctx context.Context, postID uuid.UUID, publishAt time.Time, index int) error {
	_, err := s.db.NewDelete().Model((*Delivery)(nil)).
		Where("post_id = ? AND publish_at = ? AND value_index = ? AND state = 'sending'", postID, publishAt, index).Exec(ctx)
	return err
}

func (s *Publications) MarkSent(ctx context.Context, postID uuid.UUID, publishAt time.Time, index int, externalID, url string, now time.Time) error {
	return s.settle(ctx, Delivery{PostID: postID, PublishAt: publishAt, ValueIndex: index, State: "sent", ExternalID: &externalID, URL: &url, CreatedAt: now, UpdatedAt: now})
}

func (s *Publications) MarkFailed(ctx context.Context, postID uuid.UUID, publishAt time.Time, index int, reason string, now time.Time) error {
	return s.settle(ctx, Delivery{PostID: postID, PublishAt: publishAt, ValueIndex: index, State: "failed", Error: &reason, CreatedAt: now, UpdatedAt: now})
}

func (s *Publications) settle(ctx context.Context, d Delivery) error {
	_, err := s.db.NewInsert().Model(&d).
		On("CONFLICT (post_id, publish_at, value_index) DO UPDATE").
		Set("state = EXCLUDED.state, external_id = EXCLUDED.external_id, url = EXCLUDED.url, error = EXCLUDED.error, updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// PublishPost marks a post published with its link, if it is still the
// scheduled post of that date.
func (s *Publications) PublishPost(ctx context.Context, postID uuid.UUID, publishAt time.Time, url string, now time.Time) error {
	_, err := s.db.NewUpdate().Model((*Post)(nil)).
		Set("status = 'published', release_url = ?, error = NULL, updated_at = ?", url, now).
		Where("id = ? AND publish_at = ? AND status = 'scheduled'", postID, publishAt).Exec(ctx)
	return err
}

// FailPost marks a post in error, if it is still the scheduled post of that date.
func (s *Publications) FailPost(ctx context.Context, postID uuid.UUID, publishAt time.Time, reason string, now time.Time) error {
	_, err := s.db.NewUpdate().Model((*Post)(nil)).
		Set("status = 'error', error = ?, updated_at = ?", reason, now).
		Where("id = ? AND publish_at = ? AND status = 'scheduled'", postID, publishAt).Exec(ctx)
	return err
}

// Overdue lists the scheduled posts due between from and to whose channel is
// healthy and that have no recorded attempt for their date (F13).
func (s *Publications) Overdue(ctx context.Context, from, to time.Time) ([]Post, error) {
	var posts []Post
	err := s.db.NewSelect().Model(&posts).Relation("Channel").
		Where("post.status = 'scheduled' AND post.publish_at >= ? AND post.publish_at <= ?", from, to).
		Where("NOT channel.disabled AND NOT channel.refresh_needed AND NOT channel.in_between_steps").
		Where("NOT EXISTS (SELECT 1 FROM post_deliveries AS d WHERE d.post_id = post.id AND d.publish_at = post.publish_at)").
		OrderExpr("post.publish_at").
		Scan(ctx)
	return posts, err
}
