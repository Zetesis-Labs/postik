package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/uptrace/bun"
)

type OIDCLogin struct {
	bun.BaseModel `bun:"table:oidc_logins"`

	StateHash    []byte    `bun:"state_hash,pk"`
	Nonce        string    `bun:"nonce,notnull"`
	CodeVerifier string    `bun:"code_verifier,notnull"`
	CreatedAt    time.Time `bun:"created_at,notnull"`
}

type OIDCLogins struct {
	db bun.IDB
}

func NewOIDCLogins(db bun.IDB) *OIDCLogins {
	return &OIDCLogins{db: db}
}

// Create stores a login attempt and drops the ones started before staleBefore.
func (s *OIDCLogins) Create(ctx context.Context, login OIDCLogin, staleBefore time.Time) error {
	if _, err := s.db.NewDelete().Model((*OIDCLogin)(nil)).Where("created_at < ?", staleBefore).Exec(ctx); err != nil {
		return err
	}
	_, err := s.db.NewInsert().Model(&login).Exec(ctx)
	return err
}

// Take deletes the attempt and returns it, so it can be used only once. It
// returns nil when it does not exist.
func (s *OIDCLogins) Take(ctx context.Context, stateHash []byte) (*OIDCLogin, error) {
	var login OIDCLogin
	err := s.db.NewDelete().Model(&login).Where("state_hash = ?", stateHash).Returning("*").Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &login, nil
}
