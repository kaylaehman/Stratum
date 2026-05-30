package api

import (
	"net/http"
	"time"

	"github.com/kaylaehman/stratum/backend/incident"
)

// IncidentTimeline handles GET /api/incidents/timeline.
// Query params:
//
//	from  – RFC3339 or YYYY-MM-DD; defaults to 24h ago
//	to    – RFC3339 or YYYY-MM-DD; defaults to now
//	node_id – optional; restricts results to one node
//
// Read-only, behind auth middleware. No activity-log write.
func (h *Handlers) IncidentTimeline(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	from, fromOK := parseOptTime(q.Get("from"), false)
	if !fromOK {
		writeError(w, http.StatusBadRequest, "invalid_from")
		return
	}
	to, toOK := parseOptTime(q.Get("to"), true)
	if !toOK {
		writeError(w, http.StatusBadRequest, "invalid_to")
		return
	}

	iq := incident.Query{NodeID: q.Get("node_id")}
	if from != nil {
		iq.From = *from
	}
	if to != nil {
		iq.To = *to
	}
	if iq.To.IsZero() {
		iq.To = time.Now()
	}
	if iq.From.IsZero() {
		iq.From = iq.To.Add(-24 * time.Hour)
	}

	entries, err := incident.BuildTimeline(r.Context(), h.Store, iq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	type entryView struct {
		Timestamp  string `json:"timestamp"`
		Source     string `json:"source"`
		Severity   string `json:"severity"`
		NodeID     string `json:"node_id,omitempty"`
		TargetID   string `json:"target_id,omitempty"`
		TargetType string `json:"target_type,omitempty"`
		Summary    string `json:"summary"`
		DeepLink   string `json:"deep_link,omitempty"`
	}

	views := make([]entryView, len(entries))
	for i, e := range entries {
		views[i] = entryView{
			Timestamp:  e.Timestamp.UTC().Format(time.RFC3339),
			Source:     string(e.Source),
			Severity:   string(e.Severity),
			NodeID:     e.NodeID,
			TargetID:   e.TargetID,
			TargetType: e.TargetType,
			Summary:    e.Summary,
			DeepLink:   e.DeepLink,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": views,
		"from":    iq.From.UTC().Format(time.RFC3339),
		"to":      iq.To.UTC().Format(time.RFC3339),
	})
}
