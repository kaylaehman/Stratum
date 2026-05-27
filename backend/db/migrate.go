package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate runs all up migrations against the database at startup. It fails the
// boot if any migration errors. Safe for single-instance SQLite; a Postgres
// advisory lock is noted for the future multi-replica path.
func Migrate(sqldb *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("db: set goose dialect: %w", err)
	}
	if err := goose.Up(sqldb, "migrations"); err != nil {
		return fmt.Errorf("db: migrate up: %w", err)
	}
	return nil
}
