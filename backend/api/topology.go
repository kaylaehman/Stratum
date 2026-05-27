package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
)

// NodeTopology returns the Docker network topology for a node (read-only).
func (h *Handlers) NodeTopology(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Docker {
		writeError(w, http.StatusConflict, "docker_not_available")
		return
	}
	topo, err := h.Topology.ForNode(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "node_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, topo)
}
