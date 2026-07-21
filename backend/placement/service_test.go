package placement_test

import (
	"context"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/placement"
)

// --- fakes ---

type fakeNodeLister struct {
	nodes []db.Node
}

func (f *fakeNodeLister) ListNodes(_ context.Context) ([]db.Node, error) {
	return f.nodes, nil
}

type fakeSampleReader struct {
	containers map[string][]db.Container    // nodeID -> containers
	samples    map[string][]db.ResourceSample // containerID -> samples
}

func newFakeSampleReader() *fakeSampleReader {
	return &fakeSampleReader{
		containers: make(map[string][]db.Container),
		samples:    make(map[string][]db.ResourceSample),
	}
}

func (f *fakeSampleReader) ListContainersByNode(_ context.Context, nodeID string) ([]db.Container, error) {
	return f.containers[nodeID], nil
}

func (f *fakeSampleReader) ListResourceSamples(_ context.Context, containerID string, _, _ time.Time) ([]db.ResourceSample, error) {
	return f.samples[containerID], nil
}

// dockerNode builds a db.Node with Docker capability set.
func dockerNode(id, name string) db.Node {
	return db.Node{
		ID:               id,
		Name:             name,
		CapabilitiesJSON: `{"docker":true}`,
	}
}

// sshOnlyNode builds a db.Node with no Docker capability.
func sshOnlyNode(id, name string) db.Node {
	return db.Node{
		ID:               id,
		Name:             name,
		CapabilitiesJSON: `{"docker":false}`,
	}
}

// --- tests ---

func TestRecommend_ExcludesNonDockerNodes(t *testing.T) {
	lister := &fakeNodeLister{
		nodes: []db.Node{
			dockerNode("d1", "docker-host"),
			sshOnlyNode("s1", "ssh-only"),
		},
	}
	reader := newFakeSampleReader()
	svc := placement.New(lister, reader, nil)

	recs, err := svc.Recommend(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range recs {
		if r.NodeID == "s1" {
			t.Errorf("ssh-only node should not appear in recommendations")
		}
	}
	if len(recs) != 1 || recs[0].NodeID != "d1" {
		t.Errorf("expected exactly the docker node, got %+v", recs)
	}
}

func TestRecommend_EmptyNodeList(t *testing.T) {
	lister := &fakeNodeLister{}
	svc := placement.New(lister, newFakeSampleReader(), nil)

	recs, err := svc.Recommend(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected empty recommendations, got %v", recs)
	}
}

func TestRecommend_SortsBestFirst(t *testing.T) {
	lister := &fakeNodeLister{
		nodes: []db.Node{
			dockerNode("busy", "busy-node"),
			dockerNode("idle", "idle-node"),
		},
	}
	reader := newFakeSampleReader()

	// "busy" node has a running container consuming 80% CPU and most RAM.
	reader.containers["busy"] = []db.Container{
		{ID: "c-busy", NodeID: "busy", Status: "running"},
	}
	reader.samples["c-busy"] = []db.ResourceSample{
		{ContainerID: "c-busy", CPUPct: 80, MemBytes: 3 << 30, MemLimitBytes: 4 << 30},
	}

	// "idle" node has a container at 10% CPU with plenty of RAM free.
	reader.containers["idle"] = []db.Container{
		{ID: "c-idle", NodeID: "idle", Status: "running"},
	}
	reader.samples["c-idle"] = []db.ResourceSample{
		{ContainerID: "c-idle", CPUPct: 10, MemBytes: 512 << 20, MemLimitBytes: 8 << 30},
	}

	svc := placement.New(lister, reader, nil)
	recs, err := svc.Recommend(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) < 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(recs))
	}
	if recs[0].NodeID != "idle" {
		t.Errorf("expected idle node to rank first; got %q (score=%.3f), idle (score=%.3f)",
			recs[0].NodeID, recs[0].Score, recs[1].Score)
	}
}

func TestRecommend_ResponseShape(t *testing.T) {
	lister := &fakeNodeLister{
		nodes: []db.Node{dockerNode("n1", "mynode")},
	}
	svc := placement.New(lister, newFakeSampleReader(), nil)

	recs, err := svc.Recommend(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(recs))
	}
	r := recs[0]
	if r.NodeID != "n1" {
		t.Errorf("NodeID: got %q, want n1", r.NodeID)
	}
	if r.NodeName != "mynode" {
		t.Errorf("NodeName: got %q, want mynode", r.NodeName)
	}
	if r.Reasons == nil {
		t.Error("Reasons should not be nil")
	}
}

func TestRecommend_ScoreBounded(t *testing.T) {
	lister := &fakeNodeLister{
		nodes: []db.Node{dockerNode("n1", "n1"), dockerNode("n2", "n2")},
	}
	reader := newFakeSampleReader()
	reader.containers["n1"] = []db.Container{{ID: "c1", NodeID: "n1", Status: "running"}}
	reader.samples["c1"] = []db.ResourceSample{
		{CPUPct: 5, MemBytes: 100 << 20, MemLimitBytes: 2 << 30},
	}
	svc := placement.New(lister, reader, nil)

	recs, err := svc.Recommend(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range recs {
		if r.Score < 0 || r.Score > 1 {
			t.Errorf("score %f out of [0,1] for node %q", r.Score, r.NodeID)
		}
	}
}
