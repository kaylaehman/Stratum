package metrics

import (
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
)

func sampleAt(min int, cpu float64, mem, limit int64) db.ResourceSample {
	return db.ResourceSample{
		CPUPct:        cpu,
		MemBytes:      mem,
		MemLimitBytes: limit,
		SampledAt:     time.Date(2026, 5, 1, 0, min, 0, 0, time.UTC),
	}
}

func TestDetectSpikesCPU(t *testing.T) {
	samples := []db.ResourceSample{
		sampleAt(0, 10, 0, 0),
		sampleAt(1, 85, 0, 0), // over 80 -> spike start
		sampleAt(2, 92, 0, 0), // peak
		sampleAt(3, 40, 0, 0), // back under -> spike ends
		sampleAt(4, 95, 0, 0), // second spike (single sample)
	}
	spikes := DetectSpikes(samples)
	var cpu []Spike
	for _, s := range spikes {
		if s.Metric == "cpu" {
			cpu = append(cpu, s)
		}
	}
	if len(cpu) != 2 {
		t.Fatalf("expected 2 cpu spikes, got %d: %+v", len(cpu), cpu)
	}
	if cpu[0].Peak != 92 {
		t.Errorf("first spike peak = %v, want 92", cpu[0].Peak)
	}
}

func TestDetectSpikesMemRequiresLimit(t *testing.T) {
	// 95% of limit -> spike; same usage with no limit -> ignored.
	withLimit := []db.ResourceSample{sampleAt(0, 0, 950, 1000)}
	if s := DetectSpikes(withLimit); len(s) != 1 || s[0].Metric != "mem" {
		t.Errorf("expected 1 mem spike with limit, got %+v", s)
	}
	noLimit := []db.ResourceSample{sampleAt(0, 0, 950, 0)}
	for _, s := range DetectSpikes(noLimit) {
		if s.Metric == "mem" {
			t.Error("mem spike should not fire without a memory limit")
		}
	}
}

func TestDownsample(t *testing.T) {
	// Fewer than max: passthrough.
	small := []db.ResourceSample{sampleAt(0, 1, 0, 0), sampleAt(1, 2, 0, 0)}
	if got := Downsample(small, 10); len(got) != 2 {
		t.Errorf("passthrough len = %d, want 2", len(got))
	}
	// More than max: reduced to max buckets.
	var big []db.ResourceSample
	for i := 0; i < 100; i++ {
		big = append(big, sampleAt(i, float64(i), int64(i), 0))
	}
	got := Downsample(big, 10)
	if len(got) > 10 {
		t.Fatalf("downsampled len = %d, want <= 10", len(got))
	}
	// Last bucket should carry the final sample's cumulative disk + timestamp.
	if got[len(got)-1].SampledAt != big[len(big)-1].SampledAt {
		t.Error("last bucket should keep the final sample's timestamp")
	}
}
