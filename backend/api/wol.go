package api

import (
	"errors"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/wol"
)

type wolConfigView struct {
	MAC       string `json:"mac"`
	Broadcast string `json:"broadcast"`
	Port      int    `json:"port"`
}

// GetWOL returns a node's Wake-on-LAN config (404 if none set). Read-only.
func (h *Handlers) GetWOL(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	c, err := h.Store.GetWOLConfig(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, wolConfigView{MAC: c.MAC, Broadcast: c.Broadcast, Port: c.Port})
}

type setWOLBody struct {
	MAC       string `json:"mac"`
	Broadcast string `json:"broadcast"`
	Port      int    `json:"port"`
}

// SetWOL stores a node's Wake-on-LAN config. Admin-gated + audited.
func (h *Handlers) SetWOL(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	if _, err := h.Store.GetNode(r.Context(), nodeID); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var body setWOLBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if _, err := net.ParseMAC(body.MAC); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_mac")
		return
	}
	if body.Port < 0 || body.Port > 65535 {
		writeError(w, http.StatusBadRequest, "invalid_port")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionNodeWOLSet
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
		e.Detail = map[string]string{"mac": body.MAC}
	}
	if err := h.Store.UpsertWOLConfig(r.Context(), db.WOLConfig{
		NodeID: nodeID, MAC: body.MAC, Broadcast: body.Broadcast, Port: body.Port,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// WakeNode sends a Wake-on-LAN magic packet using the node's stored config.
// Admin-gated + audited.
func (h *Handlers) WakeNode(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	c, err := h.Store.GetWOLConfig(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusConflict, "wol_not_configured")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionNodeWake
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
		e.Detail = map[string]string{"mac": c.MAC, "broadcast": c.Broadcast}
	}
	if err := wol.Send(c.MAC, c.Broadcast, c.Port); err != nil {
		writeError(w, http.StatusBadGateway, "wake_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
