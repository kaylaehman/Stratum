package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/forecast"
)

// NodeForecast returns capacity projections for all running containers on a
// node. Admin-gated; read-only.
func (h *Handlers) NodeForecast(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	if _, err := h.Store.GetNode(r.Context(), nodeID); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found")
		} else {
			writeError(w, http.StatusInternalServerError, "internal_error")
		}
		return
	}
	projections, err := h.Forecast.ForNode(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	type containerProjections struct {
		ContainerID string                `json:"container_id"`
		Projections []forecast.Projection `json:"projections"`
	}
	out := make([]containerProjections, 0, len(projections))
	for cID, projs := range projections {
		out = append(out, containerProjections{ContainerID: cID, Projections: projs})
	}
	writeJSON(w, http.StatusOK, map[string]any{"forecast": out})
}
