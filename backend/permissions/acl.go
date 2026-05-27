package permissions

import (
	"context"
	"strings"
)

// ACLEntry is one POSIX ACL entry (pinned shape; consumed by the diagnostic
// reconciler and a future AI prompt).
type ACLEntry struct {
	Tag       string `json:"tag"`       // user | group | mask | other
	Qualifier string `json:"qualifier"` // name or id; "" for owning user/group/mask/other
	Perms     string `json:"perms"`     // e.g. "rw-", "r--", "rwx"
	IsDefault bool   `json:"is_default"`
}

// ACLResult is the outcome of an ACL lookup. Available is false when getfacl is
// missing or errored (e.g. Alpine without the acl package) — the base DAC
// verdict still stands.
type ACLResult struct {
	Available bool       `json:"available"`
	Entries   []ACLEntry `json:"entries,omitempty"`
}

// ACLExec runs a command on a node and returns stdout. Injected so this package
// stays transport-agnostic (the API layer supplies an SSH-backed exec that
// quotes every arg).
type ACLExec func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// GetACL runs `getfacl -c -- <path>` via exec and parses the result. A non-nil
// exec error (missing getfacl, non-zero exit) yields Available:false, not an
// error — ACLs are simply "not evaluated".
func GetACL(ctx context.Context, nodeID, path string, exec ACLExec) ACLResult {
	out, err := exec(ctx, nodeID, "getfacl", "-c", "--", path)
	if err != nil {
		return ACLResult{Available: false}
	}
	return ACLResult{Available: true, Entries: ParseGetfacl(out)}
}

// ParseGetfacl parses `getfacl -c` output (header omitted) into ACL entries.
func ParseGetfacl(out string) []ACLEntry {
	var entries []ACLEntry
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip a trailing "#effective:..." annotation.
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		isDefault := false
		if strings.HasPrefix(line, "default:") {
			isDefault = true
			line = strings.TrimPrefix(line, "default:")
		}
		f := strings.Split(line, ":")
		if len(f) != 3 {
			continue
		}
		entries = append(entries, ACLEntry{
			Tag:       f[0],
			Qualifier: f[1],
			Perms:     f[2],
			IsDefault: isDefault,
		})
	}
	return entries
}
