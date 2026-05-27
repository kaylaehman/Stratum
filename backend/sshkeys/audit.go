// Package sshkeys audits authorized_keys across a node's users over SSH
// (Feature 21) — no agent required. It enumerates root + /home/* users'
// ~/.ssh/authorized_keys, parses each key (type, comment, SHA256 fingerprint),
// and can delete a key by rewriting the file. Last-used analysis (sshd log
// parsing) and new-key alerts are a later concern.
package sshkeys

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"
)

// ExecFunc runs a command on a node over SSH (matches fs.Service.Exec).
type ExecFunc func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// KeyEntry is one authorized key.
type KeyEntry struct {
	User        string `json:"user"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	Comment     string `json:"comment"`
	Fingerprint string `json:"fingerprint"`
}

// auditScript emits "<user>\t<path>\t<raw key line>" for every non-empty line of
// every root/home user's authorized_keys.
const auditScript = `for d in /root /home/*; do
  f="$d/.ssh/authorized_keys"
  if [ -f "$f" ]; then
    u=$(basename "$d")
    while IFS= read -r line || [ -n "$line" ]; do
      [ -z "$line" ] && continue
      printf '%s\t%s\t%s\n' "$u" "$f" "$line"
    done < "$f"
  fi
done`

// Audit lists all authorized keys on a node.
func Audit(ctx context.Context, nodeID string, exec ExecFunc) ([]KeyEntry, error) {
	out, err := exec(ctx, nodeID, "sh", "-c", auditScript)
	if err != nil {
		return nil, err
	}
	return parseAudit(out), nil
}

// parseAudit parses the tab-separated audit output into key entries, skipping
// comment lines and anything that doesn't parse as an authorized key.
func parseAudit(output string) []KeyEntry {
	entries := []KeyEntry{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		user, path, raw := parts[0], parts[1], strings.TrimSpace(parts[2])
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		pk, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(raw))
		if err != nil {
			continue
		}
		entries = append(entries, KeyEntry{
			User:        user,
			Path:        path,
			Type:        pk.Type(),
			Comment:     comment,
			Fingerprint: ssh.FingerprintSHA256(pk),
		})
	}
	return entries
}

// DeleteKey removes the key with the given fingerprint from path by reading the
// file, dropping the matching line, and writing the result back via writeFile.
// path must be an authorized_keys file (validated by the caller). Returns
// ErrKeyNotFound if no line matched.
func DeleteKey(ctx context.Context, nodeID, path, fingerprint string, exec ExecFunc,
	writeFile func(ctx context.Context, nodeID, path string, content []byte) error) error {

	content, err := exec(ctx, nodeID, "cat", path)
	if err != nil {
		return err
	}
	kept, removed := filterKey(content, fingerprint)
	if !removed {
		return ErrKeyNotFound
	}
	return writeFile(ctx, nodeID, path, []byte(kept))
}

// filterKey returns the file content with the line whose key fingerprint matches
// removed, and whether any line was removed. Non-key lines (comments, blanks)
// are preserved.
func filterKey(content, fingerprint string) (string, bool) {
	var out []string
	removed := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			if pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(trimmed)); err == nil {
				if ssh.FingerprintSHA256(pk) == fingerprint {
					removed = true
					continue // drop this line
				}
			}
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n"), removed
}

// ValidKeyPath guards the delete target to an authorized_keys file.
func ValidKeyPath(p string) bool {
	return strings.HasPrefix(p, "/") && strings.HasSuffix(p, "/.ssh/authorized_keys") && !strings.Contains(p, "..")
}

// ErrKeyNotFound is returned when no key with the requested fingerprint exists.
var ErrKeyNotFound = fmt.Errorf("sshkeys: key not found")
