package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

const runbookColumns = `id, name, description, trigger_conditions, steps, requires_approval, created_at, updated_at`

func (s *Store) CreateRunbook(ctx context.Context, rb appdb.Runbook) error {
	now := time.Now()
	if rb.CreatedAt.IsZero() {
		rb.CreatedAt = now
	}
	rb.UpdatedAt = now
	triggers, _ := json.Marshal(rb.TriggerConditions)
	steps, _ := json.Marshal(rb.Steps)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO runbooks (`+runbookColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rb.ID, rb.Name, rb.Description, string(triggers), string(steps),
		boolToInt(rb.RequiresApproval), tsText(rb.CreatedAt), tsText(rb.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create runbook: %w", err)
	}
	return nil
}

func (s *Store) GetRunbook(ctx context.Context, id string) (appdb.Runbook, error) {
	rb, err := scanRunbook(s.db.QueryRowContext(ctx, `SELECT `+runbookColumns+` FROM runbooks WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.Runbook{}, appdb.ErrNotFound
	}
	return rb, err
}

func (s *Store) ListRunbooks(ctx context.Context) ([]appdb.Runbook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+runbookColumns+` FROM runbooks ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list runbooks: %w", err)
	}
	defer rows.Close()
	var out []appdb.Runbook
	for rows.Next() {
		rb, err := scanRunbookRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rb)
	}
	return out, rows.Err()
}

func (s *Store) UpdateRunbook(ctx context.Context, rb appdb.Runbook) error {
	triggers, _ := json.Marshal(rb.TriggerConditions)
	steps, _ := json.Marshal(rb.Steps)
	res, err := s.db.ExecContext(ctx,
		`UPDATE runbooks SET name=?, description=?, trigger_conditions=?, steps=?, requires_approval=?, updated_at=? WHERE id=?`,
		rb.Name, rb.Description, string(triggers), string(steps), boolToInt(rb.RequiresApproval), tsText(time.Now()), rb.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update runbook: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteRunbook(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM runbooks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete runbook: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func scanRunbook(row *sql.Row) (appdb.Runbook, error) { return scanRunbookRows(row) }

func scanRunbookRows(sc rowScanner) (appdb.Runbook, error) {
	var rb appdb.Runbook
	var triggers, steps, createdAt, updatedAt string
	var requiresApproval int
	if err := sc.Scan(&rb.ID, &rb.Name, &rb.Description, &triggers, &steps, &requiresApproval, &createdAt, &updatedAt); err != nil {
		return appdb.Runbook{}, err
	}
	_ = json.Unmarshal([]byte(triggers), &rb.TriggerConditions)
	_ = json.Unmarshal([]byte(steps), &rb.Steps)
	if rb.TriggerConditions == nil {
		rb.TriggerConditions = []string{}
	}
	if rb.Steps == nil {
		rb.Steps = []string{}
	}
	rb.RequiresApproval = requiresApproval != 0
	rb.CreatedAt, _ = parseTS(createdAt)
	rb.UpdatedAt, _ = parseTS(updatedAt)
	return rb, nil
}
