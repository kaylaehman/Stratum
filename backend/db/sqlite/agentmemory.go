package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const agentMemoryColumns = `id, scope, scope_id, key, value, source, confirmed, created_at, updated_at`

func (s *Store) CreateAgentMemory(ctx context.Context, m appdb.AgentMemory) error {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_memory (`+agentMemoryColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Scope, m.ScopeID, m.Key, m.Value, m.Source, boolToInt(m.Confirmed),
		tsText(m.CreatedAt), tsText(m.UpdatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create agent_memory: %w", err)
	}
	return nil
}

func (s *Store) GetAgentMemory(ctx context.Context, id string) (appdb.AgentMemory, error) {
	return scanAgentMemory(s.db.QueryRowContext(ctx,
		`SELECT `+agentMemoryColumns+` FROM agent_memory WHERE id = ?`, id))
}

func (s *Store) UpdateAgentMemory(ctx context.Context, m appdb.AgentMemory) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE agent_memory SET value = ?, confirmed = ?, source = ?, updated_at = ? WHERE id = ?`,
		m.Value, boolToInt(m.Confirmed), m.Source, tsText(time.Now()), m.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update agent_memory: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAgentMemory(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM agent_memory WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete agent_memory: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) ListAgentMemory(ctx context.Context, scope, scopeID string, confirmedOnly bool) ([]appdb.AgentMemory, error) {
	q := `SELECT ` + agentMemoryColumns + ` FROM agent_memory WHERE scope = ? AND scope_id = ?`
	if confirmedOnly {
		q += ` AND confirmed = 1`
	}
	q += ` ORDER BY key`
	rows, err := s.db.QueryContext(ctx, q, scope, scopeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list agent_memory: %w", err)
	}
	defer rows.Close()
	var out []appdb.AgentMemory
	for rows.Next() {
		m, err := scanAgentMemoryRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanAgentMemory(row *sql.Row) (appdb.AgentMemory, error) {
	m, err := scanAgentMemoryRows(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.AgentMemory{}, appdb.ErrNotFound
	}
	return m, err
}

func scanAgentMemoryRows(sc rowScanner) (appdb.AgentMemory, error) {
	var m appdb.AgentMemory
	var confirmed int
	var createdAt, updatedAt string
	if err := sc.Scan(&m.ID, &m.Scope, &m.ScopeID, &m.Key, &m.Value, &m.Source, &confirmed, &createdAt, &updatedAt); err != nil {
		return appdb.AgentMemory{}, err
	}
	m.Confirmed = confirmed != 0
	m.CreatedAt, _ = parseTS(createdAt)
	m.UpdatedAt, _ = parseTS(updatedAt)
	return m, nil
}
