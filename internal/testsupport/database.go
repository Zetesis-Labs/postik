package testsupport

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"

	"github.com/zetesis-labs/postik/internal/database"
	"github.com/zetesis-labs/postik/internal/database/migrations"
)

// NewEmptyDB creates a throwaway database and drops it when the test ends.
// Tests need a real Postgres: without TEST_DATABASE_URL they fail instead of skipping.
func NewEmptyDB(t testing.TB) *bun.DB {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Fatal("TEST_DATABASE_URL is not set; run the tests inside the devcontainer")
	}
	admin, err := sql.Open("pgx", adminURL)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	name := "postik_test_" + randomSuffix(t)
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
	})

	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	parsed.Path = "/" + name
	db, err := database.Open(parsed.String())
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// NewMigratedDB is NewEmptyDB with every migration applied.
func NewMigratedDB(t testing.TB) *bun.DB {
	t.Helper()
	db := NewEmptyDB(t)
	if err := migrations.Run(context.Background(), db.DB); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	return db
}

func randomSuffix(t testing.TB) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}
