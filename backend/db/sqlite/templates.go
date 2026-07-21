package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

func marshalVars(v []appdb.TemplateVar) string {
	if v == nil {
		v = []appdb.TemplateVar{}
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func unmarshalVars(s string) []appdb.TemplateVar {
	var out []appdb.TemplateVar
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

const templateColumns = `id, name, description, tags, compose_yaml, variables, version, created_at, updated_at`

func (s *Store) CreateTemplate(ctx context.Context, t appdb.Template) error {
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	if t.Version == 0 {
		t.Version = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO templates (`+templateColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Description, marshalStrings(t.Tags), t.ComposeYAML, marshalVars(t.Variables),
		t.Version, tsText(t.CreatedAt), tsText(t.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create template: %w", err)
	}
	return nil
}

func scanTemplate(sc rowScanner) (appdb.Template, error) {
	var t appdb.Template
	var tags, vars, createdAt, updatedAt string
	if err := sc.Scan(&t.ID, &t.Name, &t.Description, &tags, &t.ComposeYAML, &vars, &t.Version, &createdAt, &updatedAt); err != nil {
		return appdb.Template{}, err
	}
	t.Tags = unmarshalStrings(tags)
	t.Variables = unmarshalVars(vars)
	t.CreatedAt, _ = parseTS(createdAt)
	t.UpdatedAt, _ = parseTS(updatedAt)
	return t, nil
}

func (s *Store) ListTemplates(ctx context.Context) ([]appdb.Template, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+templateColumns+` FROM templates ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list templates: %w", err)
	}
	defer rows.Close()
	var out []appdb.Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTemplate(ctx context.Context, id string) (appdb.Template, error) {
	t, err := scanTemplate(s.db.QueryRowContext(ctx, `SELECT `+templateColumns+` FROM templates WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.Template{}, appdb.ErrNotFound
	}
	if err != nil {
		return appdb.Template{}, fmt.Errorf("sqlite: get template: %w", err)
	}
	return t, nil
}

func (s *Store) UpdateTemplate(ctx context.Context, t appdb.Template) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE templates SET name=?, description=?, tags=?, compose_yaml=?, variables=?, version=?, updated_at=? WHERE id=?`,
		t.Name, t.Description, marshalStrings(t.Tags), t.ComposeYAML, marshalVars(t.Variables),
		t.Version, tsText(time.Now()), t.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update template: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTemplate(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete template: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) AddTemplateVersion(ctx context.Context, id string, v appdb.TemplateVersion) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO template_versions (id, template_id, version, compose_yaml, variables, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), id, v.Version, v.ComposeYAML, marshalVars(v.Variables), tsText(v.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: add template version: %w", err)
	}
	return nil
}

func (s *Store) ListTemplateVersions(ctx context.Context, id string) ([]appdb.TemplateVersion, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT version, compose_yaml, variables, created_at FROM template_versions WHERE template_id = ? ORDER BY version DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list template versions: %w", err)
	}
	defer rows.Close()
	var out []appdb.TemplateVersion
	for rows.Next() {
		var v appdb.TemplateVersion
		var vars, createdAt string
		if err := rows.Scan(&v.Version, &v.ComposeYAML, &vars, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan template version: %w", err)
		}
		v.Variables = unmarshalVars(vars)
		v.CreatedAt, _ = parseTS(createdAt)
		out = append(out, v)
	}
	return out, rows.Err()
}
