package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/scheduler"
)

// NodeSchedule returns a node's cron jobs + systemd timers (read over SSH).
// Admin-gated (reveals what runs on the host).
func (h *Handlers) NodeSchedule(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	sched, err := h.Scheduler.Read(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "ssh_failed")
		return
	}
	writeJSON(w, http.StatusOK, sched)
}

type setCronBody struct {
	User    string `json:"user"`
	Content string `json:"content"`
}

// SetCron installs a user's full crontab content. Admin-gated + audited.
func (h *Handlers) SetCron(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	var body setCronBody
	if err := decodeJSON(r, &body); err != nil || !scheduler.ValidUser(body.User) {
		writeError(w, http.StatusBadRequest, "valid_user_required")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionCronSet
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
		e.Detail = map[string]string{"user": body.User}
	}
	if err := h.Scheduler.SetCrontab(r.Context(), nodeID, body.User, body.Content); err != nil {
		writeError(w, http.StatusBadGateway, "cron_write_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
