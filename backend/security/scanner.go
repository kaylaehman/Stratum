package security

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/permissions"
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

	// notify, when set, fires a notification for a newly-detected event (e.g. a
	// new exposed port). Decoupled via a plain callback to avoid importing the
	// webhooks package here.
	notify func(ctx context.Context, trigger, title, text string)
}

// NewScanner builds a Scanner.
func NewScanner(store db.Store, provider ClientProvider, ctrUsers *permissions.ContainerCache, ttl time.Duration) *Scanner {
	return &Scanner{store: store, provider: provider, ctrUsers: ctrUsers, ttl: ttl, seeded: map[string]time.Time{}}
}

// SetNotify wires a notification callback fired on new security events.
func (sc *Scanner) SetNotify(fn func(ctx context.Context, trigger, title, text string)) {
	sc.notify = fn
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
//
// Dedup strategy: a port is "already seen" if ANY row for the same node already
// carries notified_at != nil for the same (host_ip, host_port, protocol) tuple.
// This prevents re-alerting when a container is recreated (new container ID),
// which would otherwise orphan the old rows and make the port appear brand-new.
func (sc *Scanner) persistPorts(ctx context.Context, c db.Container, ports []PortExposure) error {
	// Container-scoped lookup: preserves identity (ID, FirstSeen) across scans.
	existing, err := sc.store.ListPortExposuresByContainer(ctx, c.ID)
	if err != nil {
		return err
	}
	byKey := map[string]db.PortExposureRow{}
	for _, e := range existing {
		byKey[portKey(e.HostIP, e.HostPort, e.Protocol)] = e
	}

	// Node-scoped lookup: detects ports already notified under a previous
	// container ID (e.g. after a container recreate).
	nodeRows, err := sc.store.ListPortExposuresByNode(ctx, c.NodeID)
	if err != nil {
		return err
	}
	notifiedOnNode := map[string]bool{}
	for _, nr := range nodeRows {
		if nr.NotifiedAt != nil {
			notifiedOnNode[portKey(nr.HostIP, nr.HostPort, nr.Protocol)] = true
		}
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
			// Port row already exists under this container: preserve identity.
			row.ID, row.FirstSeen, row.IsNew, row.NotifiedAt = prev.ID, prev.FirstSeen, prev.IsNew, prev.NotifiedAt
		} else {
			row.ID, row.FirstSeen, row.IsNew = uuid.NewString(), now, true
			// If this (host_ip, host_port, protocol) was already notified on this
			// node (under a different container ID), carry the sentinel forward so
			// the notify guard below doesn't re-fire.
			if notifiedOnNode[k] {
				row.NotifiedAt = &now
			}
		}
		// Fire a one-time notification for a genuinely new exposure. The
		// notified_at column persists the sentinel across restarts.
		if row.IsNew && row.NotifiedAt == nil && sc.notify != nil {
			sc.notify(ctx, "port.new", "New exposed port",
				c.Name+" exposed "+p.HostIP+":"+itoaPort(p.HostPort)+"/"+p.Protocol+" ("+p.InterfaceClass+")")
			row.NotifiedAt = &now
		}
		rows = append(rows, row)
	}
	return sc.store.SetPortExposures(ctx, c.ID, rows)
}

func itoaPort(p int) string { return strconv.Itoa(p) }

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
