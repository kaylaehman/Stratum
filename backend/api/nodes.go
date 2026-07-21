package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/middleware"
	"github.com/KAE-Labs/stratum/backend/nodes"
)

// credentialsBody is the secret material accepted on input. It is never echoed
// back in any response.
type credentialsBody struct {
	Method         string `json:"method"`
	SSHUser        string `json:"ssh_user"`
	SSHPassword    string `json:"ssh_password"`
	SSHPrivateKey  string `json:"ssh_private_key"`
	SSHPassphrase  string `json:"ssh_passphrase"`
	ProxmoxTokenID string `json:"proxmox_token_id"`
	ProxmoxSecret  string `json:"proxmox_secret"`
	DockerTLSCA    string `json:"docker_tls_ca"`
	DockerTLSCert  string `json:"docker_tls_cert"`
	DockerTLSKey   string `json:"docker_tls_key"`
}

func (c credentialsBody) toCreds() nodes.NodeCredentials {
	return nodes.NodeCredentials{
		Method:         c.Method,
		SSHUser:        c.SSHUser,
		SSHPassword:    c.SSHPassword,
		SSHPrivateKey:  c.SSHPrivateKey,
		SSHPassphrase:  c.SSHPassphrase,
		ProxmoxTokenID: c.ProxmoxTokenID,
		ProxmoxSecret:  c.ProxmoxSecret,
		DockerTLSCA:    c.DockerTLSCA,
		DockerTLSCert:  c.DockerTLSCert,
		DockerTLSKey:   c.DockerTLSKey,
	}
}

type connBody struct {
	Host               string          `json:"host"`
	SSHPort            int             `json:"ssh_port"`
	Credentials        credentialsBody `json:"credentials"`
	ProxmoxEndpoint    string          `json:"proxmox_endpoint"`
	ProxmoxTLSInsecure bool            `json:"proxmox_tls_insecure"`
	DockerEndpoint     string          `json:"docker_endpoint"`
	PinnedHostKey      string          `json:"pinned_host_key"`
	AckInsecureDocker  bool            `json:"ack_insecure_docker"`
}

func (b connBody) toConnInput() nodes.ConnInput {
	port := b.SSHPort
	if port == 0 {
		port = 22
	}
	return nodes.ConnInput{
		Host:               b.Host,
		SSHPort:            port,
		Credentials:        b.Credentials.toCreds(),
		ProxmoxEndpoint:    b.ProxmoxEndpoint,
		ProxmoxTLSInsecure: b.ProxmoxTLSInsecure,
		DockerEndpoint:     b.DockerEndpoint,
		PinnedHostKey:      b.PinnedHostKey,
	}
}

func (b connBody) insecureDockerUnacked() bool {
	return insecureDockerUnacked(b.DockerEndpoint, b.Credentials.DockerTLSCA, b.Credentials.DockerTLSCert, b.Credentials.DockerTLSKey, b.AckInsecureDocker)
}

// insecureDockerUnacked reports whether a plaintext tcp://|http:// Docker
// endpoint was supplied without any TLS material and without explicit
// acknowledgement. Plaintext 2375 grants unauthenticated root-equivalent control
// of the host, so we refuse it unless the operator opts in. Shared by the create
// and update paths.
func insecureDockerUnacked(endpoint, tlsCA, tlsCert, tlsKey string, ack bool) bool {
	// Both tcp:// and http:// connect without TLS in the Docker SDK.
	if !strings.HasPrefix(endpoint, "tcp://") && !strings.HasPrefix(endpoint, "http://") {
		return false
	}
	hasTLS := tlsCA != "" || tlsCert != "" || tlsKey != ""
	return !hasTLS && !ack
}

type createNodeBody struct {
	connBody
	Name            string `json:"name"`
	AcceptedHostKey string `json:"accepted_host_key"`
	TypeOverride    string `json:"type_override"`
}

// ListNodes returns all registered nodes (no secrets).
func (h *Handlers) ListNodes(w http.ResponseWriter, r *http.Request) {
	views, err := h.Nodes.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": views})
}

