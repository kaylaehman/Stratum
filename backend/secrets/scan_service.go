package secrets

import (
	"context"
	"path"
	"strings"
)

// FileReader is a narrow interface for reading a file's text content from a
// node's filesystem. Satisfied by *fs.Service (via Exec) or a stub for tests.
type FileReader interface {
	// ReadFile returns the text content of path on nodeID. Returns an error
	// if the file does not exist or is not readable.
	ReadFile(ctx context.Context, nodeID, filePath string) (string, error)

	// ListDir returns the names of entries in a directory on nodeID.
	// Entries are file/dir base-names only (not full paths). Returns nil on
	// permission/absence errors (best-effort scan).
	ListDir(ctx context.Context, nodeID, dirPath string) ([]string, error)
}

// ScannerService scans a node's compose/env files for plaintext secrets.
// It is read-only: it never writes to any store or returns secret values.
type ScannerService struct {
	fs FileReader
}

// NewScannerService wires the scanner to a filesystem reader.
func NewScannerService(fs FileReader) *ScannerService {
	return &ScannerService{fs: fs}
}

// composeRoots are common directories to search for compose/env files.
var composeRoots = []string{
	"/opt",
	"/home",
	"/srv",
	"/root",
	"/etc",
}

// isTargetFile returns true when the filename looks like a compose or env file
// that may contain plaintext secrets.
func isTargetFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, ".env") {
		return true
	}
	for _, pat := range []string{"docker-compose", "compose"} {
		if strings.HasPrefix(lower, pat) && (strings.HasSuffix(lower, ".yml") || strings.HasSuffix(lower, ".yaml")) {
			return true
		}
	}
	return false
}

// Scan walks well-known directories on nodeID looking for compose/env files and
// runs ScanText against each. Findings never include secret values.
// Errors reading individual files are silently skipped (best-effort).
func (s *ScannerService) Scan(ctx context.Context, nodeID string) ([]ScanFinding, error) {
	var all []ScanFinding
	for _, root := range composeRoots {
		entries, err := s.fs.ListDir(ctx, nodeID, root)
		if err != nil {
			continue // root not accessible — skip
		}
		for _, entry := range entries {
			dir := path.Join(root, entry)
			// Descend one level into each subdirectory.
			subEntries, err := s.fs.ListDir(ctx, nodeID, dir)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if !isTargetFile(sub) {
					continue
				}
				filePath := path.Join(dir, sub)
				content, err := s.fs.ReadFile(ctx, nodeID, filePath)
				if err != nil {
					continue
				}
				findings := ScanText(filePath, content)
				all = append(all, findings...)
			}
			// Also check the root subdirectory itself.
			if isTargetFile(entry) {
				content, err := s.fs.ReadFile(ctx, nodeID, path.Join(root, entry))
				if err == nil {
					findings := ScanText(path.Join(root, entry), content)
					all = append(all, findings...)
				}
			}
		}
	}
	if all == nil {
		all = []ScanFinding{}
	}
	return all, nil
}
