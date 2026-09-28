package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// OAuthAuthorization is a connection started with a network that waits for
// the browser to come back (S06 §3).
type OAuthAuthorization struct {
	bun.BaseModel `bun:"table:oauth_authorizations"`

	State          string     `bun:"state,pk"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull"`
	UserID         uuid.UUID  `bun:"user_id,notnull"`
	Provider       string     `bun:"provider,notnull"`
	ChannelID      *uuid.UUID `bun:"channel_id"`
	Secret         []byte     `bun:"secret"`
	CreatedAt      time.Time  `bun:"created_at,notnull"`
}

type OAuth struct {
	db *bun.DB
}

func NewOAuth(db *bun.DB) *OAuth {
	return &OAuth{db: db}
}

// CreateAuthorization stores a new authorization and forgets the ones
// created before staleBefore.
func (s *OAuth) CreateAuthorization(ctx context.Context, a OAuthAuthorization, staleBefore time.Time) error {
	if _, err := s.db.NewDelete().Model((*OAuthAuthorization)(nil)).Where("created_at < ?", staleBefore).Exec(ctx); err != nil {
		return err
	}
	_, err := s.db.NewInsert().Model(&a).Exec(ctx)
	return err
}

// TakeAuthorization removes an authorization and returns it, so it can be
// used only once. It returns nil when there is none.
func (s *OAuth) TakeAuthorization(ctx context.Context, state string) (*OAuthAuthorization, error) {
	var a OAuthAuthorization
	err := s.db.NewDelete().Model(&a).Where("state = ?", state).Returning("*").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
