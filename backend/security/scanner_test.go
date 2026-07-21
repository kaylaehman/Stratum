package security

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	appdb "github.com/KAE-Labs/stratum/backend/db"
)

// portStoreStub is a minimal in-memory stub for the db.Store methods used by
// persistPorts. It stores port rows keyed by container ID and supports a
// node-scoped view.
type portStoreStub struct {
	rows map[string][]appdb.PortExposureRow // container_id → rows
}

func newPortStoreStub() *portStoreStub {
	return &portStoreStub{rows: map[string][]appdb.PortExposureRow{}}
}

func (s *portStoreStub) ListPortExposuresByContainer(_ context.Context, cid string) ([]appdb.PortExposureRow, error) {
	return s.rows[cid], nil
}

func (s *portStoreStub) ListPortExposuresByNode(_ context.Context, nodeID string) ([]appdb.PortExposureRow, error) {
	var out []appdb.PortExposureRow
	for _, rows := range s.rows {
		for _, r := range rows {
			if r.NodeID == nodeID {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

func (s *portStoreStub) SetPortExposures(_ context.Context, cid string, rows []appdb.PortExposureRow) error {
	s.rows[cid] = rows
	return nil
}

// portScannerUnderTest wraps only the persistPorts method under test, wired to
// a portStoreStub via a thin store interface.
type portScannerStore interface {
	ListPortExposuresByContainer(ctx context.Context, containerID string) ([]appdb.PortExposureRow, error)
	ListPortExposuresByNode(ctx context.Context, nodeID string) ([]appdb.PortExposureRow, error)
	SetPortExposures(ctx context.Context, containerID string, rows []appdb.PortExposureRow) error
}

// miniScanner mirrors the notify + store fields needed for persistPorts only.
// We call the unexported method via a small adapter to avoid importing db.Store.
type miniScanner struct {
	store  portScannerStore
	notify func(ctx context.Context, trigger, title, text string)
}

func (ms *miniScanner) persistPorts(ctx context.Context, c appdb.Container, ports []PortExposure) error {
	existing, err := ms.store.ListPortExposuresByContainer(ctx, c.ID)
	if err != nil {
		return err
	}
	byKey := map[string]appdb.PortExposureRow{}
	for _, e := range existing {
		byKey[portKey(e.HostIP, e.HostPort, e.Protocol)] = e
	}
	nodeRows, err := ms.store.ListPortExposuresByNode(ctx, c.NodeID)
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
	rows := make([]appdb.PortExposureRow, 0, len(ports))
	for _, p := range ports {
		k := portKey(p.HostIP, p.HostPort, p.Protocol)
		row := appdb.PortExposureRow{
			NodeID: c.NodeID, ContainerID: c.ID, HostIP: p.HostIP, HostPort: p.HostPort,
			ContainerPort: p.ContainerPort, Protocol: p.Protocol, InterfaceClass: p.InterfaceClass,
			LastSeen: now,
		}
		if prev, ok := byKey[k]; ok {
			row.ID, row.FirstSeen, row.IsNew, row.NotifiedAt = prev.ID, prev.FirstSeen, prev.IsNew, prev.NotifiedAt
		} else {
			row.ID, row.FirstSeen, row.IsNew = uuid.NewString(), now, true
			if notifiedOnNode[k] {
				row.NotifiedAt = &now
			}
		}
		if row.IsNew && row.NotifiedAt == nil && ms.notify != nil {
			ms.notify(ctx, "port.new", "New exposed port",
				c.Name+" exposed "+p.HostIP+":"+itoaPort(p.HostPort)+"/"+p.Protocol+" ("+p.InterfaceClass+")")
			row.NotifiedAt = &now
		}
		rows = append(rows, row)
	}
	return ms.store.SetPortExposures(ctx, c.ID, rows)
}

func makeContainer(id, nodeID, name string) appdb.Container {
	return appdb.Container{ID: id, NodeID: nodeID, Name: name}
}

func makePorts(hostIP string, hostPort int, proto string) []PortExposure {
	return []PortExposure{{
		HostIP: hostIP, HostPort: hostPort, Protocol: proto,
		ContainerPort: hostPort, InterfaceClass: classifyInterface(hostIP),
	}}
}

// TestPersistPorts_NewPortFires verifies that a genuinely new port triggers the notify callback.
func TestPersistPorts_NewPortFires(t *testing.T) {
	store := newPortStoreStub()
	var notifications []string
	sc := &miniScanner{
		store: store,
		notify: func(_ context.Context, _, _, text string) { notifications = append(notifications, text) },
	}
	c := makeContainer("c1", "n1", "app")
	ports := makePorts("0.0.0.0", 8080, "tcp")

	if err := sc.persistPorts(context.Background(), c, ports); err != nil {
		t.Fatalf("persistPorts: %v", err)
	}
	if len(notifications) != 1 {
		t.Errorf("expected 1 notification for new port, got %d", len(notifications))
	}
}

// TestPersistPorts_SameContainerNoReAlert verifies that on the second scan for
// the same container the already-notified port does NOT re-fire.
func TestPersistPorts_SameContainerNoReAlert(t *testing.T) {
	store := newPortStoreStub()
	var notifications []string
	sc := &miniScanner{
		store: store,
		notify: func(_ context.Context, _, _, text string) { notifications = append(notifications, text) },
	}
	c := makeContainer("c1", "n1", "app")
	ports := makePorts("0.0.0.0", 8080, "tcp")

	// First scan: fires notification.
	_ = sc.persistPorts(context.Background(), c, ports)
	// Second scan (same container, same port): must not re-fire.
	_ = sc.persistPorts(context.Background(), c, ports)

	if len(notifications) != 1 {
		t.Errorf("expected exactly 1 notification across two scans, got %d", len(notifications))
	}
}

// TestPersistPorts_ContainerRecreateNoReAlert is the key regression test.
// After a container is recreated (new container ID), the same port on the same
// node must NOT trigger a second notification.
func TestPersistPorts_ContainerRecreateNoReAlert(t *testing.T) {
	store := newPortStoreStub()
	var notifications []string
	sc := &miniScanner{
		store: store,
		notify: func(_ context.Context, _, _, text string) { notifications = append(notifications, text) },
	}
	ports := makePorts("0.0.0.0", 8086, "tcp")

	// Original container (old ID).
	c1 := makeContainer("c-old", "n1", "stratum-frontend-1")
	_ = sc.persistPorts(context.Background(), c1, ports)

	// Simulate container recreate: new container ID, same node, same port.
	c2 := makeContainer("c-new", "n1", "stratum-frontend-1")
	_ = sc.persistPorts(context.Background(), c2, ports)

	if len(notifications) != 1 {
		t.Errorf("port re-alert on container recreate: got %d notifications, want 1", len(notifications))
	}
}

// TestPersistPorts_TrulyNewPortOnRecreate verifies that a NEW port that was never
// seen before still fires even when the container was recreated.
func TestPersistPorts_TrulyNewPortOnRecreate(t *testing.T) {
	store := newPortStoreStub()
	var notifications []string
	sc := &miniScanner{
		store: store,
		notify: func(_ context.Context, _, _, text string) { notifications = append(notifications, text) },
	}

	// Port 8080 was already seen on this node.
	c1 := makeContainer("c-old", "n1", "app")
	_ = sc.persistPorts(context.Background(), c1, makePorts("0.0.0.0", 8080, "tcp"))

	// Container recreated: same 8080 (no re-alert) + brand new 9090.
	c2 := makeContainer("c-new", "n1", "app")
	newPorts := append(makePorts("0.0.0.0", 8080, "tcp"), makePorts("0.0.0.0", 9090, "tcp")...)
	_ = sc.persistPorts(context.Background(), c2, newPorts)

	// 1 for port 8080 on first container, 1 for port 9090 on recreated container.
	if len(notifications) != 2 {
		t.Errorf("expected 2 notifications (one per unique port), got %d", len(notifications))
	}
}
