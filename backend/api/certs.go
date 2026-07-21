package api

import (
	"net/http"

	"github.com/KAE-Labs/stratum/backend/activity"
)

// CertList returns all discovered certificates across nodes (admin), sorted by
// expiry. days_remaining is computed client-side from not_after.
func (h *Handlers) CertList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	list, err := h.Certs.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"certs": list})
}

// CertRescan forces a fresh cert scan across all nodes (admin). Audited.
func (h *Handlers) CertRescan(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.Certs.RescanAll(r.Context())
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionCertRescan
		e.TargetType = ptr(activity.TargetNode)
	}
	list, err := h.Certs.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"certs": list})
}
