package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func (s *Store) CreateSecretGroup(ctx context.Context, g appdb.SecretGroup) error {
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO secret_groups (id, name, description, created_at) VALUES (?, ?, ?, ?)`,
		g.ID, g.Name, g.Description, tsText(g.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create secret group: %w", err)
	}
	return nil
}

func (s *Store) ListSecretGroups(ctx context.Context) ([]appdb.SecretGroup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, created_at FROM secret_groups ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list secret groups: %w", err)
	}
	defer rows.Close()
	var out []appdb.SecretGroup
	for rows.Next() {
		var g appdb.SecretGroup
		var createdAt string
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan secret group: %w", err)
		}
		g.CreatedAt, _ = parseTS(createdAt)
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSecretGroup(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM secret_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete secret group: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) UpsertSecret(ctx context.Context, sec appdb.SecretRow) error {
	now := time.Now()
	if sec.CreatedAt.IsZero() {
		sec.CreatedAt = now
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO secrets (id, group_id, key, value_encrypted, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(group_id, key) DO UPDATE SET value_encrypted=excluded.value_encrypted, updated_at=excluded.updated_at`,
		sec.ID, sec.GroupID, sec.Key, sec.ValueEncrypted, tsText(sec.CreatedAt), tsText(now))
	if err != nil {
		return fmt.Errorf("sqlite: upsert secret: %w", err)
	}
	return nil
}

func scanSecret(sc rowScanner) (appdb.SecretRow, error) {
	var s appdb.SecretRow
	var createdAt, updatedAt string
	if err := sc.Scan(&s.ID, &s.GroupID, &s.Key, &s.ValueEncrypted, &createdAt, &updatedAt); err != nil {
		return appdb.SecretRow{}, err
	}
	s.CreatedAt, _ = parseTS(createdAt)
	s.UpdatedAt, _ = parseTS(updatedAt)
	return s, nil
}

func (s *Store) ListSecretsByGroup(ctx context.Context, groupID string) ([]appdb.SecretRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, group_id, key, value_encrypted, created_at, updated_at FROM secrets WHERE group_id = ? ORDER BY key ASC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list secrets: %w", err)
	}
	defer rows.Close()
	var out []appdb.SecretRow
	for rows.Next() {
		sec, err := scanSecret(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sec)
	}
	return out, rows.Err()
}

func (s *Store) GetSecret(ctx context.Context, id string) (appdb.SecretRow, error) {
	sec, err := scanSecret(s.db.QueryRowContext(ctx,
		`SELECT id, group_id, key, value_encrypted, created_at, updated_at FROM secrets WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.SecretRow{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.SecretRow{}, fmt.Errorf("sqlite: get secret: %w", err)
	}
	return sec, nil
}

func (s *Store) DeleteSecret(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete secret: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}
