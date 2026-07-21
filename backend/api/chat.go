package api

import (
	"net/http"

	"github.com/KAE-Labs/stratum/backend/activity"
)

// ChatConfigGet returns the non-secret chat-bot config (admin).
func (h *Handlers) ChatConfigGet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, h.Chat.Config(r.Context()))
}

type chatConfigRequest struct {
	AllowedChats []int64 `json:"allowed_chats"`
	Token        *string `json:"token"` // nil keep, "" clear, value set
}

// ChatConfigSet stores the chat-bot config (admin). Token sealed; never echoed.
// Audited.
func (h *Handlers) ChatConfigSet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req chatConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if req.AllowedChats == nil {
		req.AllowedChats = []int64{}
	}
	if err := h.Chat.SetConfig(r.Context(), req.AllowedChats, req.Token); err != nil {
		writeError(w, http.StatusInternalServerError, "save_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionChatConfig
		e.TargetType = ptr(activity.TargetUser)
	}
	writeJSON(w, http.StatusOK, h.Chat.Config(r.Context()))
}
