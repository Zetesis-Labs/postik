package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

type Customer struct {
	bun.BaseModel `bun:"table:customers"`

	ID             uuid.UUID `bun:"id,pk"`
	OrganizationID uuid.UUID `bun:"organization_id,notnull"`
	Name           string    `bun:"name,notnull"`
	CreatedAt      time.Time `bun:"created_at,notnull"`
}

type Channel struct {
	bun.BaseModel `bun:"table:channels"`

	ID             uuid.UUID  `bun:"id,pk"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull"`
	Provider       string     `bun:"provider,notnull"`
	ExternalID     string     `bun:"external_id,notnull"`
	Name           string     `bun:"name,notnull"`
	Username       string     `bun:"username,notnull"`
	Picture        *string    `bun:"picture"`
	Disabled       bool       `bun:"disabled,notnull"`
	RefreshNeeded  bool       `bun:"refresh_needed,notnull"`
	InBetweenSteps bool       `bun:"in_between_steps,notnull"`
	CustomerID     *uuid.UUID `bun:"customer_id"`
	PostingTimes   []int      `bun:"posting_times,array,notnull"`
	CreatedAt      time.Time  `bun:"created_at,notnull"`
	UpdatedAt      time.Time  `bun:"updated_at,notnull"`

	Customer *Customer `bun:"rel:belongs-to,join:customer_id=id"`
}

var ErrNotFound = errors.New("not found")

type Channels struct {
	db *bun.DB
}

func NewChannels(db *bun.DB) *Channels {
	return &Channels{db: db}
}

// List returns the channels of an organization with their customer.
func (s *Channels) List(ctx context.Context, orgID uuid.UUID) ([]Channel, error) {
	var channels []Channel
	err := s.db.NewSelect().Model(&channels).Relation("Customer").
		Where("channel.organization_id = ?", orgID).
		OrderExpr("channel.created_at, channel.id").
		Scan(ctx)
	return channels, err
}

// Get returns ErrNotFound when the channel does not exist in the organization.
func (s *Channels) Get(ctx context.Context, orgID, id uuid.UUID) (*Channel, error) {
	var channel Channel
	err := s.db.NewSelect().Model(&channel).Relation("Customer").
		Where("channel.organization_id = ? AND channel.id = ?", orgID, id).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &channel, err
}

// FindByExternal returns nil when the organization has no channel for that account.
func (s *Channels) FindByExternal(ctx context.Context, orgID uuid.UUID, provider, externalID string) (*Channel, error) {
	var channel Channel
	err := s.db.NewSelect().Model(&channel).
		Where("organization_id = ? AND provider = ? AND external_id = ?", orgID, provider, externalID).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &channel, err
}

// Save inserts the channel or updates every column of an existing one.
func (s *Channels) Save(ctx context.Context, channel *Channel) error {
	_, err := s.db.NewInsert().Model(channel).
		On("CONFLICT (id) DO UPDATE").
		Set("name = EXCLUDED.name, username = EXCLUDED.username, picture = EXCLUDED.picture, disabled = EXCLUDED.disabled, refresh_needed = EXCLUDED.refresh_needed, in_between_steps = EXCLUDED.in_between_steps, customer_id = EXCLUDED.customer_id, posting_times = EXCLUDED.posting_times, updated_at = EXCLUDED.updated_at").
		Exec(ctx)
	return err
}

// update changes columns of a channel of the organization and reports
// ErrNotFound when there is none.
func (s *Channels) update(ctx context.Context, orgID, id uuid.UUID, now time.Time, set string, args ...any) error {
	res, err := s.db.NewUpdate().Model((*Channel)(nil)).
		Set(set, args...).
		Set("updated_at = ?", now).
		Where("organization_id = ? AND id = ?", orgID, id).
		Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Channels) SetDisabled(ctx context.Context, orgID, id uuid.UUID, disabled bool, now time.Time) error {
	return s.update(ctx, orgID, id, now, "disabled = ?", disabled)
}

func (s *Channels) SetPostingTimes(ctx context.Context, orgID, id uuid.UUID, times []int, now time.Time) error {
	return s.update(ctx, orgID, id, now, "posting_times = ?", pgdialect.Array(times))
}

func (s *Channels) SetCustomer(ctx context.Context, orgID, id uuid.UUID, customerID *uuid.UUID, now time.Time) error {
	return s.update(ctx, orgID, id, now, "customer_id = ?", customerID)
}

func (s *Channels) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	res, err := s.db.NewDelete().Model((*Channel)(nil)).Where("organization_id = ? AND id = ?", orgID, id).Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ExistsInOrganization reports whether the channel belongs to the organization.
func (s *Channels) ExistsInOrganization(ctx context.Context, orgID, id uuid.UUID) (bool, error) {
	return s.db.NewSelect().Model((*Channel)(nil)).Where("organization_id = ? AND id = ?", orgID, id).Exists(ctx)
}

func (s *Channels) Customers(ctx context.Context, orgID uuid.UUID) ([]Customer, error) {
	var customers []Customer
	err := s.db.NewSelect().Model(&customers).Where("organization_id = ?", orgID).OrderExpr("name").Scan(ctx)
	return customers, err
}

// CustomerByID returns ErrNotFound when the customer is not in the organization.
func (s *Channels) CustomerByID(ctx context.Context, orgID, id uuid.UUID) (*Customer, error) {
	var customer Customer
	err := s.db.NewSelect().Model(&customer).Where("organization_id = ? AND id = ?", orgID, id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &customer, err
}

// CustomerByName returns the customer with that name, creating it if needed.
func (s *Channels) CustomerByName(ctx context.Context, orgID uuid.UUID, name string, now time.Time) (*Customer, error) {
	customer := Customer{ID: uuid.New(), OrganizationID: orgID, Name: name, CreatedAt: now}
	_, err := s.db.NewInsert().Model(&customer).On("CONFLICT (organization_id, name) DO NOTHING").Exec(ctx)
	if err != nil {
		return nil, err
	}
	var existing Customer
	err = s.db.NewSelect().Model(&existing).Where("organization_id = ? AND name = ?", orgID, name).Scan(ctx)
	return &existing, err
}
