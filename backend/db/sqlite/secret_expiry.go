package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

// SetSecretExpiry sets or clears expires_at for a secret.
// Passing nil expiresAt stores NULL (no expiry).
func (s *Store) SetSecretExpiry(ctx context.Context, id string, expiresAt *time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE secrets SET expires_at = ? WHERE id = ?`,
		nullTSText(expiresAt), id)
	if err != nil {
		return fmt.Errorf("sqlite: set secret expiry: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// MarkSecretRotated stamps rotated_at = now and optionally updates expires_at.
// When newExpiresAt is nil, the existing expires_at value is preserved.
func (s *Store) MarkSecretRotated(ctx context.Context, id string, newExpiresAt *time.Time) error {
	now := time.Now()
	var res sql.Result
	var err error
	if newExpiresAt != nil {
		res, err = s.db.ExecContext(ctx,
			`UPDATE secrets SET rotated_at = ?, expires_at = ?, updated_at = ? WHERE id = ?`,
			tsText(now), tsText(*newExpiresAt), tsText(now), id)
	} else {
		res, err = s.db.ExecContext(ctx,
			`UPDATE secrets SET rotated_at = ?, updated_at = ? WHERE id = ?`,
			tsText(now), tsText(now), id)
	}
	if err != nil {
		return fmt.Errorf("sqlite: mark secret rotated: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// ListExpiringSecrets returns all secrets with a non-NULL expires_at that is
// either within the next withinDays days or already past. The value_encrypted
// column is never selected.
func (s *Store) ListExpiringSecrets(ctx context.Context, withinDays int) ([]appdb.SecretExpiryRow, error) {
	cutoff := time.Now().AddDate(0, 0, withinDays)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, key, rotated_at, expires_at, created_at
		 FROM secrets
		 WHERE expires_at IS NOT NULL AND expires_at <= ?
		 ORDER BY expires_at ASC`,
		tsText(cutoff))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list expiring secrets: %w", err)
	}
	defer rows.Close()

	var out []appdb.SecretExpiryRow
	for rows.Next() {
		r, err := scanSecretExpiry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanSecretExpiry(sc rowScanner) (appdb.SecretExpiryRow, error) {
	var r appdb.SecretExpiryRow
	var rotatedAt, expiresAt, createdAt sql.NullString
	if err := sc.Scan(&r.SecretID, &r.GroupID, &r.Key, &rotatedAt, &expiresAt, &createdAt); err != nil {
		return appdb.SecretExpiryRow{}, fmt.Errorf("sqlite: scan secret expiry: %w", err)
	}
	var err error
	if r.RotatedAt, err = scanNullTS(rotatedAt); err != nil {
		return appdb.SecretExpiryRow{}, err
	}
	if r.ExpiresAt, err = scanNullTS(expiresAt); err != nil {
		return appdb.SecretExpiryRow{}, err
	}
	r.CreatedAt, _ = scanTS(createdAt)
	return r, nil
}
