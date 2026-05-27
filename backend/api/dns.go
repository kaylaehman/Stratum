package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
)

// NodeDNS returns the detected DNS tool for a node, its records (when listable +
// configured), and the supported-tools catalog. Admin-gated.
func (h *Handlers) NodeDNS(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	st, err := h.DNS.Status(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// SetNodeDNSConfig stores a node's DNS admin endpoint + optional token (admin).
// Token is sealed; never logged or echoed. Audited.
func (h *Handlers) SetNodeDNSConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	var req proxyConfigRequest // same shape: {endpoint, token?}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.Endpoint != "" {
		if err := validateProxyEndpoint(req.Endpoint); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_endpoint")
			return
		}
	}
	if err := h.DNS.SetConfig(r.Context(), nodeID, req.Endpoint, req.Token); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionDNSConfig
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
	}
	st, err := h.DNS.Status(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, st)
}
