package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type TelegramConnection struct {
	bun.BaseModel `bun:"table:telegram_connections"`

	Code           string     `bun:"code,pk"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull"`
	CreatedAt      time.Time  `bun:"created_at,notnull"`
	ChannelID      *uuid.UUID `bun:"channel_id"`
}

type telegramState struct {
	bun.BaseModel `bun:"table:telegram_state"`

	ID         int16 `bun:"id,pk"`
	NextOffset int64 `bun:"next_offset,notnull"`
}

type Telegram struct {
	db *bun.DB
}

func NewTelegram(db *bun.DB) *Telegram {
	return &Telegram{db: db}
}

// CreateConnection stores a pending connection. It drops the ones created
// before staleBefore and reports false if the code is already taken.
func (s *Telegram) CreateConnection(ctx context.Context, conn TelegramConnection, staleBefore time.Time) (bool, error) {
	if _, err := s.db.NewDelete().Model((*TelegramConnection)(nil)).Where("created_at < ?", staleBefore).Exec(ctx); err != nil {
		return false, err
	}
	res, err := s.db.NewInsert().Model(&conn).On("CONFLICT (code) DO NOTHING").Exec(ctx)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// Connection returns nil when there is no connection with that code.
func (s *Telegram) Connection(ctx context.Context, code string) (*TelegramConnection, error) {
	var conn TelegramConnection
	err := s.db.NewSelect().Model(&conn).Where("code = ?", code).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &conn, err
}

func (s *Telegram) MarkConnected(ctx context.Context, code string, channelID uuid.UUID) error {
	_, err := s.db.NewUpdate().Model((*TelegramConnection)(nil)).Set("channel_id = ?", channelID).Where("code = ?", code).Exec(ctx)
	return err
}

func (s *Telegram) NextOffset(ctx context.Context) (int64, error) {
	var state telegramState
	err := s.db.NewSelect().Model(&state).Where("id = 1").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return state.NextOffset, err
}

func (s *Telegram) SaveNextOffset(ctx context.Context, offset int64) error {
	state := telegramState{ID: 1, NextOffset: offset}
	_, err := s.db.NewInsert().Model(&state).On("CONFLICT (id) DO UPDATE").Set("next_offset = EXCLUDED.next_offset").Exec(ctx)
	return err
}
