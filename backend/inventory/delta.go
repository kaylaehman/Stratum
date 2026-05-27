// Package inventory keeps the resource tree fresh: it polls each node's Docker
// and Proxmox APIs, reconciles the result against the DB, and broadcasts
// per-node deltas (+ a per-cycle snapshot) over the WebSocket hub with a
// monotonic seq so clients can detect gaps and resync.
package inventory

import "sync"

// Op is the kind of change in a Delta.
type Op string

const (
	OpAdded   Op = "added"
	OpUpdated Op = "updated"
	OpRemoved Op = "removed"
)

// Kind discriminates the payload of a Delta.
const (
	KindVM        = "vm"
	KindContainer = "container"
)

// Delta is one inventory change carrying the full current row. Exactly one of
// VM / Container is set per Kind.
type Delta struct {
	Op        Op     `json:"op"`
	Kind      string `json:"kind"`
	NodeID    string `json:"node_id"`
	Seq       uint64 `json:"seq"`
	VM        *VMView        `json:"vm,omitempty"`
	Container *ContainerView `json:"container,omitempty"`
}

// seqRegistry hands out per-node monotonic sequence numbers.
type seqRegistry struct {
	mu  sync.Mutex
	seq map[string]uint64
}

func newSeqRegistry() *seqRegistry { return &seqRegistry{seq: map[string]uint64{}} }

// next increments and returns the node's seq.
func (r *seqRegistry) next(nodeID string) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq[nodeID]++
	return r.seq[nodeID]
}

// current returns the node's seq without incrementing (for GET /api/tree).
func (r *seqRegistry) current(nodeID string) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seq[nodeID]
}
