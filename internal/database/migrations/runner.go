package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
)

// RiverSchema holds River's tables. Atlas only manages public.
const RiverSchema = "river"

//go:embed *.sql
var files embed.FS

// Any constant works: it only has to be the same for every postik process.
const advisoryLockKey = 7_104_115_116

// Run applies, in name order, every migration that is not yet recorded in
// schema_migrations, each in its own transaction, and then River's own.
func Run(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", advisoryLockKey)

	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		if err := apply(ctx, conn, version, name); err != nil {
			return fmt.Errorf("migration %s: %w", version, err)
		}
	}
	return migrateRiver(ctx, db)
}

func migrateRiver(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+RiverSchema); err != nil {
		return fmt.Errorf("create schema %s: %w", RiverSchema, err)
	}
	migrator, err := rivermigrate.New(riverdatabasesql.New(db), &rivermigrate.Config{Schema: RiverSchema})
	if err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrations: %w", err)
	}
	return nil
}

func apply(ctx context.Context, conn *sql.Conn, version, name string) error {
	var applied bool
	if err := conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&applied); err != nil {
		return err
	}
	if applied {
		return nil
	}
	body, err := files.ReadFile(name)
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit()
}
