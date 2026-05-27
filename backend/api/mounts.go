package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/fs"
)

// resolveNodeDocker loads a node by URL :id and enforces the docker capability.
func (h *Handlers) resolveNodeDocker(w http.ResponseWriter, r *http.Request) (db.Node, bool) {
	node, err := h.Store.GetNode(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return db.Node{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return db.Node{}, false
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if err := capabilities.Require(caps, capabilities.Docker); err != nil {
		writeCapabilityUnavailable(w, "docker", "")
		return db.Node{}, false
	}
	return node, true
}

// ContainerMounts lists a container's mounts (forward), with the shared flag.
func (h *Handlers) ContainerMounts(w http.ResponseWriter, r *http.Request) {
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	views, err := h.Mounts.Forward(r.Context(), ctr.NodeID, ctr.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "mount_index_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mounts": views})
}

// ReverseMounts lists containers mounting host_path or any parent/child of it.
func (h *Handlers) ReverseMounts(w http.ResponseWriter, r *http.Request) {
	node, ok := h.resolveNodeDocker(w, r)
	if !ok {
		return
	}
	hostPath, err := fs.ValidatePath(r.URL.Query().Get("host_path"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_path")
		return
	}
	hits, err := h.Mounts.Reverse(r.Context(), node.ID, hostPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "mount_index_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": hits})
}

// SharedMounts lists host paths / volumes mounted into more than one container.
func (h *Handlers) SharedMounts(w http.ResponseWriter, r *http.Request) {
	node, ok := h.resolveNodeDocker(w, r)
	if !ok {
		return
	}
	shared, err := h.Mounts.Shared(r.Context(), node.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "mount_index_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shared": shared})
}
