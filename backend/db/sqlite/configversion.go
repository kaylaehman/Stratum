package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

const cvCols = `id, node_id, path, content, hash, author, created_at`

// InsertConfigVersion appends a new config-version snapshot row.
func (s *Store) InsertConfigVersion(ctx context.Context, v appdb.ConfigVersion) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO config_versions (`+cvCols+`) VALUES (?,?,?,?,?,?,?)`,
		v.ID, v.NodeID, v.Path, v.Content, v.Hash, v.Author, tsText(v.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: insert config_version: %w", err)
	}
	return nil
}

// ListConfigVersions returns history for (nodeID, path), newest first.
// Content is populated so callers can compute drift against the latest.
func (s *Store) ListConfigVersions(ctx context.Context, nodeID, path string) ([]appdb.ConfigVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+cvCols+` FROM config_versions
		  WHERE node_id = ? AND path = ?
		  ORDER BY created_at DESC`,
		nodeID, path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list config_versions: %w", err)
	}
	defer rows.Close()
	return scanCVRows(rows)
}

// GetConfigVersion fetches one snapshot by ID.
func (s *Store) GetConfigVersion(ctx context.Context, id string) (appdb.ConfigVersion, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+cvCols+` FROM config_versions WHERE id = ?`, id)
	v, err := scanCVRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.ConfigVersion{}, appdb.ErrNotFound
	}
	return v, err
}

// LatestConfigVersion returns the most-recently created snapshot for (nodeID, path).
func (s *Store) LatestConfigVersion(ctx context.Context, nodeID, path string) (appdb.ConfigVersion, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+cvCols+` FROM config_versions
		  WHERE node_id = ? AND path = ?
		  ORDER BY created_at DESC LIMIT 1`,
		nodeID, path)
	v, err := scanCVRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.ConfigVersion{}, appdb.ErrNotFound
	}
	return v, err
}

// DeleteConfigVersion removes one snapshot by ID.
func (s *Store) DeleteConfigVersion(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM config_versions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete config_version: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func scanCVRow(row *sql.Row) (appdb.ConfigVersion, error) {
	var v appdb.ConfigVersion
	var createdAt string
	if err := row.Scan(&v.ID, &v.NodeID, &v.Path, &v.Content, &v.Hash, &v.Author, &createdAt); err != nil {
		return appdb.ConfigVersion{}, err
	}
	v.CreatedAt, _ = parseTS(createdAt)
	return v, nil
}

func scanCVRows(rows *sql.Rows) ([]appdb.ConfigVersion, error) {
	var out []appdb.ConfigVersion
	for rows.Next() {
		var v appdb.ConfigVersion
		var createdAt string
		if err := rows.Scan(&v.ID, &v.NodeID, &v.Path, &v.Content, &v.Hash, &v.Author, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan config_version: %w", err)
		}
		v.CreatedAt, _ = parseTS(createdAt)
		out = append(out, v)
	}
	return out, rows.Err()
}
