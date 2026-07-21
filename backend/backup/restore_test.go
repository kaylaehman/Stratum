package backup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

// --- path validation ---

func TestValidArchivePath(t *testing.T) {
	valid := []string{
		"/mnt/backups/vol-123.tar.gz",
		"/var/lib/vz/dump/vzdump-qemu-100-2024_01_01.vma.zst",
		"/backup/data.tgz",
		"/data/archive.tar.zst",
		"/pbs/dump/foo.vma",
		"/pbs/dump/foo.vma.gz",
	}
	for _, p := range valid {
		if !ValidArchivePath(p) {
			t.Errorf("ValidArchivePath(%q) = false, want true", p)
		}
	}

	invalid := []string{
		"",
		"relative/path.tar.gz",
		"/mnt/../etc/passwd",
		"/tmp/file.txt",
		"/mnt/archive.zip",
		"/backup/file",
		"/mnt/../../etc/shadow.tar.gz",
	}
	for _, p := range invalid {
		if ValidArchivePath(p) {
			t.Errorf("ValidArchivePath(%q) = true, want false", p)
		}
	}
}

// --- exec tracking helper ---

type recordingExec struct {
	calls []struct{ cmd string; args []string }
	err   error
	out   string
}

func (r *recordingExec) fn(ctx context.Context, nodeID, cmd string, args ...string) (string, error) {
	r.calls = append(r.calls, struct{ cmd string; args []string }{cmd, args})
	return r.out, r.err
}

// --- RestoreVolume arg construction ---

func TestRestoreVolume_ValidPath(t *testing.T) {
	rec := &recordingExec{out: ""}
	svc := &Service{store: &noopStore{}, exec: rec.fn}

	_, err := svc.RestoreVolume(context.Background(), "node1", "/backup/vol.tar.gz", "/mnt/restore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(rec.calls))
	}
	call := rec.calls[0]
	if call.cmd != "tar" {
		t.Errorf("expected tar command, got %q", call.cmd)
	}
	if len(call.args) < 4 {
		t.Fatalf("expected at least 4 tar args, got %v", call.args)
	}
	if call.args[1] != "/backup/vol.tar.gz" {
		t.Errorf("expected archive path in args, got %v", call.args)
	}
	if call.args[3] != "/mnt/restore" {
		t.Errorf("expected target path in args, got %v", call.args)
	}
}

func TestRestoreVolume_InvalidPaths(t *testing.T) {
	cases := []struct {
		name        string
		archivePath string
		targetPath  string
	}{
		{"traversal in archive", "/mnt/../etc/shadow.tar.gz", "/mnt/restore"},
		{"relative archive", "backup/vol.tar.gz", "/mnt/restore"},
		{"wrong extension", "/backup/vol.tar", "/mnt/restore"},
		{"traversal in target", "/backup/vol.tar.gz", "/mnt/../etc"},
		{"relative target", "/backup/vol.tar.gz", "relative/path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingExec{}
			svc := &Service{store: &noopStore{}, exec: rec.fn}
			_, err := svc.RestoreVolume(context.Background(), "node1", tc.archivePath, tc.targetPath)
			if err == nil {
				t.Error("expected error, got nil")
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Errorf("expected ErrInvalidInput, got %v", err)
			}
			if len(rec.calls) != 0 {
				t.Error("exec must not be called with invalid paths")
			}
		})
	}
}

func TestRestoreVolume_ExecError(t *testing.T) {
	rec := &recordingExec{err: errors.New("ssh: command failed")}
	svc := &Service{store: &noopStore{}, exec: rec.fn}
	_, err := svc.RestoreVolume(context.Background(), "node1", "/backup/vol.tar.gz", "/mnt/restore")
	if err == nil {
		t.Fatal("expected error from exec, got nil")
	}
}

// --- VerifyLatest result logic ---

func TestVerifyLatest_NoBackups(t *testing.T) {
	rec := &recordingExec{}
	svc := &Service{store: &noopStore{}, exec: rec.fn}
	res, err := svc.VerifyLatest(context.Background(), "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Passed {
		t.Error("expected Passed=false when no backups exist")
	}
	if res.Error == "" {
		t.Error("expected Error to be set when no backups exist")
	}
	if len(rec.calls) != 0 {
		t.Errorf("expected 0 exec calls, got %d", len(rec.calls))
	}
}

func TestVerifyLatest_ExtractionFails(t *testing.T) {
	callNum := 0
	svc := &Service{
		store: &noopStore{backups: []db.BackupRow{
			{ID: "b1", NodeID: "node1", Kind: "volume", Status: "ok",
				DestPath: "/backup/vol.tar.gz", StartedAt: time.Now()},
		}},
		exec: func(ctx context.Context, nodeID, cmd string, args ...string) (string, error) {
			callNum++
			if cmd == "mkdir" {
				return "", nil // scratch dir created ok
			}
			if cmd == "tar" {
				return "", errors.New("tar: corrupt archive")
			}
			return "", nil // rm -rf cleanup
		},
	}
	res, err := svc.VerifyLatest(context.Background(), "node1")
	if err != nil {
		t.Fatalf("unexpected infra error: %v", err)
	}
	if res.Passed {
		t.Error("expected Passed=false when extraction fails")
	}
	if res.Error == "" {
		t.Error("expected Error to be set when extraction fails")
	}
}

