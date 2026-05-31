package api

import (
	"net/http"
	"sort"
	"time"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/updates"
)

type updateView struct {
	ContainerID   string `json:"container_id"`
	NodeID        string `json:"node_id"`
	Image         string `json:"image"`
	Status        string `json:"status"`
	CurrentDigest string `json:"current_digest"`
	LatestDigest  string `json:"latest_digest"`
	// UnknownReason is non-empty only when status == "unknown"; it explains why
	// the digest comparison could not be made (e.g. locally-built image,
	// private registry auth failure, rate-limit). Suitable for a UI tooltip.
	UnknownReason string `json:"unknown_reason,omitempty"`
	CheckedAt     string `json:"checked_at"`
}

func toUpdateView(r db.ImageUpdateRow) updateView {
	return updateView{
		ContainerID:   r.ContainerID,
		NodeID:        r.NodeID,
		Image:         r.Image,
		Status:        r.Status,
		CurrentDigest: r.CurrentDigest,
		LatestDigest:  r.LatestDigest,
		UnknownReason: r.UnknownReason,
		CheckedAt:     r.CheckedAt.UTC().Format(time.RFC3339),
	}
}

// unknownBucket is one entry in the summary's per-category breakdown of unknown
// rows. ExampleReason is the first reason string seen for that category (handy
// for a UI tooltip).
type unknownBucket struct {
	Category      string `json:"category"`
	Count         int    `json:"count"`
	ExampleReason string `json:"example_reason"`
}

// updatesSummary aggregates the update rows so the UI can show counts and
// explain why the "unknown" rows are unknown without re-deriving categories
// client-side. dominant_* describe the highest-count category among unknown
// rows (empty string / 0 when there are no unknowns).
type updatesSummary struct {
	Total                   int             `json:"total"`
	UpToDate                int             `json:"up_to_date"`
	UpdateAvailable         int             `json:"update_available"`
	Unknown                 int             `json:"unknown"`
	DominantUnknownCategory string          `json:"dominant_unknown_category"`
	DominantUnknownCount    int             `json:"dominant_unknown_count"`
	UnknownBreakdown        []unknownBucket `json:"unknown_breakdown"`
}

// summarizeUpdates computes the aggregate summary from the persisted rows.
func summarizeUpdates(rows []db.ImageUpdateRow) updatesSummary {
	s := updatesSummary{Total: len(rows), UnknownBreakdown: []unknownBucket{}}

	// category -> count, and category -> first reason seen.
	counts := map[string]int{}
	example := map[string]string{}

	for _, row := range rows {
		switch row.Status {
		case updates.StatusUpToDate:
			s.UpToDate++
		case updates.StatusUpdateAvailable:
			s.UpdateAvailable++
		case updates.StatusUnknown:
			s.Unknown++
			cat := updates.Category(row.UnknownReason)
			counts[cat]++
			if _, ok := example[cat]; !ok {
				example[cat] = row.UnknownReason
			}
		}
	}

	for cat, n := range counts {
		s.UnknownBreakdown = append(s.UnknownBreakdown, unknownBucket{
			Category:      cat,
			Count:         n,
			ExampleReason: example[cat],
		})
	}
	// Sort by count desc, then category asc for stable output.
	sort.Slice(s.UnknownBreakdown, func(i, j int) bool {
		a, b := s.UnknownBreakdown[i], s.UnknownBreakdown[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Category < b.Category
	})

	if len(s.UnknownBreakdown) > 0 {
		s.DominantUnknownCategory = s.UnknownBreakdown[0].Category
		s.DominantUnknownCount = s.UnknownBreakdown[0].Count
	}

	return s
}

// Updates lists image update-availability across docker nodes (read-only).
// Seeds the cache on demand (TTL-bounded).
func (h *Handlers) Updates(w http.ResponseWriter, r *http.Request) {
	h.Updater.EnsureAll(r.Context())
	rows, err := h.Updater.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]updateView, len(rows))
	for i, row := range rows {
		out[i] = toUpdateView(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"updates": out,
		"summary": summarizeUpdates(rows),
	})
}

// RescanUpdates forces a fresh registry check of all docker nodes. Admin-gated
// + audited (it makes outbound registry calls).
func (h *Handlers) RescanUpdates(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodes, _ := h.Store.ListNodes(r.Context())
	for _, n := range nodes {
		h.Updater.Invalidate(n.ID)
	}
	h.Updater.EnsureAll(r.Context())
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionUpdatesRescan
	}
	w.WriteHeader(http.StatusNoContent)
}
