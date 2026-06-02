package api

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/proxy"
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
	// Kind selects the proxy provider explicitly, overriding image detection.
	// "" keeps the existing config; "auto" clears the override (back to
	// detection); "cloudflare-api" selects the Cloudflare-API provider and uses
	// AccountID/TunnelID below.
	Kind      string `json:"kind,omitempty"`
	AccountID string `json:"account_id,omitempty"`
	TunnelID  string `json:"tunnel_id,omitempty"`
}

// proxyConfigMap converts the request's kind/account/tunnel fields into the
// config map persisted as config_json. Returns nil to preserve existing config.
func (req proxyConfigRequest) proxyConfigMap() map[string]string {
	switch req.Kind {
	case "":
		return nil // preserve existing config
	case "auto":
		return map[string]string{} // clear override → image auto-detection
	default:
		m := map[string]string{"kind": req.Kind}
		if req.AccountID != "" {
			m["account_id"] = req.AccountID
		}
		if req.TunnelID != "" {
			m["tunnel_id"] = req.TunnelID
		}
		return m
	}
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
	if err := h.Proxy.SetConfig(r.Context(), nodeID, req.Endpoint, req.Token, req.proxyConfigMap()); err != nil {
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

type cloudflareDiscoverRequest struct {
	Token     string `json:"token,omitempty"`      // optional override; falls back to stored token
	AccountID string `json:"account_id,omitempty"` // optional; resolves tunnels for this account
}

// DiscoverProxyCloudflare lists the Cloudflare accounts a token can see and the
// tunnels of the chosen/single account, powering the setup picker for the
// cloudflare-api provider. Admin-gated. Not audited (read-only discovery); the
// token is used in-memory only and never persisted by this handler. Cloudflare
// errors (invalid token, missing scope) are surfaced to the UI verbatim.
func (h *Handlers) DiscoverProxyCloudflare(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	var req cloudflareDiscoverRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	res, err := h.Proxy.DiscoverCloudflare(r.Context(), nodeID, req.Token, req.AccountID)
	if err != nil {
		// Partial success: the account list was fetched but resolving its tunnels
		// failed. Return 200 with the accounts + an embedded error so the picker
		// can still render the account selector.
		if len(res.Accounts) > 0 {
			res.Error = err.Error()
			writeJSON(w, http.StatusOK, res)
			return
		}
		// Hard failure: map to an accurate status and surface the message.
		status := http.StatusBadGateway // upstream Cloudflare error (bad token, scope, etc.)
		switch {
		case errors.Is(err, proxy.ErrTokenRequired):
			status = http.StatusBadRequest
		case errors.Is(err, proxy.ErrNoAdapter):
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}
