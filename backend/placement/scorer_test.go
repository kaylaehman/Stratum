package placement_test

import (
	"math"
	"testing"

	"github.com/kaylaehman/stratum/backend/placement"
)

const epsilon = 1e-9

// TestScore_ZeroHeadroom verifies that a node with no known metrics scores 0
// and emits "unknown" reasons for each dimension.
func TestScore_ZeroHeadroom(t *testing.T) {
	n := placement.NodeHeadroom{NodeID: "n1", NodeName: "empty"}
	score, reasons := placement.Score(n)
	if score != 0 {
		t.Errorf("expected score 0 for zero headroom, got %f", score)
	}
	for _, r := range reasons {
		_ = r // just ensure it doesn't panic
	}
	// At least the three dimension reasons should be present.
	if len(reasons) < 3 {
		t.Errorf("expected at least 3 reasons, got %d: %v", len(reasons), reasons)
	}
}

// TestScore_FullHeadroom verifies that a node with 100% CPU/RAM/disk free scores 1.
func TestScore_FullHeadroom(t *testing.T) {
	n := placement.NodeHeadroom{
		NodeID:        "n1",
		NodeName:      "full",
		CPUFree:       1.0,
		RAMFreeBytes:  8 << 30,
		RAMTotalBytes: 8 << 30,
		DiskFreeBytes: 100 << 30,
	}
	score, _ := placement.Score(n)
	if math.Abs(score-1.0) > epsilon {
		t.Errorf("expected score 1.0 for full headroom, got %f", score)
	}
}

// TestScore_CPUOnlyKnown verifies partial scoring when only CPU is known.
func TestScore_CPUOnlyKnown(t *testing.T) {
	n := placement.NodeHeadroom{
		NodeID:  "n1",
		CPUFree: 0.5,
	}
	score, reasons := placement.Score(n)
	// Score should be non-zero and bounded [0,1].
	if score <= 0 || score > 1 {
		t.Errorf("score out of [0,1]: %f", score)
	}
	// RAM and disk reasons should mention "unknown".
	found := 0
	for _, r := range reasons {
		if r == "RAM headroom unknown" || r == "disk headroom unknown" {
			found++
		}
	}
	if found < 2 {
		t.Errorf("expected 2 'unknown' reasons for RAM+disk, found %d in %v", found, reasons)
	}
}

// TestScore_Monotone verifies that higher free resources → higher score.
func TestScore_Monotone(t *testing.T) {
	low := placement.NodeHeadroom{
		NodeID:        "a",
		CPUFree:       0.2,
		RAMFreeBytes:  512 << 20,
		RAMTotalBytes: 4 << 30,
		DiskFreeBytes: 5 << 30,
	}
	high := placement.NodeHeadroom{
		NodeID:        "b",
		CPUFree:       0.8,
		RAMFreeBytes:  3 << 30,
		RAMTotalBytes: 4 << 30,
		DiskFreeBytes: 50 << 30,
	}
	scoreLow, _ := placement.Score(low)
	scoreHigh, _ := placement.Score(high)
	if scoreHigh <= scoreLow {
		t.Errorf("expected higher-resource node to score higher: low=%f high=%f", scoreLow, scoreHigh)
	}
}

// TestScore_USBHintAppearsInReasons verifies that USB passthrough adds a reason.
func TestScore_USBHintAppearsInReasons(t *testing.T) {
	n := placement.NodeHeadroom{
		NodeID:         "n1",
		CPUFree:        0.5,
		USBPassthrough: true,
	}
	_, reasons := placement.Score(n)
	for _, r := range reasons {
		if r == "USB devices discoverable (passthrough possible)" {
			return
		}
	}
	t.Errorf("USB reason not found in %v", reasons)
}

// TestScore_BoundedOutput ensures score is always in [0,1] even for extreme inputs.
func TestScore_BoundedOutput(t *testing.T) {
	cases := []placement.NodeHeadroom{
		{CPUFree: 2.0, RAMFreeBytes: 100 << 30, RAMTotalBytes: 1 << 30, DiskFreeBytes: 1000 << 30},
		{CPUFree: -1.0, RAMFreeBytes: -1, DiskFreeBytes: -1},
	}
	for _, n := range cases {
		score, _ := placement.Score(n)
		if score < 0 || score > 1 {
			t.Errorf("score %f out of [0,1] for headroom %+v", score, n)
		}
	}
}

// TestScore_ReasonsIncludeFreeValues checks that reasons mention the actual values.
func TestScore_ReasonsIncludeFreeValues(t *testing.T) {
	n := placement.NodeHeadroom{
		NodeID:        "n1",
		CPUFree:       0.72,
		RAMFreeBytes:  3 << 30,
		RAMTotalBytes: 4 << 30,
		DiskFreeBytes: 50 << 30,
	}
	_, reasons := placement.Score(n)
	foundCPU, foundRAM, foundDisk := false, false, false
	for _, r := range reasons {
		if containsAny(r, "CPU free") {
			foundCPU = true
		}
		if containsAny(r, "RAM free") {
			foundRAM = true
		}
		if containsAny(r, "disk free") {
			foundDisk = true
		}
	}
	if !foundCPU {
		t.Errorf("no CPU free reason in %v", reasons)
	}
	if !foundRAM {
		t.Errorf("no RAM free reason in %v", reasons)
	}
	if !foundDisk {
		t.Errorf("no disk free reason in %v", reasons)
	}
}

func containsAny(s, sub string) bool {
	return len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()
}
