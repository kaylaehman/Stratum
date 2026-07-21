package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

const snapshotColumns = `id, container_id, node_id, container_name, reason, image_ref, image_digest, spec_json, created_at`

func (s *Store) CreateSnapshot(ctx context.Context, sn appdb.Snapshot) error {
	if sn.CreatedAt.IsZero() {
		sn.CreatedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO snapshots (`+snapshotColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sn.ID, sn.ContainerID, sn.NodeID, sn.ContainerName, sn.Reason,
		sn.ImageRef, sn.ImageDigest, sn.SpecJSON, tsText(sn.CreatedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create snapshot: %w", err)
	}
	return nil
}

func (s *Store) GetSnapshot(ctx context.Context, id string) (appdb.Snapshot, error) {
	return scanSnapshot(s.db.QueryRowContext(ctx, `SELECT `+snapshotColumns+` FROM snapshots WHERE id = ?`, id))
}

func (s *Store) DeleteSnapshot(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM snapshots WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: delete snapshot: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) ListSnapshotsByContainer(ctx context.Context, nodeID, containerName string) ([]appdb.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+snapshotColumns+` FROM snapshots WHERE node_id = ? AND container_name = ? ORDER BY created_at DESC`,
		nodeID, containerName)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list snapshots: %w", err)
	}
	defer rows.Close()
	var out []appdb.Snapshot
	for rows.Next() {
		sn, err := scanSnapshotRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// PruneSnapshots deletes all but the newest `keep` snapshots for the container.
func (s *Store) PruneSnapshots(ctx context.Context, nodeID, containerName string, keep int) error {
	if keep < 0 {
		keep = 0
	}
	// rowid is the insertion-order tiebreaker so two snapshots written in the
	// same created_at tick (e.g. pre-rollback + the rollback itself) order
	// deterministically and the newest is never pruned.
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM snapshots
		 WHERE node_id = ? AND container_name = ?
		   AND rowid NOT IN (
		       SELECT rowid FROM snapshots
		       WHERE node_id = ? AND container_name = ?
		       ORDER BY created_at DESC, rowid DESC LIMIT ?
		   )`,
		nodeID, containerName, nodeID, containerName, keep)
	if err != nil {
		return fmt.Errorf("sqlite: prune snapshots: %w", err)
	}
	return nil
}

func scanSnapshot(row *sql.Row) (appdb.Snapshot, error) {
	sn, err := scanSnapshotRows(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.Snapshot{}, appdb.ErrNotFound
	}
	return sn, err
}

func scanSnapshotRows(sc rowScanner) (appdb.Snapshot, error) {
	var sn appdb.Snapshot
	var createdAt string
	if err := sc.Scan(&sn.ID, &sn.ContainerID, &sn.NodeID, &sn.ContainerName, &sn.Reason,
		&sn.ImageRef, &sn.ImageDigest, &sn.SpecJSON, &createdAt); err != nil {
		return appdb.Snapshot{}, err
	}
	sn.CreatedAt, _ = parseTS(createdAt)
	return sn, nil
}
