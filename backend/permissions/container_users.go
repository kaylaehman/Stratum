package permissions

import (
	"bytes"
	"context"
	"sync"
	"time"
)

// ContainerUsers is a container's parsed passwd/group plus derived id->name maps.
type ContainerUsers struct {
	Passwd    []PasswdEntry
	Group     []GroupEntry
	UIDToName map[int]string
	GIDToName map[int]string
}

// ContainerFetcher reads a file's bytes from a container (e.g. via the docker
// client's CopyFromContainer). Injected so this package stays transport-agnostic
// and testable. A missing file (distroless) should be reported by returning an
// error OR empty bytes — either way the corresponding table is left empty.
type ContainerFetcher func(ctx context.Context, nodeID, containerID, path string) ([]byte, error)

// ContainerCache resolves + caches container user tables per (node, container).
type ContainerCache struct {
	fetch ContainerFetcher
	ttl   time.Duration

	mu    sync.Mutex
	cache map[string]containerEntry
}

type containerEntry struct {
	users   *ContainerUsers
	expires time.Time
}

// NewContainerCache builds a cache with the given fetcher and TTL.
func NewContainerCache(fetch ContainerFetcher, ttl time.Duration) *ContainerCache {
	return &ContainerCache{fetch: fetch, ttl: ttl, cache: map[string]containerEntry{}}
}

// ResolveContainer reads and parses the container's /etc/passwd and /etc/group.
// Missing files (distroless) yield empty tables, not an error — the numeric
// comparison still works. Results are cached for the TTL.
func (c *ContainerCache) ResolveContainer(ctx context.Context, nodeID, containerID string) (*ContainerUsers, error) {
	c.mu.Lock()
	if e, ok := c.cache[containerID]; ok && time.Now().Before(e.expires) {
		c.mu.Unlock()
		return e.users, nil
	}
	c.mu.Unlock()

	cu := &ContainerUsers{UIDToName: map[int]string{}, GIDToName: map[int]string{}}
	if data, err := c.fetch(ctx, nodeID, containerID, "/etc/passwd"); err == nil && len(data) > 0 {
		cu.Passwd = ParsePasswdFull(bytes.NewReader(data))
		for _, e := range cu.Passwd {
			cu.UIDToName[e.UID] = e.Name
		}
	}
	if data, err := c.fetch(ctx, nodeID, containerID, "/etc/group"); err == nil && len(data) > 0 {
		cu.Group = ParseGroupFull(bytes.NewReader(data))
		for _, g := range cu.Group {
			cu.GIDToName[g.GID] = g.Name
		}
	}

	c.mu.Lock()
	c.cache[containerID] = containerEntry{users: cu, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return cu, nil
}

// Invalidate drops a container's cached user table (call on a SP2 'removed'
// delta or container recreate).
func (c *ContainerCache) Invalidate(containerID string) {
	c.mu.Lock()
	delete(c.cache, containerID)
	c.mu.Unlock()
}
