package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

const scriptColumns = `id, name, description, content, created_at, updated_at`

func (s *Store) CreateScript(ctx context.Context, sc appdb.Script) error {
	now := time.Now()
	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = now
	}
	sc.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO scripts (`+scriptColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		sc.ID, sc.Name, sc.Description, sc.Content, tsText(sc.CreatedAt), tsText(sc.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create script: %w", err)
	}
	return nil
}

func scanScript(s rowScanner) (appdb.Script, error) {
	var sc appdb.Script
	var createdAt, updatedAt string
	if err := s.Scan(&sc.ID, &sc.Name, &sc.Description, &sc.Content, &createdAt, &updatedAt); err != nil {
		return appdb.Script{}, err
	}
	sc.CreatedAt, _ = parseTS(createdAt)
	sc.UpdatedAt, _ = parseTS(updatedAt)
	return sc, nil
}

func (s *Store) ListScripts(ctx context.Context) ([]appdb.Script, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+scriptColumns+` FROM scripts ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list scripts: %w", err)
	}
	defer rows.Close()
	var out []appdb.Script
	for rows.Next() {
		sc, err := scanScript(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (s *Store) GetScript(ctx context.Context, id string) (appdb.Script, error) {
	sc, err := scanScript(s.db.QueryRowContext(ctx, `SELECT `+scriptColumns+` FROM scripts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.Script{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.Script{}, fmt.Errorf("sqlite: get script: %w", err)
	}
	return sc, nil
}

func (s *Store) UpdateScript(ctx context.Context, sc appdb.Script) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE scripts SET name=?, description=?, content=?, updated_at=? WHERE id=?`,
		sc.Name, sc.Description, sc.Content, tsText(time.Now()), sc.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update script: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteScript(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM scripts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete script: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}
