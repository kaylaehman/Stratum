package db

import (
	"context"
	"time"
)

// BackupVerifyRow is a recorded result of a restore-drill verification
// (backup.verify automation / on-demand POST /api/backups/verify).
type BackupVerifyRow struct {
	ID          string    `json:"id"`
	NodeID      string    `json:"node_id"`
	BackupID    string    `json:"backup_id"`
	ArchivePath string    `json:"archive_path"`
	ScratchDir  string    `json:"scratch_dir"`
	FileCount   int64     `json:"file_count"`
	TotalBytes  int64     `json:"total_bytes"`
	Passed      bool      `json:"passed"`
	Error       string    `json:"error,omitempty"`
	VerifiedAt  time.Time `json:"verified_at"`
}

// BackupRestoreStore is the narrow persistence interface owned by the backup
// package. The backup.Service accepts this interface; the full db.Store embeds
// it via the sqlite.Store implementation.
//
// Do NOT add these methods to db.Store — that would break every existing fake.
// Instead, pass a BackupRestoreStore alongside the regular db.Store.
type BackupRestoreStore interface {
	// CreateBackupVerify records a verify-drill result.
	CreateBackupVerify(ctx context.Context, r BackupVerifyRow) error
	// ListBackupVerify returns verify records for a node, newest-first.
	// Pass "" for nodeID to return all records.
	ListBackupVerify(ctx context.Context, nodeID string) ([]BackupVerifyRow, error)
}
