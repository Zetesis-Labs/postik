package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/uptrace/bun"
)

var ErrConflict = errors.New("conflict")

type Tag struct {
	bun.BaseModel `bun:"table:tags"`

	ID             uuid.UUID `bun:"id,pk"`
	OrganizationID uuid.UUID `bun:"organization_id,notnull"`
	Name           string    `bun:"name,notnull"`
	Color          string    `bun:"color,notnull"`
	CreatedAt      time.Time `bun:"created_at,notnull"`
}

type PostGroup struct {
	bun.BaseModel `bun:"table:post_groups"`

	ID             uuid.UUID  `bun:"id,pk"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull"`
	Origin         string     `bun:"origin,notnull"`
	CreatedBy      *uuid.UUID `bun:"created_by"`
	CreatedAt      time.Time  `bun:"created_at,notnull"`
}

type postGroupTag struct {
	bun.BaseModel `bun:"table:post_group_tags"`

	GroupID uuid.UUID `bun:"group_id,pk"`
	TagID   uuid.UUID `bun:"tag_id,pk"`
}

type PostMedia struct {
	ID               uuid.UUID `json:"id"`
	URL              string    `json:"url"`
	Kind             string    `json:"kind"`
	Alt              string    `json:"alt,omitempty"`
	ThumbnailURL     *string   `json:"thumbnailUrl,omitempty"`
	ThumbnailSeconds *int      `json:"thumbnailSeconds,omitempty"`
}

type PostValue struct {
	Content      string      `json:"content"`
	DelayMinutes int         `json:"delayMinutes"`
	Media        []PostMedia `json:"media"`
}

type Post struct {
	bun.BaseModel `bun:"table:posts"`

	ID             uuid.UUID      `bun:"id,pk"`
	OrganizationID uuid.UUID      `bun:"organization_id,notnull"`
	GroupID        uuid.UUID      `bun:"group_id,notnull"`
	ChannelID      uuid.UUID      `bun:"channel_id,notnull"`
	Status         string         `bun:"status,notnull"`
	PublishAt      time.Time      `bun:"publish_at,notnull"`
	Values         []PostValue    `bun:"post_values,type:jsonb,notnull"`
	Settings       map[string]any `bun:"settings,type:jsonb,notnull"`
	ReleaseURL     *string        `bun:"release_url"`
	Error          *string        `bun:"error"`
	CreatedAt      time.Time      `bun:"created_at,notnull"`
	UpdatedAt      time.Time      `bun:"updated_at,notnull"`

	Channel *Channel `bun:"rel:belongs-to,join:channel_id=id"`
}

// Scheduler queues the publication of a scheduled post in the transaction
// that saves it.
type Scheduler interface {
	SchedulePost(ctx context.Context, tx *sql.Tx, postID uuid.UUID, publishAt time.Time) error
}

type Posts struct {
	db        *bun.DB
	Scheduler Scheduler
}

func NewPosts(db *bun.DB) *Posts {
	return &Posts{db: db}
}

func (s *Posts) schedule(ctx context.Context, tx bun.Tx, post *Post) error {
	if s.Scheduler == nil || post.Status != "scheduled" {
		return nil
	}
	return s.Scheduler.SchedulePost(ctx, tx.Tx, post.ID, post.PublishAt)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Posts) Tags(ctx context.Context, orgID uuid.UUID) ([]Tag, error) {
	var tags []Tag
	err := s.db.NewSelect().Model(&tags).Where("organization_id = ?", orgID).OrderExpr("name").Scan(ctx)
	return tags, err
}

