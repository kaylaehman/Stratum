package api

import (
	"net/http"
	"time"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
)

type updateView struct {
	ContainerID   string `json:"container_id"`
	NodeID        string `json:"node_id"`
	Image         string `json:"image"`
	Status        string `json:"status"`
	CurrentDigest string `json:"current_digest"`
	LatestDigest  string `json:"latest_digest"`
	CheckedAt     string `json:"checked_at"`
}

func toUpdateView(r db.ImageUpdateRow) updateView {
	return updateView{
		ContainerID: r.ContainerID, NodeID: r.NodeID, Image: r.Image, Status: r.Status,
		CurrentDigest: r.CurrentDigest, LatestDigest: r.LatestDigest,
		CheckedAt: r.CheckedAt.UTC().Format(time.RFC3339),
	}
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
	writeJSON(w, http.StatusOK, map[string]any{"updates": out})
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
