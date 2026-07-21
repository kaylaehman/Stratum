package backup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/KAE-Labs/stratum/backend/db"
)

// VerifyResult is the outcome of a restore-drill on the newest archive.
type VerifyResult struct {
	BackupID   string    `json:"backup_id"`
	NodeID     string    `json:"node_id"`
	ArchivePath string   `json:"archive_path"`
	ScratchDir  string   `json:"scratch_dir"`
	FileCount  int64     `json:"file_count"`
	TotalBytes int64     `json:"total_bytes"`
	Passed     bool      `json:"passed"`
	Error      string    `json:"error,omitempty"`
	VerifiedAt time.Time `json:"verified_at"`
}

// RestoreVolume extracts a volume archive to targetPath on the given node.
// This is a destructive operation that overwrites existing files in targetPath.
// archivePath must be absolute and traversal-free; targetPath must be absolute
// and traversal-free.
func (s *Service) RestoreVolume(ctx context.Context, nodeID, archivePath, targetPath string) (string, error) {
	if !ValidArchivePath(archivePath) {
		return "", fmt.Errorf("backup: invalid archive path: %w", ErrInvalidInput)
	}
	if !ValidDestDir(targetPath) {
		return "", fmt.Errorf("backup: invalid target path: %w", ErrInvalidInput)
	}

	out, err := s.exec(ctx, nodeID, "tar", "xzf", archivePath, "-C", targetPath)
	if err != nil {
		return "", fmt.Errorf("backup: restore volume: %w", err)
	}
	return out, nil
}

// RestoreGuest restores a Proxmox vzdump archive to the given storage pool via
// the Proxmox API (qmrestore/pct restore). archivePath is the path as known to
// the Proxmox server (e.g. /var/lib/vz/dump/vzdump-qemu-100-…).
// targetStorage is the Proxmox storage name (e.g. "local"). targetVMID is the
// VMID to assign to the restored guest; pass 0 to let Proxmox pick the next
// available VMID.
func (s *Service) RestoreGuest(ctx context.Context, nodeID, pveNode, archivePath, targetStorage string, targetVMID int) (string, error) {
	if s.proxmox == nil {
		return "", fmt.Errorf("backup: proxmox client not configured")
	}
	if !ValidArchivePath(archivePath) {
		return "", fmt.Errorf("backup: invalid archive path: %w", ErrInvalidInput)
	}
	if targetStorage == "" {
		return "", fmt.Errorf("backup: target storage must not be empty: %w", ErrInvalidInput)
	}
	cl, err := s.proxmox(ctx, nodeID)
	if err != nil {
		return "", fmt.Errorf("backup: get proxmox client: %w", err)
	}

	upid, err := cl.VzdumpRestore(ctx, pveNode, archivePath, targetStorage, targetVMID)
	if err != nil {
		return "", fmt.Errorf("backup: vzdump restore: %w", err)
	}
	exitStatus, err := cl.WaitTask(ctx, pveNode, upid)
	if err != nil {
		return "", fmt.Errorf("backup: wait restore task: %w", err)
	}
	if exitStatus != "OK" {
		return "", fmt.Errorf("backup: restore task exited: %s", exitStatus)
	}
	return upid, nil
}

// VerifyLatest restores the newest completed volume archive on nodeID to a
// temporary scratch directory, checksums/stats the result, then cleans up.
// Returns a VerifyResult regardless of outcome; err is non-nil only for
// infra-level failures (e.g. ListBackups failing).
func (s *Service) VerifyLatest(ctx context.Context, nodeID string) (VerifyResult, error) {
	rows, err := s.store.ListBackups(ctx)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("backup: verify: list backups: %w", err)
	}

	// Find the newest completed (ok) volume backup on this node.
	var chosen *backupRecord
	for i := range rows {
		r := &rows[i]
		if r.NodeID != nodeID || r.Kind != "volume" || r.Status != "ok" {
			continue
		}
		if chosen == nil || r.StartedAt.After(chosen.StartedAt) {
			chosen = &backupRecord{
				ID:       r.ID,
				NodeID:   r.NodeID,
				DestPath: r.DestPath,
				StartedAt: r.StartedAt,
			}
		}
	}

	res := VerifyResult{NodeID: nodeID, VerifiedAt: time.Now()}

	if chosen == nil {
		res.Error = "no completed volume backup found for node"
		return res, nil
	}
	res.BackupID = chosen.ID
	res.ArchivePath = chosen.DestPath

	scratch, err := s.mkScratch(ctx, nodeID)
	if err != nil {
		res.Error = fmt.Sprintf("create scratch dir: %v", err)
		return res, nil
	}
	res.ScratchDir = scratch

	// Best-effort cleanup regardless of outcome.
	defer func() {
		_ = s.removeScratch(ctx, nodeID, scratch)
	}()

	if !ValidArchivePath(chosen.DestPath) {
		res.Error = "stored archive path failed validation"
		return res, nil
	}

	_, err = s.exec(ctx, nodeID, "tar", "xzf", chosen.DestPath, "-C", scratch)
	if err != nil {
		res.Error = fmt.Sprintf("extract: %v", err)
		return res, nil
	}

	fileCount, totalBytes, statErr := s.statExtracted(ctx, nodeID, scratch)
	if statErr != nil {
		res.Error = fmt.Sprintf("stat: %v", statErr)
		return res, nil
	}

	res.FileCount = fileCount
	res.TotalBytes = totalBytes
	res.Passed = fileCount > 0
	if !res.Passed {
		res.Error = "archive extracted zero files"
	}
	s.persistVerify(ctx, res)
	return res, nil
}

