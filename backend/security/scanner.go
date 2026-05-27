package security

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/permissions"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Scanner inspects a node's containers, classifies their security posture, and
// persists container_security + port_exposures. Seeded on query with a TTL.
type Scanner struct {
	store    db.Store
	provider ClientProvider
	ctrUsers *permissions.ContainerCache
	ttl      time.Duration

	mu     sync.Mutex
	seeded map[string]time.Time
	sf     singleflight.Group
}

// NewScanner builds a Scanner.
func NewScanner(store db.Store, provider ClientProvider, ctrUsers *permissions.ContainerCache, ttl time.Duration) *Scanner {
	return &Scanner{store: store, provider: provider, ctrUsers: ctrUsers, ttl: ttl, seeded: map[string]time.Time{}}
}

func (sc *Scanner) fresh(nodeID string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	t, ok := sc.seeded[nodeID]
	return ok && time.Since(t) < sc.ttl
}

// Invalidate forces the next EnsureFresh for a node to re-scan.
func (sc *Scanner) Invalidate(nodeID string) {
	sc.mu.Lock()
	delete(sc.seeded, nodeID)
	sc.mu.Unlock()
}

// EnsureFresh re-scans a node's containers if stale (singleflight-deduped).
func (sc *Scanner) EnsureFresh(ctx context.Context, nodeID string) error {
	if sc.fresh(nodeID) {
		return nil
	}
	_, err, _ := sc.sf.Do(nodeID, func() (any, error) {
		if sc.fresh(nodeID) {
			return nil, nil
		}
		containers, err := sc.store.ListContainersByNode(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		client, err := sc.provider(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		var firstErr error
		for _, c := range containers {
			info, err := client.Inspect(ctx, c.DockerID)
			if err != nil {
				continue
			}
			runUID, isRoot := 0, true
			if cu, err := sc.ctrUsers.ResolveContainer(ctx, nodeID, c.DockerID); err == nil {
				id := permissions.EffectiveIdentity(info.ConfigUser, cu.Passwd, cu.Group)
				runUID, isRoot = id.UID, id.IsRoot
			}
			flags, ports := Scan(info, runUID, isRoot)
			if e := sc.store.UpsertContainerSecurity(ctx, csRow(c, flags)); e != nil && firstErr == nil {
				firstErr = e
			}
			if e := sc.persistPorts(ctx, c, ports); e != nil && firstErr == nil {
				firstErr = e
			}
		}
		if firstErr != nil {
			return nil, firstErr
		}
		sc.mu.Lock()
		sc.seeded[nodeID] = time.Now()
		sc.mu.Unlock()
		return nil, nil
	})
	return err
}

// persistPorts merges scanned exposures with stored rows: existing rows keep
// their id/first_seen/is_new (is_new is durable until acknowledged); new
// exposures get is_new=1; gone exposures are dropped.
func (sc *Scanner) persistPorts(ctx context.Context, c db.Container, ports []PortExposure) error {
	existing, err := sc.store.ListPortExposuresByContainer(ctx, c.ID)
	if err != nil {
		return err
	}
	byKey := map[string]db.PortExposureRow{}
	for _, e := range existing {
		byKey[portKey(e.HostIP, e.HostPort, e.Protocol)] = e
	}
	now := time.Now()
	rows := make([]db.PortExposureRow, 0, len(ports))
	for _, p := range ports {
		k := portKey(p.HostIP, p.HostPort, p.Protocol)
		row := db.PortExposureRow{
			NodeID: c.NodeID, ContainerID: c.ID, HostIP: p.HostIP, HostPort: p.HostPort,
			ContainerPort: p.ContainerPort, Protocol: p.Protocol, InterfaceClass: p.InterfaceClass,
			LastSeen: now,
		}
		if prev, ok := byKey[k]; ok {
			row.ID, row.FirstSeen, row.IsNew, row.NotifiedAt = prev.ID, prev.FirstSeen, prev.IsNew, prev.NotifiedAt
		} else {
			row.ID, row.FirstSeen, row.IsNew = uuid.NewString(), now, true
		}
		rows = append(rows, row)
	}
	return sc.store.SetPortExposures(ctx, c.ID, rows)
}

func portKey(ip string, port int, proto string) string {
	return ip + "|" + proto + "|" + strconv.Itoa(port)
}

func csRow(c db.Container, f SecurityFlags) db.ContainerSecurityRow {
	return db.ContainerSecurityRow{
		ContainerID: c.ID, NodeID: c.NodeID,
		Privileged: f.Privileged, CapAddAll: f.CapAddAll, DangerousCaps: f.DangerousCaps,
		SeccompUnconfined: f.SeccompUnconfined, ApparmorUnconfined: f.ApparmorUnconfined,
		Devices: f.Devices, UsernsHost: f.UsernsHost, PidHost: f.PidHost, NetHost: f.NetHost,
		RunsAsRoot: f.RunsAsRoot, RunUID: f.RunUID, ScannedAt: time.Now(),
	}
}
