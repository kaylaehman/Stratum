package docker

import (
	"context"

	"github.com/docker/docker/api/types"
)

// VolumeInfo is one Docker volume with usage data. SizeBytes and RefCount are
// -1 when the daemon cannot report them (non-local drivers, or df unavailable).
type VolumeInfo struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at"`
	Labels     map[string]string `json:"labels"`
	SizeBytes  int64             `json:"size_bytes"`
	RefCount   int64             `json:"ref_count"`
}

// ListVolumes returns all volumes with usage data via the daemon's df endpoint
// (scoped to volumes so it doesn't compute image/container sizes). Size and
// RefCount come from UsageData; when absent they are reported as -1.
func (c *Client) ListVolumes(ctx context.Context) ([]VolumeInfo, error) {
	du, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{Types: []types.DiskUsageObject{types.VolumeObject}})
	if err != nil {
		return nil, err
	}
	out := make([]VolumeInfo, 0, len(du.Volumes))
	for _, v := range du.Volumes {
		if v == nil {
			continue
		}
		vi := VolumeInfo{
			Name:       v.Name,
			Driver:     v.Driver,
			Mountpoint: v.Mountpoint,
			CreatedAt:  v.CreatedAt,
			Labels:     v.Labels,
			SizeBytes:  -1,
			RefCount:   -1,
		}
		if v.UsageData != nil {
			vi.SizeBytes = v.UsageData.Size
			vi.RefCount = v.UsageData.RefCount
		}
		out = append(out, vi)
	}
	return out, nil
}

// RemoveVolume removes a volume by name. force removes even if (per the daemon)
// it appears in use; callers should only force after confirming it is unused.
func (c *Client) RemoveVolume(ctx context.Context, name string, force bool) error {
	return c.cli.VolumeRemove(ctx, name, force)
}