// GetNode returns one node.
func (h *Handlers) GetNode(w http.ResponseWriter, r *http.Request) {
	v, err := h.Nodes.Get(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// CreateNode probes + registers a node.
func (h *Handlers) CreateNode(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body createNodeBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if body.Name == "" || body.Host == "" {
		writeError(w, http.StatusBadRequest, "name_and_host_required")
		return
	}
	if body.insecureDockerUnacked() {
		writeError(w, http.StatusBadRequest, "insecure_docker_endpoint_requires_ack")
		return
	}

	view, err := h.Nodes.Create(r.Context(), nodes.CreateInput{
		ConnInput:       body.toConnInput(),
		Name:            body.Name,
		AcceptedHostKey: body.AcceptedHostKey,
		TypeOverride:    body.TypeOverride,
	})
	switch {
	case errors.Is(err, nodes.ErrHostKeyRequired):
		writeError(w, http.StatusBadRequest, "host_key_required")
		return
	case errors.Is(err, nodes.ErrHostKeyMismatch):
		writeError(w, http.StatusConflict, "host_key_mismatch")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}

	enrichNodeActivity(r, "node.create", view.ID)
	writeJSON(w, http.StatusCreated, view)
}

// updateNodeBody is the PUT /api/nodes/{id} payload. Every field is a pointer so
// the handler can distinguish "omitted" (leave as-is) from "supplied" (apply,
// possibly clearing). Backward compatible: a body of just {"name":"x"} still
// works as a plain rename. The credentials object carries optional Docker TLS PEM
// material (docker_tls_ca/cert/key); it is never echoed back.
type updateNodeBody struct {
	Name               *string          `json:"name"`
	DockerEndpoint     *string          `json:"docker_endpoint"`
	ProxmoxEndpoint    *string          `json:"proxmox_endpoint"`
	ProxmoxTLSInsecure *bool            `json:"proxmox_tls_insecure"`
	Credentials        *credentialsBody `json:"credentials"`
	AckInsecureDocker  bool             `json:"ack_insecure_docker"`
	// LinkedVMID is the manual Proxmox-guest link (tri-state). json.RawMessage so
	// the handler can tell "key omitted" (leave as-is) from an explicit JSON
	// null (AUTO), 0 (NONE), or a vmid (>=100). Absent => not supplied.
	LinkedVMID json.RawMessage `json:"linked_vmid"`
}

// RenameNode updates an existing node's display name and/or Docker/Proxmox
// transport config (PUT /api/nodes/{id}). It re-probes with the new config and
// invalidates the cached transport clients so the next poll uses the edit.
func (h *Handlers) RenameNode(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var body updateNodeBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	// A rename must not blank the name; a config-only edit may omit it entirely.
	if body.Name != nil && *body.Name == "" {
		writeError(w, http.StatusBadRequest, "name_required")
		return
	}

	in := nodes.UpdateConfigInput{
		Name:               body.Name,
		DockerEndpoint:     body.DockerEndpoint,
		ProxmoxEndpoint:    body.ProxmoxEndpoint,
		ProxmoxTLSInsecure: body.ProxmoxTLSInsecure,
	}
	// linked_vmid tri-state: absent => leave as-is; explicit null => AUTO (nil);
	// a number => NONE (0) or an explicit VMID (>=100). Reject negatives.
	if body.LinkedVMID != nil {
		in.LinkedVMIDSupplied = true
		if !isJSONNull(body.LinkedVMID) {
			var v int
			if err := json.Unmarshal(body.LinkedVMID, &v); err != nil || v < 0 {
				writeError(w, http.StatusBadRequest, "invalid_linked_vmid")
				return
			}
			in.LinkedVMID = &v
		}
	}
	if body.Credentials != nil {
		in.DockerTLSSupplied = true
		in.DockerTLSCA = body.Credentials.DockerTLSCA
		in.DockerTLSCert = body.Credentials.DockerTLSCert
		in.DockerTLSKey = body.Credentials.DockerTLSKey
	}

	// Guard a plaintext tcp:// Docker endpoint supplied without TLS or ack.
	if body.DockerEndpoint != nil && insecureDockerUnacked(*body.DockerEndpoint, in.DockerTLSCA, in.DockerTLSCert, in.DockerTLSKey, body.AckInsecureDocker) {
		writeError(w, http.StatusBadRequest, "insecure_docker_endpoint_requires_ack")
		return
	}

	v, err := h.Nodes.UpdateConfig(r.Context(), id, in)
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
		return
	case errors.Is(err, nodes.ErrHostKeyMismatch):
		writeError(w, http.StatusConflict, "host_key_mismatch")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	// Drop cached Docker/Proxmox clients so the next poll rebuilds from the edit.
	if h.Conn != nil {
		h.Conn.Invalidate(id)
	}
	enrichNodeActivity(r, "node.update", id)
	writeJSON(w, http.StatusOK, v)
}

// DeleteNode removes a node (admin + step-up 2FA — it discards the host's
// stored credentials and all derived inventory).
func (h *Handlers) DeleteNode(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	err := h.Nodes.Delete(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	enrichNodeActivity(r, "node.delete", id)
	w.WriteHeader(http.StatusNoContent)
}

// ReprobeNode re-runs detection on an existing node (admin — it makes the
// backend connect to the stored host).
func (h *Handlers) ReprobeNode(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	v, err := h.Nodes.Reprobe(r.Context(), id)
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
		return
	case errors.Is(err, nodes.ErrHostKeyMismatch):
		writeError(w, http.StatusConflict, "host_key_mismatch")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "probe_failed")
		return
	}
	enrichNodeActivity(r, "node.probe", id)
	writeJSON(w, http.StatusOK, v)
}

// ProbePreview probes a host WITHOUT persisting. Admin-only and rate-limited:
// it makes the backend connect to an operator-supplied host:port (an inherent
// SSRF surface), so it is gated to the admin user and throttled as a scanning
// oracle defense.
func (h *Handlers) ProbePreview(w http.ResponseWriter, r *http.Request) {
	if u, ok := middleware.UserFromContext(r.Context()); !ok || u.Role != "admin" {
		writeError(w, http.StatusForbidden, "admin_only")
		return
	}
	if h.PreviewLimiter != nil && !h.PreviewLimiter.Allow() {
		writeError(w, http.StatusTooManyRequests, "rate_limited")
		return
	}
	var body connBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if body.insecureDockerUnacked() {
		writeError(w, http.StatusBadRequest, "insecure_docker_endpoint_requires_ack")
		return
	}
	result := h.Nodes.ProbePreview(r.Context(), body.toConnInput())
	writeJSON(w, http.StatusOK, result)
}

// isJSONNull reports whether a raw JSON value is the literal null (ignoring
// surrounding whitespace).
func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

func enrichNodeActivity(r *http.Request, action, nodeID string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr("node")
		id := nodeID
		e.TargetID = &id
	}
}
