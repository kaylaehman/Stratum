package orchestration

import (
	"context"
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/depgraph"
)

// fakeStore implements StoreReader for tests.
type fakeStore struct {
	nodes      []db.Node
	containers map[string][]db.Container // nodeID → containers
	vms        map[string][]db.VM        // nodeID → VMs
}

func (f *fakeStore) GetNode(_ context.Context, id string) (db.Node, error) {
	for _, n := range f.nodes {
		if n.ID == id {
			return n, nil
		}
	}
	return db.Node{}, db.ErrNotFound
}

func (f *fakeStore) ListNodes(_ context.Context) ([]db.Node, error) {
	return f.nodes, nil
}

func (f *fakeStore) ListContainersByNode(_ context.Context, nodeID string) ([]db.Container, error) {
	return f.containers[nodeID], nil
}

func (f *fakeStore) ListVMsByNode(_ context.Context, nodeID string) ([]db.VM, error) {
	return f.vms[nodeID], nil
}

// fakeGraphs returns an empty graph (no cross-container deps from depgraph).
type fakeGraphs struct{}

func (fakeGraphs) ForNode(_ context.Context, _ string) (depgraph.Graph, error) {
	return depgraph.Graph{}, nil
}

var (
	testStore = &fakeStore{
		nodes: []db.Node{
			{ID: "node1", Type: "standalone"},
		},
		containers: map[string][]db.Container{
			"node1": {
				{ID: "c1", NodeID: "node1", Name: "web", ComposeProject: "myapp"},
				{ID: "c2", NodeID: "node1", Name: "db", ComposeProject: "myapp"},
				{ID: "c3", NodeID: "node1", Name: "cache", ComposeProject: "myapp"},
			},
		},
		vms: map[string][]db.VM{},
	}
	testGraphs = fakeGraphs{}
)

func newTestPlanner() *Planner {
	return NewPlanner(testStore, testGraphs)
}

func TestPlanStack_Start_HasSteps(t *testing.T) {
	p := newTestPlanner()
	plan, err := p.PlanStack(context.Background(), "node1", "myapp", "start")
	if err != nil {
		t.Fatalf("PlanStack: %v", err)
	}
	if plan.Target != "stack:myapp" {
		t.Errorf("expected target stack:myapp, got %s", plan.Target)
	}
	if plan.Action != "start" {
		t.Errorf("expected action start, got %s", plan.Action)
	}
	if len(plan.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(plan.Steps))
	}
	for _, s := range plan.Steps {
		if s.Action != "start" {
			t.Errorf("expected step action start, got %s", s.Action)
		}
		if s.Kind != "container" {
			t.Errorf("expected step kind container, got %s", s.Kind)
		}
	}
}

func TestPlanStack_Stop_HasSteps(t *testing.T) {
	p := newTestPlanner()
	plan, err := p.PlanStack(context.Background(), "node1", "myapp", "stop")
	if err != nil {
		t.Fatalf("PlanStack: %v", err)
	}
	if plan.Action != "stop" {
		t.Errorf("expected action stop, got %s", plan.Action)
	}
	if len(plan.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(plan.Steps))
	}
}

func TestPlanStack_EmptyProject(t *testing.T) {
	p := newTestPlanner()
	plan, err := p.PlanStack(context.Background(), "node1", "nonexistent", "start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan.Steps) != 0 {
		t.Errorf("expected 0 steps for empty project, got %d", len(plan.Steps))
	}
}

func TestPlanNode_Standalone(t *testing.T) {
	p := newTestPlanner()
	plan, err := p.PlanNode(context.Background(), "node1", "stop")
	if err != nil {
		t.Fatalf("PlanNode: %v", err)
	}
	if len(plan.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(plan.Steps))
	}
	for _, s := range plan.Steps {
		if s.Action != "stop" {
			t.Errorf("expected step action stop, got %s for %s", s.Action, s.Name)
		}
	}
}

func TestPlanNode_Drain_Action(t *testing.T) {
	p := newTestPlanner()
	plan, err := p.PlanNode(context.Background(), "node1", "drain")
	if err != nil {
		t.Fatalf("PlanNode drain: %v", err)
	}
	// All steps should have action=stop for drain.
	for _, s := range plan.Steps {
		if s.Action != "stop" {
			t.Errorf("drain step should be stop, got %s for %s", s.Action, s.Name)
		}
	}
}

func TestPlanNode_Proxmox(t *testing.T) {
	store := &fakeStore{
		nodes: []db.Node{{ID: "pve1", Type: "proxmox"}},
		containers: map[string][]db.Container{
			"pve1": {{ID: "c1", NodeID: "pve1", Name: "nginx", ComposeProject: "web"}},
		},
		vms: map[string][]db.VM{
			"pve1": {
				{ID: "vm1", NodeID: "pve1", Name: "ubuntu-01", Kind: "qemu", ProxmoxVMID: 100, ProxmoxNode: "pve"},
			},
		},
	}
	p := NewPlanner(store, fakeGraphs{})
	plan, err := p.PlanNode(context.Background(), "pve1", "stop")
	if err != nil {
		t.Fatalf("PlanNode proxmox: %v", err)
	}
	kinds := map[string]int{}
	for _, s := range plan.Steps {
		kinds[s.Kind]++
	}
	if kinds["container"] != 1 {
		t.Errorf("expected 1 container step, got %d", kinds["container"])
	}
	if kinds["vm"] != 1 {
		t.Errorf("expected 1 vm step, got %d", kinds["vm"])
	}
}

func TestBuildContainerPlan_StopReversesOrder(t *testing.T) {
	// With explicit deps: c1→c2 (c1 depends on c2)
	// Start order: c2, c1. Stop order: c1, c2.
	containers := []db.Container{
		{ID: "c1", Name: "web"},
		{ID: "c2", Name: "db"},
	}
	deps := map[string][]string{
		"c1": {"c2"},
		"c2": nil,
	}
	startPlan := buildContainerPlan("stack:test", "start", containers, deps)
	stopPlan := buildContainerPlan("stack:test", "stop", containers, deps)

	startPos := map[string]int{}
	for _, s := range startPlan.Steps {
		startPos[s.ID] = s.Order
	}
	stopPos := map[string]int{}
	for _, s := range stopPlan.Steps {
		stopPos[s.ID] = s.Order
	}

	// Start: db before web
	if startPos["c2"] > startPos["c1"] {
		t.Errorf("start: db (c2) should precede web (c1)")
	}
	// Stop: web before db (reversed)
	if stopPos["c1"] > stopPos["c2"] {
		t.Errorf("stop: web (c1) should precede db (c2)")
	}
}

func TestValidateRequest_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		req  PlanRequest
		ok   bool
	}{
		{"valid stack", PlanRequest{TargetKind: "stack", NodeID: "n1", Project: "p1", Action: "start"}, true},
		{"valid node", PlanRequest{TargetKind: "node", TargetID: "n1", Action: "stop"}, true},
		{"missing node_id for stack", PlanRequest{TargetKind: "stack", Project: "p1", Action: "start"}, false},
		{"missing project for stack", PlanRequest{TargetKind: "stack", NodeID: "n1", Action: "start"}, false},
		{"missing target_id for node", PlanRequest{TargetKind: "node", Action: "start"}, false},
		{"invalid action", PlanRequest{TargetKind: "node", TargetID: "n1", Action: "nuke"}, false},
		{"invalid kind", PlanRequest{TargetKind: "fleet", TargetID: "n1", Action: "start"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequest(tt.req)
			if tt.ok && err != nil {
				t.Errorf("expected ok, got error: %v", err)
			}
			if !tt.ok && err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
