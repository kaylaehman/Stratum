package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/webhooks"
)

type webhookView struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Provider string   `json:"provider"`
	Triggers []string `json:"triggers"`
	Enabled  bool     `json:"enabled"`
}

func toWebhookView(c db.WebhookConfig) webhookView {
	t := c.Triggers
	if t == nil {
		t = []string{}
	}
	return webhookView{ID: c.ID, Name: c.Name, URL: c.URL, Provider: c.Provider, Triggers: t, Enabled: c.Enabled}
}

// ListWebhooks returns all notification webhooks. Admin-gated.
func (h *Handlers) ListWebhooks(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	rows, err := h.Store.ListWebhooks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]webhookView, len(rows))
	for i, c := range rows {
		out[i] = toWebhookView(c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"webhooks": out, "available_triggers": webhooks.AllTriggers})
}

type webhookBody struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Provider string   `json:"provider"`
	Triggers []string `json:"triggers"`
	Enabled  bool     `json:"enabled"`
}

func (b webhookBody) valid() bool {
	return b.Name != "" && b.URL != "" && webhooks.ValidProvider(b.Provider)
}

// CreateWebhook adds a notification webhook. Admin-gated + audited.
func (h *Handlers) CreateWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body webhookBody
	if err := decodeJSON(r, &body); err != nil || !body.valid() {
		writeError(w, http.StatusBadRequest, "name_url_provider_required")
		return
	}
	c := db.WebhookConfig{
		ID: uuid.NewString(), Name: body.Name, URL: body.URL, Provider: body.Provider,
		Triggers: body.Triggers, Enabled: body.Enabled,
	}
	if err := h.Store.CreateWebhook(r.Context(), c); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	auditWebhook(r, activity.ActionWebhookCreate, c.ID, c.Name)
	writeJSON(w, http.StatusCreated, toWebhookView(c))
}

// UpdateWebhook edits a webhook. Admin-gated + audited.
func (h *Handlers) UpdateWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	var body webhookBody
	if err := decodeJSON(r, &body); err != nil || !body.valid() {
		writeError(w, http.StatusBadRequest, "name_url_provider_required")
		return
	}
	c := db.WebhookConfig{
		ID: id, Name: body.Name, URL: body.URL, Provider: body.Provider,
		Triggers: body.Triggers, Enabled: body.Enabled,
	}
	if err := h.Store.UpdateWebhook(r.Context(), c); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditWebhook(r, activity.ActionWebhookUpdate, id, body.Name)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteWebhook removes a webhook. Admin-gated + audited.
func (h *Handlers) DeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteWebhook(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditWebhook(r, activity.ActionWebhookDelete, id, "")
	w.WriteHeader(http.StatusNoContent)
}

// TestWebhook sends a test message to a webhook (bypasses the rate limit).
func (h *Handlers) TestWebhook(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	c, err := h.Store.GetWebhook(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if err := h.Webhooks.Test(r.Context(), c, webhooks.Message{
		Title: "Stratum test notification",
		Text:  "If you can see this, your " + c.Provider + " webhook is configured correctly.",
	}); err != nil {
		writeError(w, http.StatusBadGateway, "delivery_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func auditWebhook(r *http.Request, action, id, name string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetWebhook)
		e.TargetID = &id
		if name != "" {
			e.Detail = map[string]string{"name": name}
		}
	}
}
