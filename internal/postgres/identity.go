package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users"`

	ID        uuid.UUID `bun:"id,pk"`
	Issuer    string    `bun:"issuer,notnull"`
	Subject   string    `bun:"subject,notnull"`
	Email     string    `bun:"email,notnull"`
	Name      string    `bun:"name,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
}

type Organization struct {
	bun.BaseModel `bun:"table:organizations"`

	ID        uuid.UUID `bun:"id,pk"`
	Name      string    `bun:"name,notnull"`
	CreatedAt time.Time `bun:"created_at,notnull"`
}

type membershipRow struct {
	bun.BaseModel `bun:"table:memberships"`

	OrganizationID uuid.UUID `bun:"organization_id,pk"`
	UserID         uuid.UUID `bun:"user_id,pk"`
	Role           string    `bun:"role,notnull"`
	CreatedAt      time.Time `bun:"created_at,notnull"`
}

// MembershipView is a membership with the name of its organization.
type MembershipView struct {
	OrganizationID   uuid.UUID `bun:"organization_id"`
	OrganizationName string    `bun:"organization_name"`
	Role             string    `bun:"role"`
	JoinedAt         time.Time `bun:"joined_at"`
}

type Identity struct {
	db *bun.DB
}

func NewIdentity(db *bun.DB) *Identity {
	return &Identity{db: db}
}

// FindUser returns nil when nobody has that issuer and subject.
func (s *Identity) FindUser(ctx context.Context, issuer, subject string) (*User, error) {
	var user User
	err := s.db.NewSelect().Model(&user).Where("issuer = ? AND subject = ?", issuer, subject).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Identity) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	var user User
	if err := s.db.NewSelect().Model(&user).Where("id = ?", id).Scan(ctx); err != nil {
		return nil, err
	}
	return &user, nil
}

// CreateUserWithOrganization stores a newcomer and the organization they own, atomically.
func (s *Identity) CreateUserWithOrganization(ctx context.Context, user User, org Organization) error {
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().Model(&user).Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewInsert().Model(&org).Exec(ctx); err != nil {
			return err
		}
		membership := membershipRow{OrganizationID: org.ID, UserID: user.ID, Role: "OWNER", CreatedAt: org.CreatedAt}
		_, err := tx.NewInsert().Model(&membership).Exec(ctx)
		return err
	})
}

// Memberships lists the organizations of a person in the order they joined them.
func (s *Identity) Memberships(ctx context.Context, userID uuid.UUID) ([]MembershipView, error) {
	var views []MembershipView
	err := s.db.NewSelect().
		TableExpr("memberships AS m").
		Join("JOIN organizations AS o ON o.id = m.organization_id").
		ColumnExpr("m.organization_id, o.name AS organization_name, m.role, m.created_at AS joined_at").
		Where("m.user_id = ?", userID).
		OrderExpr("m.created_at, m.organization_id").
		Scan(ctx, &views)
	return views, err
}
