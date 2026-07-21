package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

const customSkillColumns = `id, yaml, created_by, created_at, updated_at`

// UpsertCustomSkill inserts a user-authored skill or, if one with the same id
// already exists, replaces its YAML (preserving created_at/created_by).
func (s *Store) UpsertCustomSkill(ctx context.Context, cs appdb.CustomSkill) error {
	now := time.Now()
	if cs.CreatedAt.IsZero() {
		cs.CreatedAt = now
	}
	cs.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO custom_skills (`+customSkillColumns+`) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET yaml = excluded.yaml, updated_at = excluded.updated_at`,
		cs.ID, cs.YAML, cs.CreatedBy, tsText(cs.CreatedAt), tsText(cs.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: upsert custom skill: %w", err)
	}
	return nil
}

func (s *Store) GetCustomSkill(ctx context.Context, id string) (appdb.CustomSkill, error) {
	cs, err := scanCustomSkill(s.db.QueryRowContext(ctx,
		`SELECT `+customSkillColumns+` FROM custom_skills WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.CustomSkill{}, appdb.ErrNotFound
	}
	return cs, err
}

func (s *Store) ListCustomSkills(ctx context.Context) ([]appdb.CustomSkill, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+customSkillColumns+` FROM custom_skills ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list custom skills: %w", err)
	}
	defer rows.Close()
	var out []appdb.CustomSkill
	for rows.Next() {
		cs, err := scanCustomSkillRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, rows.Err()
}

func (s *Store) DeleteCustomSkill(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM custom_skills WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete custom skill: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func scanCustomSkill(row *sql.Row) (appdb.CustomSkill, error) {
	var cs appdb.CustomSkill
	var created, updated string
	if err := row.Scan(&cs.ID, &cs.YAML, &cs.CreatedBy, &created, &updated); err != nil {
		return appdb.CustomSkill{}, err
	}
	cs.CreatedAt, _ = parseTS(created)
	cs.UpdatedAt, _ = parseTS(updated)
	return cs, nil
}

func scanCustomSkillRows(rows *sql.Rows) (appdb.CustomSkill, error) {
	var cs appdb.CustomSkill
	var created, updated string
	if err := rows.Scan(&cs.ID, &cs.YAML, &cs.CreatedBy, &created, &updated); err != nil {
		return appdb.CustomSkill{}, err
	}
	cs.CreatedAt, _ = parseTS(created)
	cs.UpdatedAt, _ = parseTS(updated)
	return cs, nil
}
