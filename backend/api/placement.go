package api

import (
	"net/http"

	"github.com/kaylaehman/stratum/backend/placement"
)

// RecommendPlacement handles GET /api/placement/recommend.
// Returns docker-capable nodes ranked by available headroom, best-first.
// Admin-gated, read-only, not audited (no mutation).
//
// Response body:
//
//	{
//	  "recommendations": [
//	    {
//	      "node_id":   "abc123",
//	      "node_name": "ubuntu-01",
//	      "score":     0.87,
//	      "reasons":   ["72% CPU free", "3.2 GiB RAM free", "disk headroom unknown"],
//	      "headroom": {
//	        "node_id":         "abc123",
//	        "node_name":       "ubuntu-01",
//	        "cpu_free":        0.72,
//	        "ram_free_bytes":  3435134976,
//	        "ram_total_bytes": 8589934592,
//	        "disk_free_bytes": 0,
//	        "usb_passthrough": false
//	      }
//	    }
//	  ]
//	}
func (h *Handlers) RecommendPlacement(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	recs, err := h.Placement.Recommend(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "placement_error")
		return
	}

	// Ensure a non-null array in the response even when there are no results.
	if recs == nil {
		recs = []placement.Recommendation{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"recommendations": recs,
	})
}
