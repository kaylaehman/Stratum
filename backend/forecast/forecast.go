// Package forecast implements capacity-forecasting (C4): it reads recent
// ResourceSample rows from the DB, fits a linear trend per metric, and returns
// time-to-threshold projections for disk free, RAM, and CPU.
//
// All projection math is pure (no side effects) so it is easy to unit-test.
// The Service layer reads from the DB via a narrow SampleReader interface so
// that the package has no compile-time dependency on *sqlite.Store.
package forecast

import (
	"context"
	"math"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

// MetricName identifies which resource is being projected.
type MetricName string

const (
	MetricCPU    MetricName = "cpu"
	MetricMemory MetricName = "memory"
	MetricDisk   MetricName = "disk_write" // cumulative disk-write bytes as a proxy for disk consumption
)

// Point is a (time, value) observation. T is seconds since an arbitrary epoch
// (caller-chosen); Value is the metric reading.
type Point struct {
	T     float64
	Value float64
}

// Projection is the forecast result for one metric on one container/node.
type Projection struct {
	// Metric identifies the projected resource.
	Metric MetricName `json:"metric"`
	// Current is the most-recent observed value.
	Current float64 `json:"current"`
	// Threshold is the value used as the "full / critical" boundary.
	Threshold float64 `json:"threshold"`
	// EtaSeconds is the projected seconds until Current reaches Threshold at
	// the observed trend slope.  -1 means "no ETA" (flat/negative slope, or
	// threshold already satisfied in the wrong direction).
	EtaSeconds float64 `json:"eta_seconds"`
	// Slope is the regression slope in (value/second). Positive = growing.
	Slope float64 `json:"slope"`
	// Trend is "rising", "stable", or "falling".
	Trend string `json:"trend"`
}

// SampleReader is the narrow read interface the forecast Service needs.
// *sqlite.Store satisfies it automatically.
type SampleReader interface {
	ListResourceSamples(ctx context.Context, containerID string, from, to time.Time) ([]db.ResourceSample, error)
	ListContainersByNode(ctx context.Context, nodeID string) ([]db.Container, error)
}

// Service computes capacity projections for containers belonging to a node.
type Service struct {
	store SampleReader
	// lookback is how far back to read samples for trend fitting.
	lookback time.Duration
}

// New builds a Service. lookback is typically 24h–7d.
func New(store SampleReader, lookback time.Duration) *Service {
	return &Service{store: store, lookback: lookback}
}

// ForNode returns one Projection slice per running container on the node.
// The outer slice is keyed by container ID; map[containerID][]Projection.
func (s *Service) ForNode(ctx context.Context, nodeID string) (map[string][]Projection, error) {
	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	to := time.Now()
	from := to.Add(-s.lookback)
	result := make(map[string][]Projection, len(containers))
	for _, c := range containers {
		if c.Status != "running" {
			continue
		}
		samples, err := s.store.ListResourceSamples(ctx, c.ID, from, to)
		if err != nil || len(samples) < minPoints {
			continue
		}
		result[c.ID] = projectContainer(samples)
	}
	return result, nil
}

// minPoints is the minimum number of samples needed to produce a projection.
const minPoints = 5

// projectContainer derives projections from a raw sample slice (ascending time).
func projectContainer(samples []db.ResourceSample) []Projection {
	pts := toPoints(samples)
	return []Projection{
		projectCPU(samples, pts),
		projectMemory(samples, pts),
		projectDiskWrite(samples, pts),
	}
}

func projectCPU(samples []db.ResourceSample, pts []Point) Projection {
	const threshold = 95.0 // percent
	cpuPts := make([]Point, len(pts))
	for i, s := range samples {
		cpuPts[i] = Point{T: pts[i].T, Value: s.CPUPct}
	}
	eta, slope, ok := LinearProjection(cpuPts, threshold)
	current := samples[len(samples)-1].CPUPct
	return makeProjection(MetricCPU, current, threshold, eta, slope, ok)
}

func projectMemory(samples []db.ResourceSample, pts []Point) Projection {
	const thresholdFrac = 0.90 // 90% of limit
	// Use absolute bytes; threshold = 90% of the most-recent limit.
	last := samples[len(samples)-1]
	if last.MemLimitBytes <= 0 {
		return Projection{Metric: MetricMemory, Current: float64(last.MemBytes), EtaSeconds: -1, Trend: "stable"}
	}
	threshold := thresholdFrac * float64(last.MemLimitBytes)
	memPts := make([]Point, len(pts))
	for i, s := range samples {
		memPts[i] = Point{T: pts[i].T, Value: float64(s.MemBytes)}
	}
	eta, slope, ok := LinearProjection(memPts, threshold)
	return makeProjection(MetricMemory, float64(last.MemBytes), threshold, eta, slope, ok)
}

func projectDiskWrite(samples []db.ResourceSample, pts []Point) Projection {
	// Disk writes are cumulative counters. Threshold: 2× the first observed
	// value (i.e. doubling from the window start is a useful early warning).
	first := float64(samples[0].DiskWriteBytes)
	last := float64(samples[len(samples)-1].DiskWriteBytes)
	if first <= 0 {
		return Projection{Metric: MetricDisk, Current: last, EtaSeconds: -1, Trend: "stable"}
	}
	threshold := first * 2.0
	diskPts := make([]Point, len(pts))
	for i, s := range samples {
		diskPts[i] = Point{T: pts[i].T, Value: float64(s.DiskWriteBytes)}
	}
	eta, slope, ok := LinearProjection(diskPts, threshold)
	return makeProjection(MetricDisk, last, threshold, eta, slope, ok)
}

// toPoints converts a slice of samples to []Point using epoch seconds for T.
func toPoints(samples []db.ResourceSample) []Point {
	pts := make([]Point, len(samples))
	for i, s := range samples {
		pts[i].T = float64(s.SampledAt.Unix())
	}
	return pts
}

func makeProjection(m MetricName, current, threshold, eta, slope float64, ok bool) Projection {
	p := Projection{
		Metric:    m,
		Current:   current,
		Threshold: threshold,
		Slope:     slope,
	}
	if !ok {
		p.EtaSeconds = -1
	} else {
		p.EtaSeconds = eta
	}
	p.Trend = trendLabel(slope)
	return p
}

const slopeSteadyBand = 1e-9 // treat slopes smaller than this as "stable"

func trendLabel(slope float64) string {
	if math.Abs(slope) < slopeSteadyBand {
		return "stable"
	}
	if slope > 0 {
		return "rising"
	}
	return "falling"
}

// LinearProjection fits a simple ordinary-least-squares line through points
// and returns:
//
//   - etaSeconds: how many seconds from the last observed point until the
//     trend line crosses threshold (>= 0)
//   - slope: the regression slope in value/second
//   - ok: false when the projection is meaningless (too few points, flat or
//     negative slope when threshold > current, or threshold already exceeded)
//
// The function handles edge cases:
//   - fewer than 2 points → ok=false
//   - zero/near-zero or negative slope when growth is needed → ok=false
//   - threshold already exceeded (current >= threshold) → etaSeconds≈0, ok=true
func LinearProjection(points []Point, threshold float64) (etaSeconds float64, slope float64, ok bool) {
	if len(points) < 2 {
		return 0, 0, false
	}

	n := float64(len(points))
	var sumT, sumV, sumTT, sumTV float64
	for _, p := range points {
		sumT += p.T
		sumV += p.Value
		sumTT += p.T * p.T
		sumTV += p.T * p.Value
	}
	denom := n*sumTT - sumT*sumT
	if math.Abs(denom) < 1e-12 {
		// All T values are identical — no slope determinable.
		return 0, 0, false
	}
	slope = (n*sumTV - sumT*sumV) / denom
	intercept := (sumV - slope*sumT) / n

	// Current estimate from regression at the last T.
	lastT := points[len(points)-1].T
	currentFit := slope*lastT + intercept

	// Threshold already exceeded.
	if currentFit >= threshold {
		return 0, slope, true
	}

	// Flat or falling — no ETA.
	if slope <= slopeSteadyBand {
		return 0, slope, false
	}

	// How many seconds from lastT until the line hits threshold?
	eta := (threshold - currentFit) / slope
	if eta < 0 {
		return 0, slope, false
	}
	return eta, slope, true
}
