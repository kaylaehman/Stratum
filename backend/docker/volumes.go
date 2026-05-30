package docker

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/volume"
)

// VolumeInfo is one Docker volume with usage data. SizeBytes and RefCount are
// -1 when the daemon cannot report them (non-local drivers, df unavailable, or
// the df call timed out).
type VolumeInfo struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at"`
	Labels     map[string]string `json:"labels"`
	SizeBytes  int64             `json:"size_bytes"`
	RefCount   int64             `json:"ref_count"`
}

// dfTimeout bounds the /system/df sub-call inside ListVolumes. The df endpoint
// runs `du` on every volume directory which can be slow on NFS or large disks;
// we give it a generous budget but never let it block the volume list entirely.
const dfTimeout = 20 * time.Second

// ListVolumes returns all volumes on the daemon. It uses two Docker API calls:
//
//  1. VolumeList (/volumes) — fast, returns the complete list with no size info.
//  2. DiskUsage (/system/df?type=volume) — slow (runs du), enriches each volume
//     with SizeBytes and RefCount. This call is best-effort: if it fails or
//     times out, volumes are still returned with SizeBytes=-1 / RefCount=-1.
//
// The two-call approach ensures the volume list is always returned even when the
// df computation is slow or the daemon doesn't support the type filter.
func (c *Client) ListVolumes(ctx context.Context) ([]VolumeInfo, error) {
	// Step 1: get the full volume list — always fast.
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{Filters: filters.Args{}})
	if err != nil {
		return nil, err
	}

	// Index volumes by name for O(1) usage-data merge below.
	out := make([]VolumeInfo, 0, len(resp.Volumes))
	idx := make(map[string]int, len(resp.Volumes))
	for _, v := range resp.Volumes {
		if v == nil {
			continue
		}
		idx[v.Name] = len(out)
		out = append(out, VolumeInfo{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			CreatedAt:  v.CreatedAt,
			Labels:     v.Labels,
			SizeBytes:  -1,
			RefCount:   -1,
		})
	}

	// Step 2: enrich with usage data. DiskUsage runs du on the volume directories
	// and can be slow; use a separate sub-deadline so a sluggish daemon doesn't
	// prevent the list from being returned at all.
	dfCtx, cancel := context.WithTimeout(ctx, dfTimeout)
	defer cancel()
	du, err := c.cli.DiskUsage(dfCtx, types.DiskUsageOptions{Types: []types.DiskUsageObject{types.VolumeObject}})
	if err == nil {
		for _, v := range du.Volumes {
			if v == nil || v.UsageData == nil {
				continue
			}
			if i, ok := idx[v.Name]; ok {
				out[i].SizeBytes = v.UsageData.Size
				out[i].RefCount = v.UsageData.RefCount
			}
		}
	}
	// DiskUsage failure is intentionally swallowed: the volume list is still
	// useful even without size/refcount data.

	return out, nil
}

// RemoveVolume removes a volume by name. force removes even if (per the daemon)
// it appears in use; callers should only force after confirming it is unused.
func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	return c.cli.VolumeRemove(ctx, name, force)
}
