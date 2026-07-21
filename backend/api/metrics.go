package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/metrics"
)

// metricsRange maps a range selector to a lookback duration. Default 1h.
func metricsRange(v string) time.Duration {
	switch v {
	case "6h":
		return 6 * time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	default:
		return time.Hour
	}
}

// maxTimelinePoints caps the points returned per series (downsampled).
const maxTimelinePoints = 500

type sampleView struct {
	SampledAt      string  `json:"sampled_at"`
	CPUPct         float64 `json:"cpu_pct"`
	MemBytes       int64   `json:"mem_bytes"`
	MemLimitBytes  int64   `json:"mem_limit_bytes"`
	DiskReadBytes  int64   `json:"disk_read_bytes"`
	DiskWriteBytes int64   `json:"disk_write_bytes"`
}

func toSampleView(s db.ResourceSample) sampleView {
	return sampleView{
		SampledAt:      s.SampledAt.UTC().Format(time.RFC3339),
		CPUPct:         s.CPUPct,
		MemBytes:       s.MemBytes,
		MemLimitBytes:  s.MemLimitBytes,
		DiskReadBytes:  s.DiskReadBytes,
		DiskWriteBytes: s.DiskWriteBytes,
	}
}

// ContainerMetrics returns the downsampled resource timeline + detected spikes
// for a container over the requested range. Read-only.
func (h *Handlers) ContainerMetrics(w http.ResponseWriter, r *http.Request) {
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	to := time.Now()
	from := to.Add(-metricsRange(r.URL.Query().Get("range")))
	samples, err := h.Store.ListResourceSamples(r.Context(), ctr.ID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	spikes := metrics.DetectSpikes(samples)
	down := metrics.Downsample(samples, maxTimelinePoints)
	views := make([]sampleView, len(down))
	for i, s := range down {
		views[i] = toSampleView(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"samples": views, "spikes": spikes})
}

// ContainerMetricsCSV streams the full (non-downsampled) timeline as CSV.
func (h *Handlers) ContainerMetricsCSV(w http.ResponseWriter, r *http.Request) {
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	to := time.Now()
	from := to.Add(-metricsRange(r.URL.Query().Get("range")))
	samples, err := h.Store.ListResourceSamples(r.Context(), ctr.ID, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="metrics.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"timestamp", "cpu_pct", "mem_bytes", "mem_limit_bytes", "disk_read_bytes", "disk_write_bytes"})
	for _, s := range samples {
		_ = cw.Write([]string{
			s.SampledAt.UTC().Format(time.RFC3339),
			strconv.FormatFloat(s.CPUPct, 'f', 2, 64),
			strconv.FormatInt(s.MemBytes, 10),
			strconv.FormatInt(s.MemLimitBytes, 10),
			strconv.FormatInt(s.DiskReadBytes, 10),
			strconv.FormatInt(s.DiskWriteBytes, 10),
		})
	}
	cw.Flush()
}
