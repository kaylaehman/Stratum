// Package db opens the application database, runs migrations at startup, and
// defines the Store repository seam. SP0 ships a SQLite implementation
// (modernc, CGO-free); a Postgres implementation is additive behind Store when
// RBAC (feature 30) lands.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Open parses DATABASE_URL, opens the underlying *sql.DB, and applies pragmas.
// Supported schemes: sqlite:// (implemented), postgres:// (deferred).
func Open(databaseURL string) (*sql.DB, error) {
	switch {
	case strings.HasPrefix(databaseURL, "sqlite:"):
		return openSQLite(strings.TrimPrefix(databaseURL, "sqlite://"))
	case strings.HasPrefix(databaseURL, "postgres:"), strings.HasPrefix(databaseURL, "postgresql:"):
		return nil, errors.New("db: postgres is deferred until RBAC (feature 30)")
	default:
		return nil, fmt.Errorf("db: unsupported DATABASE_URL scheme in %q", databaseURL)
	}
}

// openSQLite opens a modernc sqlite database at the given path with WAL,
// foreign keys, and a busy timeout. Pass ":memory:" or a temp path in tests.
func openSQLite(path string) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("db: empty sqlite path")
	}
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(ON)"
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open sqlite: %w", err)
	}
	if err := sqldb.Ping(); err != nil {
		sqldb.Close()
		return nil, fmt.Errorf("db: ping sqlite: %w", err)
	}
	return sqldb, nil
}
