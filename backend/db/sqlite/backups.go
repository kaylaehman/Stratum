package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const backupColumns = `id, node_id, kind, target, dest_path, size_bytes, status, error, started_at, finished_at`

func (s *Store) CreateBackup(ctx context.Context, b appdb.BackupRow) error {
	if b.StartedAt.IsZero() {
		b.StartedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO backups (`+backupColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.NodeID, b.Kind, b.Target, b.DestPath, b.SizeBytes, b.Status, b.Error,
		tsText(b.StartedAt), nullTSText(b.FinishedAt))
	if err != nil {
		return fmt.Errorf("sqlite: create backup: %w", err)
	}
	return nil
}

func (s *Store) UpdateBackup(ctx context.Context, b appdb.BackupRow) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE backups SET size_bytes=?, status=?, error=?, finished_at=? WHERE id=?`,
		b.SizeBytes, b.Status, b.Error, nullTSText(b.FinishedAt), b.ID)
	if err != nil {
		return fmt.Errorf("sqlite: update backup: %w", err)
	}
	return nil
}

func (s *Store) ListBackups(ctx context.Context) ([]appdb.BackupRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+backupColumns+` FROM backups ORDER BY started_at DESC LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list backups: %w", err)
	}
	defer rows.Close()
	var out []appdb.BackupRow
	for rows.Next() {
		var b appdb.BackupRow
		var startedAt string
		var finishedAt sql.NullString
		if err := rows.Scan(&b.ID, &b.NodeID, &b.Kind, &b.Target, &b.DestPath, &b.SizeBytes, &b.Status, &b.Error, &startedAt, &finishedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scan backup: %w", err)
		}
		b.StartedAt, _ = parseTS(startedAt)
		if b.FinishedAt, err = scanNullTS(finishedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
