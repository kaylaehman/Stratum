package placement

import (
	"context"
	"sort"
	"time"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
)

// Recommendation is the scored result for one docker-capable node.
// This is the JSON shape the frontend receives from GET /api/placement/recommend.
type Recommendation struct {
	NodeID   string       `json:"node_id"`
	NodeName string       `json:"node_name"`
	Score    float64      `json:"score"`
	Reasons  []string     `json:"reasons"`
	Headroom NodeHeadroom `json:"headroom"`
}

// NodeLister returns all registered nodes. Satisfied by *sqlite.Store without
// touching db.Store: the placement package uses a narrow read interface.
type NodeLister interface {
	ListNodes(ctx context.Context) ([]db.Node, error)
}

// SampleReader returns the most-recent metric samples for a container in the
// specified time window. Satisfied by *sqlite.Store.
type SampleReader interface {
	ListResourceSamples(ctx context.Context, containerID string, from, to time.Time) ([]db.ResourceSample, error)
	ListContainersByNode(ctx context.Context, nodeID string) ([]db.Container, error)
}

// DiskProber optionally supplies disk-free bytes for a node. The placement
// service accepts a nil prober and gracefully omits disk scores.
type DiskProber interface {
	DiskFreeBytes(ctx context.Context, nodeID string) (int64, error)
}

// Service collects headroom data and ranks docker-capable nodes.
type Service struct {
	nodes   NodeLister
	samples SampleReader
	disk    DiskProber // may be nil
}

// sampleLookback is how far back we look when averaging recent CPU/RAM samples.
const sampleLookback = 15 * time.Minute

// New builds the placement service. disk may be nil; disk scores will be
// omitted for all nodes when no prober is wired.
func New(nodes NodeLister, samples SampleReader, disk DiskProber) *Service {
	return &Service{nodes: nodes, samples: samples, disk: disk}
}

// Recommend returns nodes ranked by placement suitability, best-first.
// Only docker-capable nodes are included. Errors from individual nodes are
// swallowed — the node is excluded from results, not a fatal failure.
func (s *Service) Recommend(ctx context.Context) ([]Recommendation, error) {
	all, err := s.nodes.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	var recs []Recommendation
	for _, n := range all {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if !caps.Docker {
			continue
		}
		headroom := s.collectHeadroom(ctx, n)
		sc, reasons := Score(headroom)
		recs = append(recs, Recommendation{
			NodeID:   n.ID,
			NodeName: n.Name,
			Score:    sc,
			Reasons:  reasons,
			Headroom: headroom,
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Score > recs[j].Score
	})
	return recs, nil
}

// collectHeadroom assembles NodeHeadroom for one node from recent samples and
// the optional disk prober. Best-effort: missing data leaves fields zero.
func (s *Service) collectHeadroom(ctx context.Context, n db.Node) NodeHeadroom {
	h := NodeHeadroom{NodeID: n.ID, NodeName: n.Name}

	// Aggregate the most-recent samples across all containers on this node.
	ctrs, err := s.samples.ListContainersByNode(ctx, n.ID)
	if err != nil || len(ctrs) == 0 {
		return h
	}

	from := time.Now().Add(-sampleLookback)
	to := time.Now()

	var totalCPUUsed, totalRAMUsed, totalRAMLimit float64
	var sampleCount int

	for _, c := range ctrs {
		if c.Status != "running" {
			continue
		}
		rows, err := s.samples.ListResourceSamples(ctx, c.ID, from, to)
		if err != nil || len(rows) == 0 {
			continue
		}
		// Average the window for this container.
		var cpuSum, memSum float64
		var memLimit float64
		for _, r := range rows {
			cpuSum += r.CPUPct
			memSum += float64(r.MemBytes)
			if r.MemLimitBytes > 0 {
				memLimit = float64(r.MemLimitBytes)
			}
		}
		n := float64(len(rows))
		totalCPUUsed += cpuSum / n
		totalRAMUsed += memSum / n
		totalRAMLimit += memLimit
		sampleCount++
	}

	if sampleCount > 0 {
		// CPUFree: sum of per-container % used relative to 100% per core.
		// A rough heuristic: treat 100% total as the single-core baseline.
		usedFrac := totalCPUUsed / 100.0
		if usedFrac > 1 {
			usedFrac = 1
		}
		h.CPUFree = 1 - usedFrac

		if totalRAMLimit > 0 {
			h.RAMTotalBytes = int64(totalRAMLimit)
			h.RAMFreeBytes = int64(totalRAMLimit - totalRAMUsed)
			if h.RAMFreeBytes < 0 {
				h.RAMFreeBytes = 0
			}
		}
	}

	// Disk free from optional prober.
	if s.disk != nil {
		if free, err := s.disk.DiskFreeBytes(ctx, n.ID); err == nil {
			h.DiskFreeBytes = free
		}
	}

	return h
}