// CreateTag reports ErrConflict when the name is taken in the organization.
func (s *Posts) CreateTag(ctx context.Context, tag *Tag) error {
	_, err := s.db.NewInsert().Model(tag).Exec(ctx)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (s *Posts) UpdateTag(ctx context.Context, tag *Tag) error {
	res, err := s.db.NewUpdate().Model(tag).Column("name", "color").
		Where("organization_id = ? AND id = ?", tag.OrganizationID, tag.ID).Exec(ctx)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Posts) DeleteTag(ctx context.Context, orgID, id uuid.UUID) error {
	res, err := s.db.NewDelete().Model((*Tag)(nil)).Where("organization_id = ? AND id = ?", orgID, id).Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TagsByID returns the tags of the organization among ids.
func (s *Posts) TagsByID(ctx context.Context, orgID uuid.UUID, ids []uuid.UUID) ([]Tag, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var tags []Tag
	err := s.db.NewSelect().Model(&tags).Where("organization_id = ? AND id IN (?)", orgID, bun.List(ids)).Scan(ctx)
	return tags, err
}

// CreateGroup stores a group, its tags and its posts atomically.
func (s *Posts) CreateGroup(ctx context.Context, group PostGroup, tagIDs []uuid.UUID, posts []Post) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&group).Exec(ctx); err != nil {
			return err
		}
		if err := setGroupTags(ctx, tx, group.ID, tagIDs); err != nil {
			return err
		}
		if _, err := tx.NewInsert().Model(&posts).Exec(ctx); err != nil {
			return err
		}
		for i := range posts {
			if err := s.schedule(ctx, tx, &posts[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

func setGroupTags(ctx context.Context, tx bun.Tx, groupID uuid.UUID, tagIDs []uuid.UUID) error {
	if _, err := tx.NewDelete().Model((*postGroupTag)(nil)).Where("group_id = ?", groupID).Exec(ctx); err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}
	rows := make([]postGroupTag, len(tagIDs))
	for i, id := range tagIDs {
		rows[i] = postGroupTag{GroupID: groupID, TagID: id}
	}
	_, err := tx.NewInsert().Model(&rows).Exec(ctx)
	return err
}

// Get returns ErrNotFound for posts of another organization.
func (s *Posts) Get(ctx context.Context, orgID, id uuid.UUID) (*Post, error) {
	var post Post
	err := s.db.NewSelect().Model(&post).Relation("Channel").
		Where("post.organization_id = ? AND post.id = ?", orgID, id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &post, err
}

// GroupTags returns the tags of each group.
func (s *Posts) GroupTags(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]Tag, error) {
	out := map[uuid.UUID][]Tag{}
	if len(groupIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		GroupID uuid.UUID `bun:"group_id"`
		Tag
	}
	err := s.db.NewSelect().
		TableExpr("post_group_tags AS pgt").
		Join("JOIN tags AS tag ON tag.id = pgt.tag_id").
		ColumnExpr("pgt.group_id, tag.*").
		Where("pgt.group_id IN (?)", bun.List(groupIDs)).
		OrderExpr("tag.name").
		Scan(ctx, &rows)
	for _, r := range rows {
		out[r.GroupID] = append(out[r.GroupID], r.Tag)
	}
	return out, err
}

// Range lists the posts of the organization between from (included) and to
// (excluded), optionally only in the channels of a customer.
func (s *Posts) Range(ctx context.Context, orgID uuid.UUID, from, to time.Time, customerID *uuid.UUID) ([]Post, error) {
	var posts []Post
	q := s.db.NewSelect().Model(&posts).Relation("Channel").
		Where("post.organization_id = ? AND post.publish_at >= ? AND post.publish_at < ?", orgID, from, to).
		OrderExpr("post.publish_at, post.id")
	if customerID != nil {
		q = q.Where("channel.customer_id = ?", *customerID)
	}
	return posts, q.Scan(ctx)
}

// Page lists posts newest first, optionally of one status.
func (s *Posts) Page(ctx context.Context, orgID uuid.UUID, status string, page, size int) ([]Post, int, error) {
	var posts []Post
	q := s.db.NewSelect().Model(&posts).Relation("Channel").
		Where("post.organization_id = ?", orgID).
		OrderExpr("post.publish_at DESC, post.id").
		Limit(size).Offset((page - 1) * size)
	if status != "" {
		q = q.Where("post.status = ?", status)
	}
	total, err := q.ScanAndCount(ctx)
	return posts, total, err
}

// Save updates a post and the tags of its group.
func (s *Posts) Save(ctx context.Context, post *Post, tagIDs []uuid.UUID) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model(post).Column("status", "publish_at", "post_values", "settings", "updated_at").WherePK().Exec(ctx); err != nil {
			return err
		}
		if err := setGroupTags(ctx, tx, post.GroupID, tagIDs); err != nil {
			return err
		}
		return s.schedule(ctx, tx, post)
	})
}

func (s *Posts) Move(ctx context.Context, post *Post) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewUpdate().Model(post).Column("status", "publish_at", "updated_at").WherePK().Exec(ctx); err != nil {
			return err
		}
		return s.schedule(ctx, tx, post)
	})
}

// SetReleaseURL links a published post that has no link. It reports
// ErrConflict when the post is not published or already has one.
func (s *Posts) SetReleaseURL(ctx context.Context, orgID, id uuid.UUID, url string, now time.Time) error {
	res, err := s.db.NewUpdate().Model((*Post)(nil)).
		Set("release_url = ?, updated_at = ?", url, now).
		Where("organization_id = ? AND id = ? AND status = 'published' AND release_url IS NULL", orgID, id).Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	if _, err := s.Get(ctx, orgID, id); err != nil {
		return err
	}
	return ErrConflict
}

func (s *Posts) DeleteGroup(ctx context.Context, orgID, groupID uuid.UUID) error {
	_, err := s.db.NewDelete().Model((*PostGroup)(nil)).Where("organization_id = ? AND id = ?", orgID, groupID).Exec(ctx)
	return err
}

// Taken returns the minutes that already have a post of the organization.
func (s *Posts) Taken(ctx context.Context, orgID uuid.UUID, from, to time.Time) (map[time.Time]bool, error) {
	var dates []time.Time
	err := s.db.NewSelect().Model((*Post)(nil)).Column("publish_at").
		Where("organization_id = ? AND publish_at >= ? AND publish_at < ?", orgID, from, to).
		Scan(ctx, &dates)
	taken := make(map[time.Time]bool, len(dates))
	for _, d := range dates {
		taken[d.UTC().Truncate(time.Minute)] = true
	}
	return taken, err
}
