package sqlite

import (
	"context"
	"fmt"
	"time"
)

// ListFeatureFlags returns the explicitly-set flags as key->enabled. Keys not
// present use the caller's built-in default.
func (s *Store) ListFeatureFlags(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, enabled FROM feature_flags`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list feature_flags: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var key string
		var enabled int
		if err := rows.Scan(&key, &enabled); err != nil {
			return nil, fmt.Errorf("sqlite: scan feature_flag: %w", err)
		}
		out[key] = enabled != 0
	}
	return out, rows.Err()
}

// SetFeatureFlag upserts one flag.
func (s *Store) SetFeatureFlag(ctx context.Context, key string, enabled bool) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feature_flags (key, enabled, configured_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET enabled=excluded.enabled, configured_at=excluded.configured_at`,
		key, boolToInt(enabled), tsText(time.Now()))
	if err != nil {
		return fmt.Errorf("sqlite: set feature_flag: %w", err)
	}
	return nil
}
