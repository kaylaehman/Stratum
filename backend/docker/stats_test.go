package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestCPUPercent(t *testing.T) {
	prev := StatSample{CPUTotalUsage: 100, CPUSystemUsage: 1000, OnlineCPUs: 2}
	cur := StatSample{CPUTotalUsage: 200, CPUSystemUsage: 1500, OnlineCPUs: 2}
	// cpuDelta=100, sysDelta=500 => 0.2 * 2 * 100 = 40%
	if got := CPUPercent(prev, cur); got != 40 {
		t.Errorf("CPUPercent = %v, want 40", got)
	}
	// No system delta => 0 (avoid div-by-zero / garbage).
	if got := CPUPercent(cur, cur); got != 0 {
		t.Errorf("zero-delta CPUPercent = %v, want 0", got)
	}
	// Counter reset (cur < prev) => 0, not negative.
	if got := CPUPercent(cur, prev); got != 0 {
		t.Errorf("reset CPUPercent = %v, want 0", got)
	}
}

func TestBlkioBytesCaseInsensitive(t *testing.T) {
	b := container.BlkioStats{IoServiceBytesRecursive: []container.BlkioStatEntry{
		{Op: "Read", Value: 100},
		{Op: "read", Value: 50},
		{Op: "WRITE", Value: 200},
	}}
	if got := blkioBytes(b, "read"); got != 150 {
		t.Errorf("read bytes = %d, want 150", got)
	}
	if got := blkioBytes(b, "write"); got != 200 {
		t.Errorf("write bytes = %d, want 200", got)
	}
}

func TestWorkingSetMemory(t *testing.T) {
	// cgroup v2 inactive_file subtracted.
	m := container.MemoryStats{Usage: 1000, Stats: map[string]uint64{"inactive_file": 300}}
	if got := workingSetMemory(m); got != 700 {
		t.Errorf("working set = %d, want 700", got)
	}
	// v1 total_inactive_file takes precedence.
	m2 := container.MemoryStats{Usage: 1000, Stats: map[string]uint64{"inactive_file": 300, "total_inactive_file": 400}}
	if got := workingSetMemory(m2); got != 600 {
		t.Errorf("working set v1 = %d, want 600", got)
	}
	// Cache larger than usage => clamp to usage (no underflow).
	m3 := container.MemoryStats{Usage: 100, Stats: map[string]uint64{"inactive_file": 500}}
	if got := workingSetMemory(m3); got != 100 {
		t.Errorf("clamp = %d, want 100", got)
	}
}
