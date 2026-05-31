package forecast

import (
	"math"
	"testing"
)

// approxEqual returns true when a and b differ by less than epsilon.
func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

// TestLinearProjection_RisingSeries checks that a cleanly rising series
// produces the expected ETA with the correct sign.
func TestLinearProjection_RisingSeries(t *testing.T) {
	// 10 points, value = T (slope 1.0 value/sec), threshold = 20.
	// Last point T=9, value=9. ETA = (20-9)/1.0 = 11 s.
	pts := make([]Point, 10)
	for i := range pts {
		pts[i] = Point{T: float64(i), Value: float64(i)}
	}
	eta, slope, ok := LinearProjection(pts, 20.0)
	if !ok {
		t.Fatal("expected ok=true for rising series")
	}
	if slope <= 0 {
		t.Errorf("slope should be positive, got %f", slope)
	}
	// ETA should be ~11 seconds (threshold 20, current fit at T=9 is 9).
	if !approxEqual(eta, 11.0, 0.5) {
		t.Errorf("eta: want ≈11, got %f", eta)
	}
}

// TestLinearProjection_FastRise tests a steeper slope.
func TestLinearProjection_FastRise(t *testing.T) {
	// value = 2T, threshold = 50, last T=9, fit=18, eta=(50-18)/2 = 16.
	pts := make([]Point, 10)
	for i := range pts {
		pts[i] = Point{T: float64(i), Value: float64(i) * 2}
	}
	eta, slope, ok := LinearProjection(pts, 50.0)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if !approxEqual(slope, 2.0, 0.01) {
		t.Errorf("slope: want ≈2, got %f", slope)
	}
	if !approxEqual(eta, 16.0, 0.5) {
		t.Errorf("eta: want ≈16, got %f", eta)
	}
}

// TestLinearProjection_FlatSeries — no ETA.
func TestLinearProjection_FlatSeries(t *testing.T) {
	pts := make([]Point, 10)
	for i := range pts {
		pts[i] = Point{T: float64(i), Value: 42.0}
	}
	_, _, ok := LinearProjection(pts, 100.0)
	if ok {
		t.Fatal("expected ok=false for flat series (threshold not reachable)")
	}
}

// TestLinearProjection_FallingSeries — falling toward a lower threshold: no ETA.
func TestLinearProjection_FallingSeries(t *testing.T) {
	pts := make([]Point, 10)
	for i := range pts {
		pts[i] = Point{T: float64(i), Value: float64(100 - i)}
	}
	// Threshold is 150 (above the series); slope is -1, so will never reach 150.
	_, _, ok := LinearProjection(pts, 150.0)
	if ok {
		t.Fatal("expected ok=false for falling series with threshold above")
	}
}

// TestLinearProjection_FewPoints — fewer than 2 points → ok=false.
func TestLinearProjection_FewPoints(t *testing.T) {
	tests := [][]Point{
		{},
		{{T: 0, Value: 5}},
	}
	for _, pts := range tests {
		_, _, ok := LinearProjection(pts, 10.0)
		if ok {
			t.Errorf("expected ok=false for %d points", len(pts))
		}
	}
}

// TestLinearProjection_ThresholdAlreadyCrossed — current >= threshold → eta≈0, ok=true.
func TestLinearProjection_ThresholdAlreadyCrossed(t *testing.T) {
	// Rising series, values 10..19, threshold = 5 (already exceeded).
	pts := make([]Point, 10)
	for i := range pts {
		pts[i] = Point{T: float64(i), Value: float64(10 + i)}
	}
	eta, _, ok := LinearProjection(pts, 5.0)
	if !ok {
		t.Fatal("expected ok=true when threshold already exceeded")
	}
	if eta != 0 {
		t.Errorf("eta: want 0, got %f", eta)
	}
}

// TestLinearProjection_NoiseHandling — noisy but clearly rising series.
func TestLinearProjection_NoiseHandling(t *testing.T) {
	// Values roughly follow y = T + noise; should still give a positive slope.
	raw := []float64{0.1, 1.9, 2.2, 3.8, 4.1, 5.9, 6.0, 7.8, 8.1, 9.9}
	pts := make([]Point, len(raw))
	for i, v := range raw {
		pts[i] = Point{T: float64(i), Value: v}
	}
	_, slope, ok := LinearProjection(pts, 20.0)
	if !ok {
		t.Fatal("expected ok=true for noisy rising series")
	}
	if slope <= 0 {
		t.Errorf("expected positive slope for rising data, got %f", slope)
	}
}

// TestLinearProjection_AllSameT — degenerate: all points at same T.
func TestLinearProjection_AllSameT(t *testing.T) {
	pts := []Point{{T: 5, Value: 1}, {T: 5, Value: 2}, {T: 5, Value: 3}}
	_, _, ok := LinearProjection(pts, 10.0)
	if ok {
		t.Fatal("expected ok=false when all T values are identical")
	}
}

// TestTrendLabel checks the label assignment.
func TestTrendLabel(t *testing.T) {
	cases := []struct {
		slope float64
		want  string
	}{
		{1.0, "rising"},
		{-1.0, "falling"},
		{0.0, "stable"},
		{5e-10, "stable"}, // within slopeSteadyBand
	}
	for _, tc := range cases {
		got := trendLabel(tc.slope)
		if got != tc.want {
			t.Errorf("trendLabel(%f) = %q, want %q", tc.slope, got, tc.want)
		}
	}
}

// TestMakeProjection_EtaNegativeOneWhenNotOk checks that ok=false results in EtaSeconds=-1.
func TestMakeProjection_EtaNegativeOneWhenNotOk(t *testing.T) {
	p := makeProjection(MetricCPU, 50.0, 95.0, 0, 0, false)
	if p.EtaSeconds != -1 {
		t.Errorf("want EtaSeconds=-1, got %f", p.EtaSeconds)
	}
}
