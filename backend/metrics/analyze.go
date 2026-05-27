package metrics

import (
	"time"

	"github.com/kaylaehman/stratum/backend/db"
)

// Spike thresholds (Feature 9): CPU above 80%, or RAM above 90% of the limit.
const (
	CPUSpikePct      = 80.0
	MemSpikeFraction = 0.90
)

// Spike is a maximal run of consecutive samples whose metric exceeded the
// threshold. Metric is "cpu" or "mem"; Peak is the maximum value in the run
// (percent for cpu, bytes for mem).
type Spike struct {
	Metric string    `json:"metric"`
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Peak   float64   `json:"peak"`
}

// DetectSpikes scans samples (assumed ascending by time) and emits a spike for
// each sustained run over threshold, separately for CPU and memory.
func DetectSpikes(samples []db.ResourceSample) []Spike {
	out := []Spike{}
	out = append(out, runs(samples, "cpu")...)
	out = append(out, runs(samples, "mem")...)
	return out
}

func runs(samples []db.ResourceSample, metric string) []Spike {
	var out []Spike
	var cur *Spike
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, s := range samples {
		over, val := exceeds(s, metric)
		if !over {
			flush()
			continue
		}
		if cur == nil {
			cur = &Spike{Metric: metric, From: s.SampledAt, To: s.SampledAt, Peak: val}
		} else {
			cur.To = s.SampledAt
			if val > cur.Peak {
				cur.Peak = val
			}
		}
	}
	flush()
	return out
}

func exceeds(s db.ResourceSample, metric string) (bool, float64) {
	if metric == "cpu" {
		return s.CPUPct > CPUSpikePct, s.CPUPct
	}
	// memory: only meaningful when a limit is set
	if s.MemLimitBytes <= 0 {
		return false, float64(s.MemBytes)
	}
	frac := float64(s.MemBytes) / float64(s.MemLimitBytes)
	return frac > MemSpikeFraction, float64(s.MemBytes)
}

// Downsample reduces samples to at most maxPoints by averaging within equal-count
// buckets: CPU and memory are averaged, cumulative disk counters take the last
// value, and the bucket timestamp is its last sample's time. Fewer than maxPoints
// samples pass through unchanged.
func Downsample(samples []db.ResourceSample, maxPoints int) []db.ResourceSample {
	n := len(samples)
	if maxPoints <= 0 || n <= maxPoints {
		return samples
	}
	out := make([]db.ResourceSample, 0, maxPoints)
	bucket := float64(n) / float64(maxPoints)
	for i := 0; i < maxPoints; i++ {
		start := int(float64(i) * bucket)
		end := int(float64(i+1) * bucket)
		if i == maxPoints-1 || end > n {
			end = n
		}
		if start >= end {
			continue
		}
		out = append(out, averageBucket(samples[start:end]))
	}
	return out
}

func averageBucket(b []db.ResourceSample) db.ResourceSample {
	var cpuSum, memSum float64
	for _, s := range b {
		cpuSum += s.CPUPct
		memSum += float64(s.MemBytes)
	}
	last := b[len(b)-1]
	count := float64(len(b))
	return db.ResourceSample{
		ContainerID:    last.ContainerID,
		NodeID:         last.NodeID,
		CPUPct:         cpuSum / count,
		MemBytes:       int64(memSum / count),
		MemLimitBytes:  last.MemLimitBytes,
		DiskReadBytes:  last.DiskReadBytes,
		DiskWriteBytes: last.DiskWriteBytes,
		SampledAt:      last.SampledAt,
	}
}
