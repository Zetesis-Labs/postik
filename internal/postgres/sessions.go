package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/zetesis-labs/postik/internal/core/access"
)

type sessionRow struct {
	bun.BaseModel `bun:"table:sessions"`

	IDHash     []byte     `bun:"id_hash,pk"`
	Kind       string     `bun:"kind,notnull"`
	UserID     *uuid.UUID `bun:"user_id"`
	CreatedAt  time.Time  `bun:"created_at,notnull"`
	LastSeenAt time.Time  `bun:"last_seen_at,notnull"`
}

type Session struct {
	IDHash []byte
	UserID *uuid.UUID
	access.SessionTimes
}

type Sessions struct {
	db bun.IDB
}

func NewSessions(db bun.IDB) *Sessions {
	return &Sessions{db: db}
}

func (s *Sessions) Create(ctx context.Context, session Session) error {
	row := sessionRow{
		IDHash:     session.IDHash,
		Kind:       string(session.Kind),
		UserID:     session.UserID,
		CreatedAt:  session.CreatedAt,
		LastSeenAt: session.LastSeenAt,
	}
	_, err := s.db.NewInsert().Model(&row).Exec(ctx)
	return err
}

// Get returns nil when the session does not exist.
func (s *Sessions) Get(ctx context.Context, idHash []byte) (*Session, error) {
	var row sessionRow
	err := s.db.NewSelect().Model(&row).Where("id_hash = ?", idHash).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &Session{
		IDHash: row.IDHash,
		UserID: row.UserID,
		SessionTimes: access.SessionTimes{
			Kind:       access.SessionKind(row.Kind),
			CreatedAt:  row.CreatedAt,
			LastSeenAt: row.LastSeenAt,
		},
	}, nil
}

func (s *Sessions) Touch(ctx context.Context, idHash []byte, at time.Time) error {
	_, err := s.db.NewUpdate().Model((*sessionRow)(nil)).
		Set("last_seen_at = ?", at).
		Where("id_hash = ?", idHash).
		Exec(ctx)
	return err
}

func (s *Sessions) Delete(ctx context.Context, idHash []byte) error {
	_, err := s.db.NewDelete().Model((*sessionRow)(nil)).Where("id_hash = ?", idHash).Exec(ctx)
	return err
}
