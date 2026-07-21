package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

// UpsertStackEnvVar inserts or replaces one env-var row for (node, project, key).
func (s *Store) UpsertStackEnvVar(ctx context.Context, r appdb.StackEnvRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO stack_env (node_id, project_name, key, value, secret_id)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(node_id, project_name, key)
		 DO UPDATE SET value=excluded.value, secret_id=excluded.secret_id`,
		r.NodeID, r.ProjectName, r.Key, r.Value, r.SecretID)
	if err != nil {
		return fmt.Errorf("sqlite: upsert stack env: %w", err)
	}
	return nil
}

// ListStackEnvVars returns all env vars for a (node, project) pair, sorted by key.
func (s *Store) ListStackEnvVars(ctx context.Context, nodeID, projectName string) ([]appdb.StackEnvRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT node_id, project_name, key, value, secret_id
		 FROM stack_env
		 WHERE node_id = ? AND project_name = ?
		 ORDER BY key ASC`,
		nodeID, projectName)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list stack env: %w", err)
	}
	defer rows.Close()
	var out []appdb.StackEnvRow
	for rows.Next() {
		var r appdb.StackEnvRow
		if err := rows.Scan(&r.NodeID, &r.ProjectName, &r.Key, &r.Value, &r.SecretID); err != nil {
			return nil, fmt.Errorf("sqlite: scan stack env: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteStackEnvVar removes one env var. Returns ErrNotFound if it didn't exist.
func (s *Store) DeleteStackEnvVar(ctx context.Context, nodeID, projectName, key string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM stack_env WHERE node_id = ? AND project_name = ? AND key = ?`,
		nodeID, projectName, key)
	if err != nil {
		return fmt.Errorf("sqlite: delete stack env: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

// ensure the sql import is referenced even if no ErrNoRows check is needed here.
var _ = sql.ErrNoRows
