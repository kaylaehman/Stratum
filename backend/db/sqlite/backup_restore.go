package sqlite

import (
	"context"
	"fmt"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

const backupVerifyColumns = `id, node_id, backup_id, archive_path, scratch_dir,
	file_count, total_bytes, passed, error, verified_at`

// CreateBackupVerify inserts a verify-drill result row.
func (s *Store) CreateBackupVerify(ctx context.Context, r appdb.BackupVerifyRow) error {
	if r.VerifiedAt.IsZero() {
		r.VerifiedAt = time.Now()
	}
	passed := 0
	if r.Passed {
		passed = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO backup_verify_results
			(id, node_id, backup_id, archive_path, scratch_dir,
			 file_count, total_bytes, passed, error, verified_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.NodeID, r.BackupID, r.ArchivePath, r.ScratchDir,
		r.FileCount, r.TotalBytes, passed, r.Error, tsText(r.VerifiedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: create backup verify: %w", err)
	}
	return nil
}

// ListBackupVerify returns verify results, newest-first.
// Pass "" for nodeID to return all records (up to 200).
func (s *Store) ListBackupVerify(ctx context.Context, nodeID string) ([]appdb.BackupVerifyRow, error) {
	const q = `SELECT ` + backupVerifyColumns + ` FROM backup_verify_results WHERE (? = '' OR node_id = ?) ORDER BY verified_at DESC LIMIT 200`
	rows, err := s.db.QueryContext(ctx, q, nodeID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list backup verify: %w", err)
	}
	defer rows.Close()

	var out []appdb.BackupVerifyRow
	for rows.Next() {
		var r appdb.BackupVerifyRow
		var verifiedAt string
		var passed int
		if err := rows.Scan(
			&r.ID, &r.NodeID, &r.BackupID, &r.ArchivePath, &r.ScratchDir,
			&r.FileCount, &r.TotalBytes, &passed, &r.Error, &verifiedAt,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scan backup verify: %w", err)
		}
		r.Passed = passed != 0
		r.VerifiedAt, _ = parseTS(verifiedAt)
		out = append(out, r)
	}
	return out, rows.Err()
}
