// Package volumes implements Volume Health (Feature 7): a cross-node view of
// Docker volumes with attached/unused status, size, a daily-sampled size trend,
// and safe removal of unused volumes. Volumes are listed live from the daemon
// (seed-on-query); only the size trend is persisted. The volume→container
// mapping reuses the SP7 mount index rather than re-inspecting containers.
package volumes

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/mountindex"
)

// ClientProvider yields a docker client for a node.
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Status classifies a volume by reference count.
const (
	StatusAttached = "attached" // referenced by ≥1 container
	StatusUnused   = "unused"   // referenced by no container (orphaned/dangling)
	StatusUnknown  = "unknown"  // daemon could not report a reference count
)

// SamplePoint is one point on the size trend.
type SamplePoint struct {
	SampledAt time.Time `json:"sampled_at"`
	SizeBytes int64     `json:"size_bytes"`
}

// VolumeView is one volume with derived health fields.
type VolumeView struct {
	NodeID             string        `json:"node_id"`
	Name               string        `json:"name"`
	Driver             string        `json:"driver"`
	Mountpoint         string        `json:"mountpoint"`
	CreatedAt          string        `json:"created_at"`
	SizeBytes          int64         `json:"size_bytes"`
	RefCount           int64         `json:"ref_count"`
	Status             string        `json:"status"`
	AttachedContainers []string      `json:"attached_containers"`
	OverThreshold      bool          `json:"over_threshold"`
	Samples            []SamplePoint `json:"samples"`
}

// Service lists and manages volumes across docker nodes.
type Service struct {
	store     db.Store
	provider  ClientProvider
	mounts    *mountindex.Index
	threshold int64 // alert threshold in bytes; 0 disables the flag
}

// New builds the service. thresholdBytes flags volumes at or above the size; 0
// disables the alert flag.
func New(store db.Store, provider ClientProvider, mounts *mountindex.Index, thresholdBytes int64) *Service {
	return &Service{store: store, provider: provider, mounts: mounts, threshold: thresholdBytes}
}

// ListForNode returns the volumes on one node with derived status, attached
// container names (from the mount index), and the persisted size trend.
func (s *Service) ListForNode(ctx context.Context, nodeID string) ([]VolumeView, error) {
	client, err := s.provider(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	vols, err := client.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	byVolume := s.containersByVolume(ctx, nodeID)
	samples := s.samplesByVolume(ctx, nodeID)

	out := make([]VolumeView, 0, len(vols))
	for _, v := range vols {
		attached := byVolume[v.Name]
		sort.Strings(attached)
		// Never leave these slices nil: a nil slice marshals to JSON `null`, and
		// the frontend does `.attached_containers.length` / `<Sparkline samples=>`
		// which throws on null (white-screens the Volumes page). Empty `[]` is safe.
		if attached == nil {
			attached = []string{}
		}
		pts := samples[v.Name]
		if pts == nil {
			pts = []SamplePoint{}
		}
		out = append(out, VolumeView{
			NodeID:             nodeID,
			Name:               v.Name,
			Driver:             v.Driver,
			Mountpoint:         v.Mountpoint,
			CreatedAt:          v.CreatedAt,
			SizeBytes:          v.SizeBytes,
			RefCount:           v.RefCount,
			Status:             status(v.RefCount, len(attached)),
			AttachedContainers: attached,
			OverThreshold:      s.threshold > 0 && v.SizeBytes >= s.threshold,
			Samples:            pts,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// containersByVolume maps each volume name to the names of containers that mount
// it, using the (freshened) SP7 mount index. Best-effort: a seed failure yields
// an empty map (status falls back to the daemon's RefCount).
func (s *Service) containersByVolume(ctx context.Context, nodeID string) map[string][]string {
	out := map[string][]string{}
	if err := s.mounts.EnsureFresh(ctx, nodeID); err != nil {
		return out
	}
	mountRows, err := s.store.ListMountsByNode(ctx, nodeID)
	if err != nil {
		return out
	}
	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return out
	}
	nameByID := make(map[string]string, len(containers))
	for _, c := range containers {
		nameByID[c.ID] = c.Name
	}
	for _, m := range mountRows {
		if m.VolumeName == "" {
			continue
		}
		name := nameByID[m.ContainerID]
		if name == "" {
			name = m.ContainerID
		}
		out[m.VolumeName] = appendUnique(out[m.VolumeName], name)
	}
	return out
}

func (s *Service) samplesByVolume(ctx context.Context, nodeID string) map[string][]SamplePoint {
	out := map[string][]SamplePoint{}
	rows, err := s.store.ListVolumeSamplesByNode(ctx, nodeID)
	if err != nil {
		return out
	}
	for _, r := range rows {
		out[r.VolumeName] = append(out[r.VolumeName], SamplePoint{SampledAt: r.SampledAt, SizeBytes: r.SizeBytes})
	}
	return out
}

// Remove deletes an unused volume. It refuses to remove a volume the mount index
// shows as still attached (defense against a stale RefCount); the daemon is the
// final arbiter (force is never set).
func (s *Service) Remove(ctx context.Context, nodeID, name string) error {
	if attached := s.containersByVolume(ctx, nodeID)[name]; len(attached) > 0 {
		return ErrVolumeInUse
	}
	client, err := s.provider(ctx, nodeID)
	if err != nil {
		return err
	}
	if err := client.RemoveVolume(ctx, name, false); err != nil {
		return err
	}
	s.mounts.Invalidate(nodeID)
	return nil
}

// Sample records one size/refcount reading per volume on a node (called by the
// daily sampler).
func (s *Service) Sample(ctx context.Context, nodeID string) error {
	client, err := s.provider(ctx, nodeID)
	if err != nil {
		return err
	}
	vols, err := client.ListVolumes(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, v := range vols {
		_ = s.store.InsertVolumeSample(ctx, db.VolumeSample{
			ID:         uuid.NewString(),
			NodeID:     nodeID,
			VolumeName: v.Name,
			SizeBytes:  v.SizeBytes,
			RefCount:   v.RefCount,
			SampledAt:  now,
		})
	}
	return nil
}

// sampleRetention bounds how long size-trend samples are kept. The table is
// regenerable (not an audit trail), so old points are pruned to cap growth.
const sampleRetention = 90 * 24 * time.Hour

// SampleAll records a size sample for every volume on every docker-capable node,
// then prunes samples older than the retention window.
func (s *Service) SampleAll(ctx context.Context) {
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return
	}
	for _, n := range nodes {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if !caps.Docker {
			continue
		}
		_ = s.Sample(ctx, n.ID)
	}
	_, _ = s.store.PruneVolumeSamplesBefore(ctx, time.Now().Add(-sampleRetention))
}

// RunDailySampler samples once immediately, then every interval, until ctx is
// done. Intended to be launched in its own goroutine from main.
func (s *Service) RunDailySampler(ctx context.Context, interval time.Duration) {
	s.SampleAll(ctx)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.SampleAll(ctx)
		}
	}
}

func status(refCount int64, attached int) string {
	if refCount > 0 || attached > 0 {
		return StatusAttached
	}
	if refCount == 0 {
		return StatusUnused
	}
	return StatusUnknown
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