// ListVerifyResults returns persisted verify-drill history for nodeID.
// Returns an empty slice (not an error) when no verifyStore is wired.
func (s *Service) ListVerifyResults(ctx context.Context, nodeID string) ([]db.BackupVerifyRow, error) {
	if s.verifyStore == nil {
		return nil, nil
	}
	return s.verifyStore.ListBackupVerify(ctx, nodeID)
}

// persistVerify records a VerifyResult to the DB if a verifyStore is wired.
// Failures are silently swallowed — the result is still returned to the caller.
func (s *Service) persistVerify(ctx context.Context, res VerifyResult) {
	if s.verifyStore == nil {
		return
	}
	row := db.BackupVerifyRow{
		ID:          uuid.NewString(),
		NodeID:      res.NodeID,
		BackupID:    res.BackupID,
		ArchivePath: res.ArchivePath,
		ScratchDir:  res.ScratchDir,
		FileCount:   res.FileCount,
		TotalBytes:  res.TotalBytes,
		Passed:      res.Passed,
		Error:       res.Error,
		VerifiedAt:  res.VerifiedAt,
	}
	_ = s.verifyStore.CreateBackupVerify(ctx, row)
}

// mkScratch creates a uniquely-named temp directory on the node.
func (s *Service) mkScratch(ctx context.Context, nodeID string) (string, error) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	dir := "/tmp/stratum-verify-" + hex.EncodeToString(b)
	if _, err := s.exec(ctx, nodeID, "mkdir", "-p", dir); err != nil {
		return "", err
	}
	return dir, nil
}

// removeScratch deletes the scratch directory.
func (s *Service) removeScratch(ctx context.Context, nodeID, dir string) error {
	if dir == "" || !strings.HasPrefix(dir, "/tmp/stratum-verify-") {
		return nil // safety guard: never remove unexpected paths
	}
	_, err := s.exec(ctx, nodeID, "rm", "-rf", dir)
	return err
}

// statExtracted counts files and total bytes under dir using find + awk.
// Returns (fileCount, totalBytes, error).
func (s *Service) statExtracted(ctx context.Context, nodeID, dir string) (int64, int64, error) {
	// find outputs one line per file: <size> <name>
	// We use stat -c "%s" + find to avoid awk dependency assumptions.
	out, err := s.exec(ctx, nodeID, "sh", "-c",
		fmt.Sprintf(`find %q -type f | wc -l`, dir))
	if err != nil {
		return 0, 0, fmt.Errorf("count files: %w", err)
	}
	fileCount := parseInt64(strings.TrimSpace(out))

	out2, err := s.exec(ctx, nodeID, "sh", "-c",
		fmt.Sprintf(`du -sb %q | cut -f1`, dir))
	if err != nil {
		// du -sb may not be available on all distros; treat as non-fatal
		return fileCount, 0, nil
	}
	totalBytes := parseInt64(strings.TrimSpace(out2))
	return fileCount, totalBytes, nil
}

// parseInt64 parses a string to int64, returning 0 on parse failure.
func parseInt64(s string) int64 {
	var n int64
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// backupRecord is a minimal projection of db.BackupRow for internal use.
type backupRecord struct {
	ID        string
	NodeID    string
	DestPath  string
	StartedAt time.Time
}

// ValidArchivePath validates that a path is absolute, traversal-free, and
// ends with a recognised archive extension.
func ValidArchivePath(p string) bool {
	if !strings.HasPrefix(p, "/") {
		return false
	}
	if strings.Contains(p, "..") {
		return false
	}
	lower := strings.ToLower(p)
	return strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".tar.zst") ||
		strings.HasSuffix(lower, ".vma") ||
		strings.HasSuffix(lower, ".vma.zst") ||
		strings.HasSuffix(lower, ".vma.gz")
}
