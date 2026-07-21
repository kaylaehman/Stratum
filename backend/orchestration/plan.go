package orchestration

import (
	"context"
	"fmt"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/depgraph"
)

// Step is one unit of work in an execution plan.
type Step struct {
	Kind   string `json:"kind"`   // "container" | "vm"
	ID     string `json:"id"`     // internal DB id
	Name   string `json:"name"`   // display name
	Action string `json:"action"` // "start" | "stop" | "restart"
	Order  int    `json:"order"`  // 0-based execution order
}

// Plan is a dependency-ordered execution plan.
type Plan struct {
	Target string     `json:"target"` // e.g. "stack:myproject" or "node:abc"
	Action string     `json:"action"` // "start" | "stop" | "restart"
	Steps  []Step     `json:"steps"`
	Cycles [][]string `json:"cycles,omitempty"` // IDs in detected SCCs
}

// StepResult is the outcome of executing one step.
type StepResult struct {
	Step       Step   `json:"step"`
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// StoreReader is the narrow slice of db.Store the planner needs.
type StoreReader interface {
	ListContainersByNode(ctx context.Context, nodeID string) ([]db.Container, error)
	ListVMsByNode(ctx context.Context, nodeID string) ([]db.VM, error)
	GetNode(ctx context.Context, id string) (db.Node, error)
	ListNodes(ctx context.Context) ([]db.Node, error)
}

// Planner builds Plans from depgraph data.
type Planner struct {
	store  StoreReader
	graphs GraphProvider
}

// GraphProvider can build a dependency graph for a node.
type GraphProvider interface {
	ForNode(ctx context.Context, nodeID string) (depgraph.Graph, error)
}

// NewPlanner constructs a Planner.
func NewPlanner(store StoreReader, graphs GraphProvider) *Planner {
	return &Planner{store: store, graphs: graphs}
}

// PlanStack builds a plan for all containers in a compose project on a node.
func (p *Planner) PlanStack(ctx context.Context, nodeID, project, action string) (Plan, error) {
	containers, err := p.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return Plan{}, fmt.Errorf("orchestration: list containers: %w", err)
	}
	var members []db.Container
	for _, c := range containers {
		if c.ComposeProject == project {
			members = append(members, c)
		}
	}
	if len(members) == 0 {
		return Plan{Target: "stack:" + project, Action: action, Steps: []Step{}, Cycles: nil}, nil
	}

	graph, _ := p.graphs.ForNode(ctx, nodeID) // best-effort; empty on error
	deps := buildContainerDeps(members, graph)
	return buildContainerPlan("stack:"+project, action, members, deps), nil
}

// PlanNode builds a plan for all guests/containers on a node.
func (p *Planner) PlanNode(ctx context.Context, nodeID, action string) (Plan, error) {
	node, err := p.store.GetNode(ctx, nodeID)
	if err != nil {
		return Plan{}, fmt.Errorf("orchestration: get node: %w", err)
	}

	containers, err := p.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return Plan{}, fmt.Errorf("orchestration: list containers: %w", err)
	}

	var steps []Step
	var allIDs []string
	idToDeps := map[string][]string{}

	// Docker containers.
	graph, _ := p.graphs.ForNode(ctx, nodeID)
	cDeps := buildContainerDeps(containers, graph)
	for id := range cDeps {
		allIDs = append(allIDs, id)
		idToDeps[id] = cDeps[id]
	}

	// Proxmox VMs (if applicable).
	if node.Type == "proxmox" {
		vms, err := p.store.ListVMsByNode(ctx, nodeID)
		if err != nil {
			return Plan{}, fmt.Errorf("orchestration: list vms: %w", err)
		}
		for _, vm := range vms {
			allIDs = append(allIDs, vm.ID)
			idToDeps[vm.ID] = nil
		}
		sorted, cycles := topoSort(allIDs, idToDeps)
		if action == "stop" || action == "drain" {
			reverse(sorted)
		}
		order := 0
		for _, id := range sorted {
			// Check if it's a container.
			for _, c := range containers {
				if c.ID == id {
					a := action
					if a == "drain" {
						a = "stop"
					}
					steps = append(steps, Step{Kind: "container", ID: c.ID, Name: c.Name, Action: a, Order: order})
					order++
					goto next
				}
			}
			// Must be a VM.
			for _, vm := range vms {
				if vm.ID == id {
					a := action
					if a == "drain" {
						a = "stop"
					}
					steps = append(steps, Step{Kind: "vm", ID: vm.ID, Name: vm.Name, Action: a, Order: order})
					order++
					goto next
				}
			}
		next:
		}
		return Plan{Target: "node:" + nodeID, Action: action, Steps: steps, Cycles: cycles}, nil
	}

	// Non-Proxmox: containers only.
	plan := buildContainerPlan("node:"+nodeID, action, containers, cDeps)
	if action == "drain" {
		plan.Action = action
		for i := range plan.Steps {
			plan.Steps[i].Action = "stop"
		}
	}
	return plan, nil
}

// buildContainerDeps builds a dependency map for containers using depgraph edges.
// A container depends on the containers that share the same network/volume if
// they are in the same compose project and appear first.
func buildContainerDeps(containers []db.Container, graph depgraph.Graph) map[string][]string {
	deps := map[string][]string{}
	for _, c := range containers {
		deps[c.ID] = nil // ensure all members are keyed
	}

	// Build index: depgraph node ID -> container DB ID.
	dgToInternal := map[string]string{}
	for _, c := range containers {
		dgToInternal["container:"+c.ID] = c.ID
	}

	// For each shared network/volume, link containers that share it.
	// A container depends on containers in the same compose project that share a
	// resource and have a lower sort-index (stable, avoids cycles in simple cases).
	networkContainers := map[string][]string{} // target -> []containerID
	for _, e := range graph.Edges {
		cID, ok := dgToInternal[e.Source]
		if !ok {
			continue
		}
		networkContainers[e.Target] = append(networkContainers[e.Target], cID)
	}
	// No explicit depends_on data from depgraph; depgraph only has network/volume
	// edges. We do NOT impose arbitrary ordering within a shared network — that
	// would create false dependencies. Only explicit depends_on (future) would add
	// directed edges. So deps remain empty (all independent) unless explicitly set.
	_ = networkContainers

	return deps
}

func buildContainerPlan(target, action string, containers []db.Container, deps map[string][]string) Plan {
	ids := make([]string, 0, len(containers))
	for _, c := range containers {
		ids = append(ids, c.ID)
	}

	sorted, cycles := topoSort(ids, deps)
	if action == "stop" {
		reverse(sorted)
	}
	if action == "restart" {
		// restart = stop order then start order; we flatten into single steps tagged restart
		// actual execute loop will call restart per-step
	}

	byID := map[string]db.Container{}
	for _, c := range containers {
		byID[c.ID] = c
	}

	var steps []Step
	for i, id := range sorted {
		c, ok := byID[id]
		if !ok {
			continue
		}
		steps = append(steps, Step{Kind: "container", ID: c.ID, Name: c.Name, Action: action, Order: i})
	}
	return Plan{Target: target, Action: action, Steps: steps, Cycles: cycles}
}

func reverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
