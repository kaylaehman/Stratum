package api

import (
	"encoding/json"
	"net/http"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/middleware"
	"github.com/kaylaehman/stratum/backend/push"
)

// VAPIDKey returns the server's VAPID public key for the frontend.
// GET /api/push/vapid-key — authenticated but no role requirement.
func (h *Handlers) VAPIDKey(w http.ResponseWriter, r *http.Request) {
	if h.Push == nil {
		http.Error(w, "push notifications not enabled", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": h.Push.PublicKey()})
}

// PushSubscribe stores a push subscription for the authenticated user.
// POST /api/push/subscribe
func (h *Handlers) PushSubscribe(w http.ResponseWriter, r *http.Request) {
	if h.Push == nil {
		http.Error(w, "push notifications not enabled", http.StatusServiceUnavailable)
		return
	}

	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var sub push.Subscription
	if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.Push.Subscribe(r.Context(), u.ID, sub); err != nil {
		h.Logger.Warn("push subscribe failed", "user", u.ID, "err", err)
		http.Error(w, "failed to store subscription", http.StatusBadRequest)
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionPushSubscribe
		e.TargetType = ptr(activity.TargetUser)
		e.Detail = map[string]string{"user_id": u.ID}
	}
	w.WriteHeader(http.StatusNoContent)
}

// PushUnsubscribe removes a push subscription by endpoint.
// POST /api/push/unsubscribe
func (h *Handlers) PushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	if h.Push == nil {
		http.Error(w, "push notifications not enabled", http.StatusServiceUnavailable)
		return
	}

	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Endpoint == "" {
		http.Error(w, "endpoint required", http.StatusBadRequest)
		return
	}

	if err := h.Push.Unsubscribe(r.Context(), body.Endpoint); err != nil {
		h.Logger.Warn("push unsubscribe failed", "err", err)
		http.Error(w, "failed to remove subscription", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PushTest sends a test push to all subscriptions. Admin-only.
// POST /api/push/test
func (h *Handlers) PushTest(w http.ResponseWriter, r *http.Request) {
	if h.Push == nil {
		http.Error(w, "push notifications not enabled", http.StatusServiceUnavailable)
		return
	}
	if !h.requireAdmin(w, r) {
		return
	}

	payload := push.Payload{
		Title: "Stratum test push",
		Body:  "Web Push notifications are working.",
		Tag:   "stratum-test",
		URL:   "/notifications",
	}
	if err := h.Push.SendToAll(r.Context(), payload); err != nil {
		h.Logger.Warn("push test send failed", "err", err)
		http.Error(w, "send failed", http.StatusInternalServerError)
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionPushTest
		e.TargetType = ptr(activity.TargetUser)
		e.Detail = map[string]string{"result": "sent"}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