func TestVerifyLatest_ZeroFiles(t *testing.T) {
	svc := &Service{
		store: &noopStore{backups: []db.BackupRow{
			{ID: "b1", NodeID: "node1", Kind: "volume", Status: "ok",
				DestPath: "/backup/vol.tar.gz", StartedAt: time.Now()},
		}},
		exec: func(ctx context.Context, nodeID, cmd string, args ...string) (string, error) {
			switch cmd {
			case "mkdir":
				return "", nil
			case "tar":
				return "", nil
			case "sh":
				return "0\n", nil // zero file count
			case "rm":
				return "", nil
			}
			return "", nil
		},
	}
	res, err := svc.VerifyLatest(context.Background(), "node1")
	if err != nil {
		t.Fatalf("unexpected infra error: %v", err)
	}
	if res.Passed {
		t.Error("expected Passed=false when zero files extracted")
	}
}

func TestVerifyLatest_Success(t *testing.T) {
	svc := &Service{
		store: &noopStore{backups: []db.BackupRow{
			{ID: "b1", NodeID: "node1", Kind: "volume", Status: "ok",
				DestPath: "/backup/vol.tar.gz", StartedAt: time.Now()},
		}},
		exec: func(ctx context.Context, nodeID, cmd string, args ...string) (string, error) {
			switch cmd {
			case "mkdir":
				return "", nil
			case "tar":
				return "", nil
			case "sh":
				return "42\n", nil // 42 files / 42 bytes
			case "rm":
				return "", nil
			}
			return "", nil
		},
	}
	res, err := svc.VerifyLatest(context.Background(), "node1")
	if err != nil {
		t.Fatalf("unexpected infra error: %v", err)
	}
	if !res.Passed {
		t.Errorf("expected Passed=true, error=%q", res.Error)
	}
	if res.FileCount != 42 {
		t.Errorf("expected FileCount=42, got %d", res.FileCount)
	}
}

// TestVerifyLatest_PicksNewest ensures the most recent ok backup is chosen.
func TestVerifyLatest_PicksNewest(t *testing.T) {
	older := time.Now().Add(-24 * time.Hour)
	newer := time.Now()
	var usedArchive string

	svc := &Service{
		store: &noopStore{backups: []db.BackupRow{
			{ID: "old", NodeID: "node1", Kind: "volume", Status: "ok",
				DestPath: "/backup/old.tar.gz", StartedAt: older},
			{ID: "new", NodeID: "node1", Kind: "volume", Status: "ok",
				DestPath: "/backup/new.tar.gz", StartedAt: newer},
		}},
		exec: func(ctx context.Context, nodeID, cmd string, args ...string) (string, error) {
			if cmd == "tar" {
				// Second arg is the archive path (xzf <path> -C <dir>)
				if len(args) > 1 {
					usedArchive = args[1]
				}
			}
			return "5\n", nil
		},
	}
	_, err := svc.VerifyLatest(context.Background(), "node1")
	if err != nil {
		t.Fatal(err)
	}
	if usedArchive != "/backup/new.tar.gz" {
		t.Errorf("expected newest archive, got %q", usedArchive)
	}
}

// --- parseInt64 ---

func TestParseInt64(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"42", 42},
		{"1000000", 1000000},
		{"", 0},
		{"not_a_number", 0},
		{"-1", -1},
	}
	for _, tc := range cases {
		if got := parseInt64(tc.in); got != tc.want {
			t.Errorf("parseInt64(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// --- removeScratch safety guard ---

func TestRemoveScratch_SafetyGuard(t *testing.T) {
	var rmCalled bool
	svc := &Service{
		exec: func(_ context.Context, _ string, cmd string, _ ...string) (string, error) {
			if cmd == "rm" {
				rmCalled = true
			}
			return "", nil
		},
	}

	for _, p := range []string{"", "/etc", "/tmp/other-tool"} {
		_ = svc.removeScratch(context.Background(), "node1", p)
	}
	if rmCalled {
		t.Error("rm must not be called for paths that don't match /tmp/stratum-verify-")
	}

	_ = svc.removeScratch(context.Background(), "node1", "/tmp/stratum-verify-abc123")
	if !rmCalled {
		t.Error("rm must be called for valid scratch path")
	}
}

// --- noopStore: minimal db.Store stub for backup tests ---

type noopStore struct {
	backups []db.BackupRow
	db.Store // embed to satisfy interface; panics on any unimplemented method
}

func (n *noopStore) CreateBackup(_ context.Context, _ db.BackupRow) error { return nil }
func (n *noopStore) UpdateBackup(_ context.Context, _ db.BackupRow) error { return nil }
func (n *noopStore) ListBackups(_ context.Context) ([]db.BackupRow, error) {
	return n.backups, nil
}
