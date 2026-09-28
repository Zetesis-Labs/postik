package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// Credential holds the sealed tokens of a channel.
type Credential struct {
	bun.BaseModel `bun:"table:channel_credentials"`

	ChannelID        uuid.UUID  `bun:"channel_id,pk"`
	AccessToken      []byte     `bun:"access_token,notnull"`
	AccessSecret     []byte     `bun:"access_secret"`
	RefreshToken     []byte     `bun:"refresh_token"`
	ExpiresAt        *time.Time `bun:"expires_at"`
	RefreshExpiresAt *time.Time `bun:"refresh_expires_at"`
	ExpiryWarnedFor  *time.Time `bun:"expiry_warned_for"`
	UpdatedAt        time.Time  `bun:"updated_at,notnull"`
}

// ExpiringChannel is a channel whose token expires and cannot be renewed.
type ExpiringChannel struct {
	Credential
	OrganizationID uuid.UUID `bun:"organization_id"`
	Provider       string    `bun:"provider"`
	Name           string    `bun:"name"`
	RefreshNeeded  bool      `bun:"refresh_needed"`
}

type Credentials struct {
	db *bun.DB
}

func NewCredentials(db *bun.DB) *Credentials {
	return &Credentials{db: db}
}

// Get returns nil when the channel has no credentials.
func (s *Credentials) Get(ctx context.Context, channelID uuid.UUID) (*Credential, error) {
	var c Credential
	err := s.db.NewSelect().Model(&c).Where("channel_id = ?", channelID).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Save replaces the credentials of a channel. New tokens start the expiry
// warning over.
func (s *Credentials) Save(ctx context.Context, c *Credential) error {
	return save(ctx, s.db, c)
}

func save(ctx context.Context, db bun.IDB, c *Credential) error {
	c.ExpiryWarnedFor = nil
	_, err := db.NewInsert().Model(c).
		On("CONFLICT (channel_id) DO UPDATE").
		Set("access_token = EXCLUDED.access_token, access_secret = EXCLUDED.access_secret, refresh_token = EXCLUDED.refresh_token, expires_at = EXCLUDED.expires_at, refresh_expires_at = EXCLUDED.refresh_expires_at, expiry_warned_for = NULL, updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// Update locks the credentials of a channel while renew decides on them. When
// renew returns new credentials they replace the old ones.
func (s *Credentials) Update(ctx context.Context, channelID uuid.UUID, renew func(current *Credential) (*Credential, error)) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var current Credential
		err := tx.NewSelect().Model(&current).Where("channel_id = ?", channelID).For("UPDATE").Scan(ctx)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		next, err := renew(&current)
		if err != nil || next == nil {
			return err
		}
		return save(ctx, tx, next)
	})
}

// Expiring lists the channels whose token expires and has no refresh token.
func (s *Credentials) Expiring(ctx context.Context) ([]ExpiringChannel, error) {
	var out []ExpiringChannel
	err := s.db.NewSelect().
		TableExpr("channel_credentials AS cc").
		Join("JOIN channels AS c ON c.id = cc.channel_id").
		ColumnExpr("cc.*, c.organization_id, c.provider, c.name, c.refresh_needed").
		Where("cc.refresh_token IS NULL AND cc.expires_at IS NOT NULL").
		OrderExpr("cc.expires_at").
		Scan(ctx, &out)
	return out, err
}

func (s *Credentials) MarkWarned(ctx context.Context, channelID uuid.UUID, expiresAt time.Time) error {
	_, err := s.db.NewUpdate().Model((*Credential)(nil)).Set("expiry_warned_for = ?", expiresAt).Where("channel_id = ?", channelID).Exec(ctx)
	return err
}
