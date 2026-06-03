package agent

import (
	"math/rand/v2"
	"testing"
	"time"
)

// deterministicSource implements rand.Source using a fixed sequence so tests
// are reproducible.
type deterministicSource struct{ values []uint64; idx int }

func (d *deterministicSource) Uint64() uint64 {
	v := d.values[d.idx%len(d.values)]
	d.idx++
	return v
}

func TestBackoffSchedule(t *testing.T) {
	// Use a source that always returns the maximum uint64 so jitter is always
	// ceiling-1 (worst case = full cap exposure).
	maxSrc := &deterministicSource{values: []uint64{^uint64(0)}}
	b := backoffConfig{base: 1 * time.Second, max: 30 * time.Second, factor: 2.0, randSrc: maxSrc}

	cases := []struct {
		attempt     int
		wantCeiling time.Duration // jitter upper bound for this attempt
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second}, // capped
		{6, 30 * time.Second}, // still capped
		{99, 30 * time.Second}, // large attempt stays capped
	}

	for _, tc := range cases {
		d := b.next(tc.attempt)
		if d < 0 {
			t.Errorf("attempt %d: got negative delay %v", tc.attempt, d)
		}
		if d >= tc.wantCeiling {
			t.Errorf("attempt %d: delay %v >= ceiling %v", tc.attempt, d, tc.wantCeiling)
		}
	}
}

func TestBackoffNeverExceedsCap(t *testing.T) {
	b := backoffConfig{base: 1 * time.Second, max: 30 * time.Second, factor: 2.0}
	cap := 30 * time.Second
	for i := 0; i < 200; i++ {
		if d := b.next(i); d > cap {
			t.Errorf("attempt %d: delay %v > max %v", i, d, cap)
		}
	}
}

func TestBackoffJitterBounds(t *testing.T) {
	// Run many samples and verify they are in [0, max).
	b := backoffConfig{base: 1 * time.Second, max: 30 * time.Second, factor: 2.0}
	for i := 0; i < 1000; i++ {
		d := b.next(5) // attempt 5 → ceiling=30s
		if d < 0 || d >= 30*time.Second {
			t.Fatalf("jitter out of bounds: %v", d)
		}
	}
}

func TestBackoffZeroMax(t *testing.T) {
	b := backoffConfig{base: 0, max: 0, factor: 2.0}
	if d := b.next(0); d != 0 {
		t.Errorf("expected 0, got %v", d)
	}
}

func TestBackoffDeterministicSource(t *testing.T) {
	// Verify that injecting a fixed source produces repeatable results.
	src := rand.NewPCG(42, 0)
	b1 := backoffConfig{base: 1 * time.Second, max: 10 * time.Second, factor: 2.0, randSrc: src}
	src2 := rand.NewPCG(42, 0)
	b2 := backoffConfig{base: 1 * time.Second, max: 10 * time.Second, factor: 2.0, randSrc: src2}

	for attempt := 0; attempt < 5; attempt++ {
		d1 := b1.next(attempt)
		d2 := b2.next(attempt)
		if d1 != d2 {
			t.Errorf("attempt %d: non-deterministic (%v != %v)", attempt, d1, d2)
		}
	}
}
