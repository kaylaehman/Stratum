package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/nodeconn"
)

// depgraphTimeout bounds the network + mount-index calls. 45s matches the
// topology timeout: both do N+1 Docker RPCs and mount-index refresh on homelab
// daemons that may be briefly slow.
const depgraphTimeout = 45 * time.Second

// NodeDependencyGraph returns the container/network/volume dependency graph for
// a node (read-only).
func (h *Handlers) NodeDependencyGraph(w http.ResponseWriter, r *http.Request) {
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
	ctx, cancel := context.WithTimeout(r.Context(), depgraphTimeout)
	defer cancel()
	g, err := h.DepGraph.ForNode(ctx, nodeID)
	if err != nil && nodeconn.IsTransportError(err) {
		h.Logger.Warn("depgraph: transport error on cached client; rebuilding and retrying",
			"node", nodeID, "error", err)
		if _, rerr := h.Conn.Rebuild(ctx, nodeID); rerr == nil {
			g, err = h.DepGraph.ForNode(ctx, nodeID)
		}
	}
	if err != nil {
		h.Logger.Warn("depgraph: docker call failed", "node", nodeID, "error", err)
		writeError(w, http.StatusBadGateway, "node_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, g)
}
