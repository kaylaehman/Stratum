package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

// ListAutomations returns all rows from the automations table.
func (s *Store) ListAutomations(ctx context.Context) ([]appdb.AutomationRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT key, enabled, interval_seconds, config_json,
		        last_run, last_status, last_detail
		 FROM automations ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list automations: %w", err)
	}
	defer rows.Close()
	var out []appdb.AutomationRow
	for rows.Next() {
		r, err := scanAutomation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetAutomation returns a single row. Returns db.ErrNotFound when absent.
func (s *Store) GetAutomation(ctx context.Context, key string) (appdb.AutomationRow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT key, enabled, interval_seconds, config_json,
		        last_run, last_status, last_detail
		 FROM automations WHERE key = ?`, key)
	r, err := scanAutomation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.AutomationRow{}, appdb.ErrNotFound
	}
	return r, err
}

// UpsertAutomation inserts or replaces the user-override row for a key.
func (s *Store) UpsertAutomation(ctx context.Context, key string, enabled bool, intervalSeconds int, configJSON string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO automations (key, enabled, interval_seconds, config_json)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET
		   enabled          = excluded.enabled,
		   interval_seconds = excluded.interval_seconds,
		   config_json      = excluded.config_json`,
		key, boolToInt(enabled), intervalSeconds, configJSON)
	if err != nil {
		return fmt.Errorf("sqlite: upsert automation %q: %w", key, err)
	}
	return nil
}

// SetAutomationRun persists the outcome of one automation run.
func (s *Store) SetAutomationRun(ctx context.Context, key, status, detail string, ranAt time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE automations
		 SET last_status = ?, last_detail = ?, last_run = ?
		 WHERE key = ?`,
		status, detail, tsText(ranAt), key)
	if err != nil {
		return fmt.Errorf("sqlite: set automation run %q: %w", key, err)
	}
	return nil
}

// scanner is the shared row-scan helper; works for both *sql.Row and *sql.Rows.
type automationScanner interface {
	Scan(dest ...any) error
}

func scanAutomation(r automationScanner) (appdb.AutomationRow, error) {
	var (
		row     appdb.AutomationRow
		enabled int
		lastRun sql.NullString
	)
	err := r.Scan(
		&row.Key,
		&enabled,
		&row.IntervalSeconds,
		&row.ConfigJSON,
		&lastRun,
		&row.LastStatus,
		&row.LastDetail,
	)
	if err != nil {
		return appdb.AutomationRow{}, fmt.Errorf("sqlite: scan automation: %w", err)
	}
	row.Enabled = enabled != 0
	if lastRun.Valid && lastRun.String != "" {
		t, err := parseTS(lastRun.String)
		if err == nil {
			row.LastRun = &t
		}
	}
	return row, nil
}

