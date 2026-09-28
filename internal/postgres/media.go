package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Media struct {
	bun.BaseModel `bun:"table:media"`

	ID               uuid.UUID  `bun:"id,pk"`
	OrganizationID   uuid.UUID  `bun:"organization_id,notnull"`
	Name             string     `bun:"name,notnull"`
	Path             string     `bun:"path,notnull"`
	Kind             string     `bun:"kind,notnull"`
	Mime             string     `bun:"mime,notnull"`
	Size             int64      `bun:"size,notnull"`
	Alt              string     `bun:"alt,notnull"`
	ThumbnailPath    *string    `bun:"thumbnail_path"`
	ThumbnailSeconds *int       `bun:"thumbnail_seconds"`
	CreatedAt        time.Time  `bun:"created_at,notnull"`
	DeletedAt        *time.Time `bun:"deleted_at"`
}

type MediaStore struct {
	db *bun.DB
}

func NewMediaStore(db *bun.DB) *MediaStore {
	return &MediaStore{db: db}
}

func (s *MediaStore) Create(ctx context.Context, m *Media) error {
	_, err := s.db.NewInsert().Model(m).Exec(ctx)
	return err
}

// List returns one page of the library, newest first, and the total count.
func (s *MediaStore) List(ctx context.Context, orgID uuid.UUID, search string, page, pageSize int) ([]Media, int, error) {
	var items []Media
	q := s.db.NewSelect().Model(&items).
		Where("organization_id = ? AND deleted_at IS NULL", orgID).
		OrderExpr("created_at DESC, id DESC").
		Limit(pageSize).
		Offset((page - 1) * pageSize)
	if search = strings.TrimSpace(search); search != "" {
		q = q.Where("name ILIKE ?", "%"+escapeLike(search)+"%")
	}
	total, err := q.ScanAndCount(ctx)
	return items, total, err
}

// Get returns ErrNotFound for media of another organization or deleted.
func (s *MediaStore) Get(ctx context.Context, orgID, id uuid.UUID) (*Media, error) {
	var m Media
	err := s.db.NewSelect().Model(&m).Where("organization_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

func (s *MediaStore) Update(ctx context.Context, m *Media) error {
	_, err := s.db.NewUpdate().Model(m).Column("alt", "thumbnail_path", "thumbnail_seconds").WherePK().Exec(ctx)
	return err
}

func (s *MediaStore) SoftDelete(ctx context.Context, orgID, id uuid.UUID, now time.Time) error {
	res, err := s.db.NewUpdate().Model((*Media)(nil)).
		Set("deleted_at = ?", now).
		Where("organization_id = ? AND id = ? AND deleted_at IS NULL", orgID, id).
		Exec(ctx)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
