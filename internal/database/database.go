package database

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func Open(databaseURL string) (*bun.DB, error) {
	sqldb, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	return bun.NewDB(sqldb, pgdialect.New()), nil
}
