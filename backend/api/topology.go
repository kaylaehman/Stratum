package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/nodeconn"
)

// topologyTimeout bounds the N+1 network inspects so a slow daemon can't hold
// the request open indefinitely. 45s is generous for homelab daemons: each
// NetworkInspect RPC is fast individually but N networks × a slow daemon can
// accumulate. The transport-level responseHeaderTimeout (30s, set in
// docker.newHTTPClient) handles a fully hung TCP connection before this fires.
const topologyTimeout = 45 * time.Second

// NodeTopology returns the network topology for a node (read-only).
// The node-level reachability status is always sourced from the DB (owned by
// the poller) and returned in the response body, so the frontend never has to
// infer reachability from a Docker transport failure.
// A 502 is only returned when the DB itself cannot be queried or the node
// record is not found — not when Docker is merely unavailable.
func (h *Handlers) NodeTopology(w http.ResponseWriter, r *http.Request) {
	// Network topology is host reconnaissance — operator-gated, matching the
	// host-FS reads (a read-only viewer has no need to map the node's network).
	if !h.requireOperator(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	_, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), topologyTimeout)
	defer cancel()
	topo, err := h.Topology.ForNode(ctx, nodeID)
	if err != nil && nodeconn.IsTransportError(err) {
		// Stale keep-alive connection (e.g. daemon restarted): rebuild the
		// cached client and retry once before returning an error to the UI.
		h.Logger.Warn("topology: transport error on cached client; rebuilding and retrying",
			"node", nodeID, "error", err)
		if _, rerr := h.Conn.Rebuild(ctx, nodeID); rerr == nil {
			topo, err = h.Topology.ForNode(ctx, nodeID)
		}
	}
	if err != nil {
		// Only a genuine store error reaches here (Docker failures are absorbed
		// into topo.DockerError by the service layer).
		h.Logger.Warn("topology: service call failed", "node", nodeID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, topo)
}
