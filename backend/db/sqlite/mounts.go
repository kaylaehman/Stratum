package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const mountColumns = `id, node_id, container_id, type, source, normalized_source, volume_name, destination, rw`

// ReplaceContainerMounts deletes a container's existing mount rows and inserts
// the given set in one transaction (so a removed destination leaves no orphan).
func (s *Store) ReplaceContainerMounts(ctx context.Context, containerID string, rows []appdb.MountRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin mounts tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM mounts WHERE container_id = ?`, containerID); err != nil {
		return fmt.Errorf("sqlite: clear mounts: %w", err)
	}
	for _, m := range rows {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO mounts (`+mountColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			m.ID, m.NodeID, m.ContainerID, m.Type, m.Source, m.NormalizedSource,
			nullableEmpty(m.VolumeName), m.Destination, boolToInt(m.RW)); err != nil {
			return fmt.Errorf("sqlite: insert mount: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit mounts: %w", err)
	}
	return nil
}

func (s *Store) ListMountsByNode(ctx context.Context, nodeID string) ([]appdb.MountRow, error) {
	return s.queryMounts(ctx, `SELECT `+mountColumns+` FROM mounts WHERE node_id = ?`, nodeID)
}

func (s *Store) ListMountsByContainer(ctx context.Context, containerID string) ([]appdb.MountRow, error) {
	return s.queryMounts(ctx, `SELECT `+mountColumns+` FROM mounts WHERE container_id = ?`, containerID)
}

func (s *Store) queryMounts(ctx context.Context, query string, arg string) ([]appdb.MountRow, error) {
	rows, err := s.db.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, fmt.Errorf("sqlite: query mounts: %w", err)
	}
	defer rows.Close()
	var out []appdb.MountRow
	for rows.Next() {
		var m appdb.MountRow
		var volumeName sql.NullString
		var rw int
		if err := rows.Scan(&m.ID, &m.NodeID, &m.ContainerID, &m.Type, &m.Source, &m.NormalizedSource, &volumeName, &m.Destination, &rw); err != nil {
			return nil, fmt.Errorf("sqlite: scan mount: %w", err)
		}
		m.VolumeName = volumeName.String
		m.RW = rw != 0
		out = append(out, m)
	}
	return out, rows.Err()
}
