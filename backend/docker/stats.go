package docker

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// StatSample is a single raw stats reading for one container. CPU is reported as
// cumulative counters (nanoseconds) so the caller can compute a percentage from
// the delta between two consecutive samples — robust regardless of whether the
// daemon primed PreCPUStats. Disk I/O is cumulative bytes since container start.
type StatSample struct {
	CPUTotalUsage  uint64 // container CPU time consumed (ns), cumulative
	CPUSystemUsage uint64 // host CPU time (ns), cumulative
	OnlineCPUs     int    // number of CPUs (for the percentage scale)
	MemUsageBytes  uint64 // working-set memory (usage minus inactive file cache)
	MemLimitBytes  uint64
	BlkReadBytes   uint64 // cumulative bytes read from block devices
	BlkWriteBytes  uint64 // cumulative bytes written
}

// StatsOneShot reads a single stats frame for a container and extracts the raw
// counters. It uses the non-streaming one-shot endpoint (no priming); CPU
// percentage is derived by the caller from consecutive samples.
func (c *Client) StatsOneShot(ctx context.Context, id string) (StatSample, error) {
	resp, err := c.cli.ContainerStatsOneShot(ctx, id)
	if err != nil {
		return StatSample{}, err
	}
	defer resp.Body.Close()

	var s container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return StatSample{}, err
	}

	onlineCPUs := int(s.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = len(s.CPUStats.CPUUsage.PercpuUsage)
	}

	return StatSample{
		CPUTotalUsage:  s.CPUStats.CPUUsage.TotalUsage,
		CPUSystemUsage: s.CPUStats.SystemUsage,
		OnlineCPUs:     onlineCPUs,
		MemUsageBytes:  workingSetMemory(s.MemoryStats),
		MemLimitBytes:  s.MemoryStats.Limit,
		BlkReadBytes:   blkioBytes(s.BlkioStats, "read"),
		BlkWriteBytes:  blkioBytes(s.BlkioStats, "write"),
	}, nil
}

// workingSetMemory mirrors `docker stats`: subtract the reclaimable file cache
// from total usage. cgroup v1 exposes "total_inactive_file" (hierarchy sum) and
// v2 exposes "inactive_file"; prefer the v1 key when present (it's the value
// Docker itself subtracts on v1), falling back to the v2 key. The clamp guards
// against underflow on pathological readings.
func workingSetMemory(m container.MemoryStats) uint64 {
	cache := m.Stats["inactive_file"]
	if v, ok := m.Stats["total_inactive_file"]; ok {
		cache = v
	}
	if m.Usage > cache {
		return m.Usage - cache
	}
	return m.Usage
}

// blkioBytes sums the recursive io-service-bytes entries for a given op
// ("read"/"write"), case-insensitively (Docker reports "Read"/"read").
func blkioBytes(b container.BlkioStats, op string) uint64 {
	var total uint64
	for _, e := range b.IoServiceBytesRecursive {
		if strings.EqualFold(e.Op, op) {
			total += e.Value
		}
	}
	return total
}

// CPUPercent computes the CPU usage percentage between two consecutive samples
// (prev older, cur newer), scaled by the online CPU count — matching the
// `docker stats` formula. Returns 0 when deltas are not meaningful.
func CPUPercent(prev, cur StatSample) float64 {
	cpuDelta := float64(cur.CPUTotalUsage) - float64(prev.CPUTotalUsage)
	sysDelta := float64(cur.CPUSystemUsage) - float64(prev.CPUSystemUsage)
	cpus := cur.OnlineCPUs
	if cpus == 0 {
		cpus = 1
	}
	if cpuDelta <= 0 || sysDelta <= 0 {
		return 0
	}
	return (cpuDelta / sysDelta) * float64(cpus) * 100.0
}
