package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Notification struct {
	bun.BaseModel `bun:"table:notifications"`

	ID             uuid.UUID         `bun:"id,pk"`
	OrganizationID uuid.UUID         `bun:"organization_id,notnull"`
	Kind           string            `bun:"kind,notnull"`
	Template       string            `bun:"template,notnull"`
	Params         map[string]string `bun:"params,type:jsonb,notnull"`
	CreatedAt      time.Time         `bun:"created_at,notnull"`
	DigestedAt     *time.Time        `bun:"digested_at"`
}

// Recipient is a member of an organization and the emails they want.
type Recipient struct {
	UserID       uuid.UUID `bun:"id"`
	Email        string    `bun:"email"`
	EmailSuccess bool      `bun:"email_success"`
	EmailFailure bool      `bun:"email_failure"`
}

type Preferences struct {
	EmailSuccess bool `bun:"email_success"`
	EmailFailure bool `bun:"email_failure"`
}

type Notifications struct {
	db *bun.DB
}

func NewNotifications(db *bun.DB) *Notifications {
	return &Notifications{db: db}
}

func (s *Notifications) Insert(ctx context.Context, n *Notification) error {
	_, err := s.db.NewInsert().Model(n).Exec(ctx)
	return err
}

// Recipients lists the members of an organization with their email preferences.
func (s *Notifications) Recipients(ctx context.Context, orgID uuid.UUID) ([]Recipient, error) {
	var out []Recipient
	err := s.db.NewSelect().
		TableExpr("memberships AS m").
		Join("JOIN users AS u ON u.id = m.user_id").
		ColumnExpr("u.id, u.email, u.email_success, u.email_failure").
		Where("m.organization_id = ?", orgID).
		OrderExpr("u.email").
		Scan(ctx, &out)
	return out, err
}

// Latest returns the newest notifications of an organization.
func (s *Notifications) Latest(ctx context.Context, orgID uuid.UUID, limit int) ([]Notification, error) {
	var out []Notification
	err := s.db.NewSelect().Model(&out).Where("organization_id = ?", orgID).
		OrderExpr("created_at DESC, id").Limit(limit).Scan(ctx)
	return out, err
}

// ReadAt is when the user last opened the notifications, or nil if never.
func (s *Notifications) ReadAt(ctx context.Context, userID uuid.UUID) (*time.Time, error) {
	var readAt *time.Time
	err := s.db.NewSelect().TableExpr("users").Column("notifications_read_at").Where("id = ?", userID).Scan(ctx, &readAt)
	return readAt, err
}

// Unread counts the notifications of an organization created after since (all when nil).
func (s *Notifications) Unread(ctx context.Context, orgID uuid.UUID, since *time.Time) (int, error) {
	q := s.db.NewSelect().Model((*Notification)(nil)).Where("organization_id = ?", orgID)
	if since != nil {
		q = q.Where("created_at > ?", *since)
	}
	return q.Count(ctx)
}

func (s *Notifications) MarkRead(ctx context.Context, userID uuid.UUID, now time.Time) error {
	_, err := s.db.NewUpdate().TableExpr("users").Set("notifications_read_at = ?", now).Where("id = ?", userID).Exec(ctx)
	return err
}

// Undigested lists the successes not yet sent in a summary, oldest first.
func (s *Notifications) Undigested(ctx context.Context) ([]Notification, error) {
	var out []Notification
	err := s.db.NewSelect().Model(&out).Where("kind = 'success' AND digested_at IS NULL").
		OrderExpr("organization_id, created_at, id").Scan(ctx)
	return out, err
}

func (s *Notifications) MarkDigested(ctx context.Context, ids []uuid.UUID, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := s.db.NewUpdate().Model((*Notification)(nil)).Set("digested_at = ?", now).Where("id IN (?)", bun.List(ids)).Exec(ctx)
	return err
}

func (s *Notifications) Preferences(ctx context.Context, userID uuid.UUID) (Preferences, error) {
	var p Preferences
	err := s.db.NewSelect().TableExpr("users").Column("email_success", "email_failure").Where("id = ?", userID).Scan(ctx, &p)
	return p, err
}

func (s *Notifications) SetPreferences(ctx context.Context, userID uuid.UUID, p Preferences) error {
	_, err := s.db.NewUpdate().TableExpr("users").
		Set("email_success = ?, email_failure = ?", p.EmailSuccess, p.EmailFailure).
		Where("id = ?", userID).Exec(ctx)
	return err
}
