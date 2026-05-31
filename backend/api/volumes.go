package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/volumes"
)

// volumeListNodeTimeout bounds a single node's volume listing. The VolumeList
// call inside is fast; the DiskUsage enrichment has its own inner timeout (see
// docker.dfTimeout). This outer deadline guards against hung TCP connections or
// extremely slow stores.
const volumeListNodeTimeout = 30 * time.Second

// ListVolumes lists volumes across all docker-capable nodes with health status.
// Read-only; available to any authenticated user.
func (h *Handlers) ListVolumes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodes, err := h.Store.ListNodes(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := []volumes.VolumeView{}
	for _, n := range nodes {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if !caps.Docker {
			continue
		}
		// Bound each node so a slow/hung daemon can't stall the whole response.
		nodeCtx, cancel := context.WithTimeout(ctx, volumeListNodeTimeout)
		vols, err := h.Volumes.ListForNode(nodeCtx, n.ID)
		cancel()
		if err != nil {
			// Log so operators can diagnose why a node's volumes are absent,
			// but do not fail the whole cross-node list.
			slog.Warn("volumes: list for node failed, skipping", "node", n.ID, "name", n.Name, "error", err)
			continue
		}
		out = append(out, vols...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"volumes": out})
}

// RemoveVolume deletes an unused volume on a node. Admin-gated + audited.
func (h *Handlers) RemoveVolume(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "volume_name_required")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionVolumeRemove
		e.TargetType = ptr(activity.TargetVolume)
		e.TargetID = &name
		e.Detail = map[string]string{"node_id": nodeID, "volume": name}
	}

	err := h.Volumes.Remove(r.Context(), nodeID, name)
	switch {
	case errors.Is(err, volumes.ErrVolumeInUse):
		writeError(w, http.StatusConflict, "volume_in_use")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "remove_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pruneVolumeResult is the per-volume outcome in the prune response.
type pruneVolumeResult struct {
	NodeID string `json:"node_id"`
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

// PruneUnusedVolumes removes every volume the service classifies as "unused" on
// the target node (or all docker-capable nodes when node_id is omitted). The
// candidate set is recomputed server-side — the client never supplies a name
// list. Admin-gated + step-up + audited.
func (h *Handlers) PruneUnusedVolumes(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}

	var body struct {
		NodeID string `json:"node_id"`
	}
	// Tolerate an empty body (=> prune all docker-capable nodes). Only a
	// malformed (non-empty, non-JSON) body is rejected.
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body")
			return
		}
	}

	results, err := h.Volumes.PruneUnused(r.Context(), body.NodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "prune_failed")
		return
	}

	out := make([]pruneVolumeResult, 0, len(results))
	removed, failed := 0, 0
	for _, res := range results {
		if res.OK {
			removed++
		} else {
			failed++
		}
		out = append(out, pruneVolumeResult{NodeID: res.NodeID, Name: res.Name, OK: res.OK, Error: res.Error})
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionVolumePruneUnused
		e.TargetType = ptr(activity.TargetVolume)
		e.Detail = map[string]any{
			"node_id":       body.NodeID,
			"removed_count": removed,
			"failed_count":  failed,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results":       out,
		"removed_count": removed,
		"failed_count":  failed,
	})
}
