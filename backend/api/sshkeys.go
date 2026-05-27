package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/sshkeys"
)

// ListSSHKeys audits authorized_keys across a node's users over SSH. Admin-gated
// (exposes key fingerprints/comments across all users).
func (h *Handlers) ListSSHKeys(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	keys, err := sshkeys.Audit(r.Context(), nodeID, h.Files.Exec)
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

type deleteKeyBody struct {
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
}

// DeleteSSHKey removes a key (by fingerprint) from an authorized_keys file.
// Admin-gated + audited.
func (h *Handlers) DeleteSSHKey(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	var body deleteKeyBody
	if err := decodeJSON(r, &body); err != nil || body.Fingerprint == "" || !sshkeys.ValidKeyPath(body.Path) {
		writeError(w, http.StatusBadRequest, "path_and_fingerprint_required")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionSSHKeyDelete
		e.TargetType = ptr(activity.TargetSSHKey)
		e.TargetID = &body.Fingerprint
		e.Detail = map[string]string{"node_id": nodeID, "path": body.Path}
	}

	writeFile := func(ctx context.Context, nid, p string, content []byte) error {
		return h.Files.Write(ctx, nid, p, content, nil)
	}
	err := sshkeys.DeleteKey(r.Context(), nodeID, body.Path, body.Fingerprint, h.Files.Exec, writeFile)
	switch {
	case errors.Is(err, sshkeys.ErrKeyNotFound):
		writeError(w, http.StatusNotFound, "key_not_found")
		return
	case err != nil:
		writeError(w, http.StatusBadGateway, "delete_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
