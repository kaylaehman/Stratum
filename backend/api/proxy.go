package api

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
)

// validateProxyEndpoint ensures a proxy admin endpoint is a host-only http(s)
// base URL (the adapter appends its own API path), preventing path/query
// smuggling on the server-side fetch.
func validateProxyEndpoint(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("endpoint must be http or https")
	}
	if u.Host == "" {
		return errors.New("endpoint must include a host")
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("endpoint must not include a path, query, or fragment")
	}
	return nil
}

// NodeProxy returns the detected reverse-proxy tool for a node, its rules (when
// listable + configured), and the supported-tools catalog. Admin-gated (proxy
// config can reveal infrastructure topology).
func (h *Handlers) NodeProxy(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	st, err := h.Proxy.Status(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, st)
}

type proxyConfigRequest struct {
	Endpoint string  `json:"endpoint"`
	Token    *string `json:"token"` // nil = keep existing; "" = clear
}

// SetNodeProxyConfig stores a node's proxy admin endpoint + optional token
// (admin). The token is sealed; never logged or echoed. Audited.
func (h *Handlers) SetNodeProxyConfig(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	var req proxyConfigRequest
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
	if err := h.Proxy.SetConfig(r.Context(), nodeID, req.Endpoint, req.Token); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionProxyConfig
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
	}
	st, err := h.Proxy.Status(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, st)
}
