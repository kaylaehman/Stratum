// Package placement implements the C8 service-placement advisor: it ranks
// docker-capable nodes by available headroom (CPU, RAM, disk) and returns an
// ordered list of recommendations. This is a hint, not a scheduler — it never
// moves workloads autonomously.
package placement

import "fmt"

// NodeHeadroom is the observed resource state of one docker-capable node,
// collected from the latest metric samples and node metadata. All fields are
// percentages or bytes; zero values are treated as unknown (not zero capacity).
type NodeHeadroom struct {
	NodeID   string
	NodeName string

	// CPU headroom: fraction free (0.0–1.0). A value of 0 means unknown.
	CPUFree float64

	// RAM headroom: bytes free. 0 means unknown.
	RAMFreeBytes int64

	// RAM total on the node, used to compute a free-fraction for scoring.
	// 0 means unknown.
	RAMTotalBytes int64

	// Disk free bytes at the typical Docker storage root. 0 means unknown.
	DiskFreeBytes int64

	// USBPassthrough indicates whether the node has discoverable USB devices
	// that could be passed through to a container. This is a soft signal only.
	USBPassthrough bool
}

// scoreWeights control the relative importance of each resource dimension.
// They sum to 1.0 so the returned score is bounded [0, 1].
const (
	weightCPU  = 0.45
	weightRAM  = 0.45
	weightDisk = 0.10
)

// Score computes a [0, 1] placement suitability score for a node together with
// human-readable reasons explaining the result. Higher scores mean more
// headroom. Dimensions with unknown values (zero) are excluded from the
// weighted average and a "unknown" reason is emitted for them instead.
//
// Score is a pure function with no I/O — all data comes from NodeHeadroom.
// This makes it trivially testable without mocks.
func Score(n NodeHeadroom) (score float64, reasons []string) {
	var total, weightUsed float64

	// --- CPU ---
	if n.CPUFree > 0 {
		cpuScore := clamp(n.CPUFree)
		total += cpuScore * weightCPU
		weightUsed += weightCPU
		reasons = append(reasons, fmt.Sprintf("%.0f%% CPU free", n.CPUFree*100))
	} else {
		reasons = append(reasons, "CPU headroom unknown")
	}

	// --- RAM ---
	if n.RAMFreeBytes > 0 {
		var ramScore float64
		if n.RAMTotalBytes > 0 {
			ramScore = clamp(float64(n.RAMFreeBytes) / float64(n.RAMTotalBytes))
		} else {
			// No total available: map absolute free bytes on a 0–8 GiB scale.
			ramScore = clamp(float64(n.RAMFreeBytes) / float64(8<<30))
		}
		total += ramScore * weightRAM
		weightUsed += weightRAM
		reasons = append(reasons, fmt.Sprintf("%s RAM free", humanBytes(n.RAMFreeBytes)))
	} else {
		reasons = append(reasons, "RAM headroom unknown")
	}

	// --- Disk ---
	if n.DiskFreeBytes > 0 {
		// Map free bytes on a 0–100 GiB scale.
		diskScore := clamp(float64(n.DiskFreeBytes) / float64(100<<30))
		total += diskScore * weightDisk
		weightUsed += weightDisk
		reasons = append(reasons, fmt.Sprintf("%s disk free", humanBytes(n.DiskFreeBytes)))
	} else {
		reasons = append(reasons, "disk headroom unknown")
	}

	// USB passthrough is an informational hint, not a scoring factor.
	if n.USBPassthrough {
		reasons = append(reasons, "USB devices discoverable (passthrough possible)")
	}

	if weightUsed == 0 {
		return 0, reasons
	}
	return total / weightUsed, reasons
}

// clamp ensures v is in [0, 1].
func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// humanBytes formats a byte count in a human-readable unit (GiB or MiB).
func humanBytes(b int64) string {
	if b >= 1<<30 {
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1<<30))
	}
	return fmt.Sprintf("%d MiB", b>>20)
}
