package permissions

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Maps holds resolved id->name lookups for one host.
type Maps struct {
	UIDToName map[int]string
	GIDToName map[int]string
}

// ParsePasswd parses /etc/passwd content -> uid->name.
// Lines look like "name:x:1000:1000:...". Malformed lines are skipped.
// Never errors on content; returns a (possibly empty) map.
func ParsePasswd(r io.Reader) map[int]string {
	return parseIDFile(r, 0, 2)
}

// ParseGroup parses /etc/group content -> gid->name.
// Lines look like "name:x:1000:...".
func ParseGroup(r io.Reader) map[int]string {
	return parseIDFile(r, 0, 2)
}

// parseIDFile parses colon-delimited id files where nameField and idField
// identify which colon-separated column holds the name and numeric id.
func parseIDFile(r io.Reader, nameField, idField int) map[int]string {
	result := make(map[int]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ":")
		maxField := nameField
		if idField > maxField {
			maxField = idField
		}
		if len(fields) <= maxField {
			continue
		}
		id, err := strconv.Atoi(fields[idField])
		if err != nil {
			continue
		}
		result[id] = fields[nameField]
	}
	return result
}

// Fetcher reads a file's bytes from a node (e.g. SFTP-backed).
// Injected so the resolver isn't tied to SFTP — SP4 can supply a
// docker-exec fetcher for a container's passwd/group.
type Fetcher func(ctx context.Context, nodeID, path string) ([]byte, error)

type cacheEntry struct {
	maps      *Maps
	expiresAt time.Time
}

// UserDB resolves + caches per-node Maps with a TTL.
type UserDB struct {
	fetch Fetcher
	ttl   time.Duration
	mu    sync.Mutex
	cache map[string]cacheEntry
}

// NewUserDB builds a UserDB with the given fetcher and cache TTL (e.g. 5*time.Minute).
func NewUserDB(fetch Fetcher, ttl time.Duration) *UserDB {
	return &UserDB{
		fetch: fetch,
		ttl:   ttl,
		cache: make(map[string]cacheEntry),
	}
}

// Resolve returns the node's Maps, fetching+parsing /etc/passwd and /etc/group
// on a cache miss or expiry. A fetch error for EITHER file is non-fatal: the
// corresponding map is left empty (minimal/Alpine images may lack the files).
// Resolve only returns an error if BOTH fetches fail (node unreachable).
func (u *UserDB) Resolve(ctx context.Context, nodeID string) (*Maps, error) {
	u.mu.Lock()
	entry, ok := u.cache[nodeID]
	if ok && time.Now().Before(entry.expiresAt) {
		u.mu.Unlock()
		return entry.maps, nil
	}
	u.mu.Unlock()

	passwdBytes, passwdErr := u.fetch(ctx, nodeID, "/etc/passwd")
	groupBytes, groupErr := u.fetch(ctx, nodeID, "/etc/group")

	if passwdErr != nil && groupErr != nil {
		return nil, errors.Join(passwdErr, groupErr)
	}

	m := &Maps{
		UIDToName: make(map[int]string),
		GIDToName: make(map[int]string),
	}
	if passwdErr == nil {
		m.UIDToName = ParsePasswd(bytes.NewReader(passwdBytes))
	}
	if groupErr == nil {
		m.GIDToName = ParseGroup(bytes.NewReader(groupBytes))
	}

	u.mu.Lock()
	u.cache[nodeID] = cacheEntry{maps: m, expiresAt: time.Now().Add(u.ttl)}
	u.mu.Unlock()

	return m, nil
}

// Invalidate drops a node's cached Maps.
func (u *UserDB) Invalidate(nodeID string) {
	u.mu.Lock()
	delete(u.cache, nodeID)
	u.mu.Unlock()
}
