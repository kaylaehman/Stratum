// Package backup orchestrates Docker volume backups (Feature 28). A volume is
// archived with the idiomatic throwaway-container tar (no host root needed):
// `docker run --rm -v <vol>:/data:ro -v <dest>:/backup alpine tar czf ...`.
// Jobs run asynchronously; the DB row tracks running → ok/error. Proxmox vzdump
// + scheduling are a later concern.
package backup

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/db"
)

// ExecFunc runs a command on a node over SSH (matches fs.Service.Exec).
type ExecFunc func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

const backupTimeout = time.Hour

// Service starts and records volume backups.
type Service struct {
	store db.Store
	exec  ExecFunc
}

// New wires the store + SSH exec.
func New(store db.Store, exec ExecFunc) *Service {
	return &Service{store: store, exec: exec}
}

// StartVolumeBackup validates inputs, records a running backup row, and runs the
// archive in the background. Returns the backup id immediately.
func (s *Service) StartVolumeBackup(ctx context.Context, nodeID, volume, destDir string) (string, error) {
	if !ValidVolume(volume) || !ValidDestDir(destDir) {
		return "", ErrInvalidInput
	}
	file := volume + "-" + strconv.FormatInt(time.Now().Unix(), 10) + ".tar.gz"
	destPath := strings.TrimRight(destDir, "/") + "/" + file

	id := uuid.NewString()
	if err := s.store.CreateBackup(ctx, db.BackupRow{
		ID: id, NodeID: nodeID, Kind: "volume", Target: volume, DestPath: destPath, Status: "running",
	}); err != nil {
		return "", err
	}

	go s.run(nodeID, id, volume, destDir, destPath)
	return id, nil
}

// run performs the archive + size capture on a detached context and finalizes
// the row.
func (s *Service) run(nodeID, id, volume, destDir, destPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), backupTimeout)
	defer cancel()

	finish := func(status, errMsg string, size int64) {
		t := time.Now()
		_ = s.store.UpdateBackup(ctx, db.BackupRow{
			ID: id, SizeBytes: size, Status: status, Error: errMsg, FinishedAt: &t,
		})
	}

	// docker run mounts the volume read-only + the dest dir, tars into it. All
	// args are discrete (no shell), and volume/destDir are charset-validated.
	_, err := s.exec(ctx, nodeID, "docker", "run", "--rm",
		"-v", volume+":/data:ro", "-v", destDir+":/backup",
		"alpine", "tar", "czf", "/backup/"+archiveName(destPath), "-C", "/data", ".")
	if err != nil {
		finish("error", "archive failed", 0)
		return
	}
	size := s.statSize(ctx, nodeID, destPath)
	finish("ok", "", size)
}

func (s *Service) statSize(ctx context.Context, nodeID, path string) int64 {
	out, err := s.exec(ctx, nodeID, "stat", "-c", "%s", path)
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	return n
}

// archiveName returns the file component of destPath.
func archiveName(destPath string) string {
	if i := strings.LastIndex(destPath, "/"); i >= 0 {
		return destPath[i+1:]
	}
	return destPath
}

// List returns the backup history.
func (s *Service) List(ctx context.Context) ([]db.BackupRow, error) {
	return s.store.ListBackups(ctx)
}

// ValidVolume allows the Docker volume-name charset.
func ValidVolume(v string) bool {
	if v == "" || len(v) > 128 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == '-') {
			return false
		}
	}
	return true
}

// ValidDestDir requires an absolute, traversal-free path.
func ValidDestDir(d string) bool {
	return strings.HasPrefix(d, "/") && !strings.Contains(d, "..")
}
