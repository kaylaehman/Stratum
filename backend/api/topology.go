package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
)

// topologyTimeout bounds the N+1 network inspects so a slow daemon can't hold
// the request open indefinitely.
const topologyTimeout = 10 * time.Second

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
	ctx, cancel := context.WithTimeout(r.Context(), topologyTimeout)
	defer cancel()
	topo, err := h.Topology.ForNode(ctx, nodeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "node_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, topo)
}
