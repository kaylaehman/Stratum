// Package metrics implements the Resource Timeline (Feature 9): a 15s sampler
// that records per-container CPU/RAM/disk-IO into a time series, plus query-time
// spike detection and downsampling for the charts. Samples are regenerable
// telemetry (not an audit trail) and pruned on a retention window.
package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Sampler polls running containers' stats and persists samples.
type Sampler struct {
	store     db.Store
	provider  ClientProvider
	interval  time.Duration
	retention time.Duration

	mu        sync.Mutex
	prev      map[string]docker.StatSample // keyed by container internal id
	lastPrune time.Time
}

// statCallTimeout bounds a single StatsOneShot so one hung daemon cannot stall
// the whole sample cycle.
const statCallTimeout = 8 * time.Second

// pruneInterval throttles retention pruning (the sampler ticks every 15s, but a
// table-scan delete each tick is wasteful).
const pruneInterval = time.Hour

// NewSampler builds a sampler. interval is the poll period (e.g. 15s); retention
// bounds how long samples are kept (e.g. 7d).
func NewSampler(store db.Store, provider ClientProvider, interval, retention time.Duration) *Sampler {
	return &Sampler{
		store:     store,
		provider:  provider,
		interval:  interval,
		retention: retention,
		prev:      map[string]docker.StatSample{},
	}
}

// Run polls every interval until ctx is done. Intended to run in its own goroutine.
func (s *Sampler) Run(ctx context.Context) {
	t := time.NewTicker(s.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.cycle(ctx)
		}
	}
}

func (s *Sampler) cycle(ctx context.Context) {
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return
	}
	live := map[string]bool{} // container ids seen this cycle (to evict stale prev)
	for _, n := range nodes {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if !caps.Docker {
			continue
		}
		s.sampleNode(ctx, n.ID, live)
	}
	s.evictMissing(live)
	if now := time.Now(); now.Sub(s.lastPrune) >= pruneInterval {
		s.lastPrune = now
		_, _ = s.store.PruneResourceSamplesBefore(ctx, now.Add(-s.retention))
	}
}

func (s *Sampler) sampleNode(ctx context.Context, nodeID string, live map[string]bool) {
	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return
	}
	var client *docker.Client
	for _, c := range containers {
		if c.Status != "running" {
			continue
		}
		if client == nil {
			client, err = s.provider(ctx, nodeID)
			if err != nil {
				return // node unreachable this cycle
			}
		}
		callCtx, cancel := context.WithTimeout(ctx, statCallTimeout)
		cur, err := client.StatsOneShot(callCtx, c.DockerID)
		cancel()
		if err != nil {
			continue // container vanished mid-cycle, or the daemon timed out
		}
		live[c.ID] = true
		cpuPct := s.cpuPercent(c.ID, cur)
		_ = s.store.InsertResourceSample(ctx, db.ResourceSample{
			ID:             uuid.NewString(),
			ContainerID:    c.ID,
			NodeID:         nodeID,
			CPUPct:         cpuPct,
			MemBytes:       int64(cur.MemUsageBytes),
			MemLimitBytes:  int64(cur.MemLimitBytes),
			DiskReadBytes:  int64(cur.BlkReadBytes),
			DiskWriteBytes: int64(cur.BlkWriteBytes),
			SampledAt:      time.Now(),
		})
	}
}

// cpuPercent computes CPU% from the delta against the previous sample for this
// container, then stores the current reading as the new previous. The first
// sample for a container yields 0 (no prior delta).
func (s *Sampler) cpuPercent(containerID string, cur docker.StatSample) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, ok := s.prev[containerID]
	s.prev[containerID] = cur
	if !ok {
		return 0
	}
	return docker.CPUPercent(prev, cur)
}

// evictMissing drops prev readings for containers not seen this cycle so a
// restarted container's counters don't produce a negative/garbage delta.
func (s *Sampler) evictMissing(live map[string]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.prev {
		if !live[id] {
			delete(s.prev, id)
		}
	}
}
