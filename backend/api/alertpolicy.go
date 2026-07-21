package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/alertpolicy"
	appdb "github.com/KAE-Labs/stratum/backend/db"
)

// ListAlertPolicies returns all alert routing policies. Admin-gated.
func (h *Handlers) ListAlertPolicies(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.AlertPolicy == nil {
		writeError(w, http.StatusServiceUnavailable, "alert_policy_not_configured")
		return
	}
	policies, err := h.AlertPolicy.ListPolicies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if policies == nil {
		policies = []appdb.AlertPolicy{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"alert_policies": policies})
}

// alertPolicyBody is the request body for create/update.
type alertPolicyBody struct {
	Name           string                  `json:"name"`
	Enabled        bool                    `json:"enabled"`
	MinSeverity    string                  `json:"min_severity"`
	Channels       []string                `json:"channels"`
	Match          *appdb.AlertPolicyMatch `json:"match,omitempty"`
	QuietHours     *appdb.AlertQuietHours  `json:"quiet_hours,omitempty"`
	DedupWindowSec int                     `json:"dedup_window_sec"`
	Escalate       *appdb.AlertEscalate    `json:"escalate,omitempty"`
}

func (b alertPolicyBody) validate() error {
	if b.Name == "" {
		return errors.New("name is required")
	}
	sev := b.MinSeverity
	if sev == "" {
		sev = "info"
	}
	valid := map[string]bool{"info": true, "warning": true, "critical": true}
	if !valid[sev] {
		return errors.New("min_severity must be info, warning, or critical")
	}
	if b.DedupWindowSec < 0 {
		return errors.New("dedup_window_sec must be >= 0")
	}
	return nil
}

func alertPolicyFromBody(id string, b alertPolicyBody, now time.Time) appdb.AlertPolicy {
	sev := b.MinSeverity
	if sev == "" {
		sev = "info"
	}
	channels := b.Channels
	if channels == nil {
		channels = []string{}
	}
	match := appdb.AlertPolicyMatch{}
	if b.Match != nil {
		match = *b.Match
	}
	return appdb.AlertPolicy{
		ID:             id,
		Name:           b.Name,
		Enabled:        b.Enabled,
		MinSeverity:    sev,
		Channels:       channels,
		Match:          match,
		QuietHours:     b.QuietHours,
		DedupWindowSec: b.DedupWindowSec,
		Escalate:       b.Escalate,
		UpdatedAt:      now,
	}
}

// CreateAlertPolicy creates a new routing policy. Admin-gated + audited.
func (h *Handlers) CreateAlertPolicy(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.AlertPolicy == nil {
		writeError(w, http.StatusServiceUnavailable, "alert_policy_not_configured")
		return
	}
	var body alertPolicyBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if err := body.validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error")
		return
	}
	now := time.Now()
	p := alertPolicyFromBody(uuid.NewString(), body, now)
	p.CreatedAt = now

	if err := h.alertPolicyStore().CreateAlertPolicy(r.Context(), p); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	auditAlertPolicy(r, p.ID, p.Name)
	writeJSON(w, http.StatusCreated, p)
}

// UpdateAlertPolicy replaces a policy. Admin-gated + audited.
func (h *Handlers) UpdateAlertPolicy(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.AlertPolicy == nil {
		writeError(w, http.StatusServiceUnavailable, "alert_policy_not_configured")
		return
	}
	id := chi.URLParam(r, "id")
	var body alertPolicyBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if err := body.validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error")
		return
	}
	p := alertPolicyFromBody(id, body, time.Now())
	if err := h.alertPolicyStore().UpdateAlertPolicy(r.Context(), p); errors.Is(err, appdb.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditAlertPolicy(r, id, body.Name)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteAlertPolicy removes a policy. Admin-gated + audited.
func (h *Handlers) DeleteAlertPolicy(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.AlertPolicy == nil {
		writeError(w, http.StatusServiceUnavailable, "alert_policy_not_configured")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.alertPolicyStore().DeleteAlertPolicy(r.Context(), id); errors.Is(err, appdb.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditAlertPolicy(r, id, "")
	w.WriteHeader(http.StatusNoContent)
}

// ListAlertDeliveries returns recent delivery records. Admin-gated. Read-only.
func (h *Handlers) ListAlertDeliveries(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if h.AlertPolicy == nil {
		writeError(w, http.StatusServiceUnavailable, "alert_policy_not_configured")
		return
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	deliveries, err := h.AlertPolicy.ListDeliveries(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if deliveries == nil {
		deliveries = []appdb.AlertDelivery{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"alert_deliveries": deliveries})
}

// alertPolicyStore returns the underlying store from the AlertPolicy service,
// or nil when the service is not wired.
func (h *Handlers) alertPolicyStore() alertpolicy.Store {
	if h.AlertPolicy == nil {
		return nil
	}
	return h.AlertPolicy.StoreRef()
}

// auditAlertPolicy emits an audit entry for an alert-policy mutation.
// The orchestrator must add ActionAlertPolicyConfig = "alertpolicy.config"
// and TargetAlertPolicy = "alert_policy" to backend/activity/actions.go.
func auditAlertPolicy(r *http.Request, id, name string) {
	e := activity.FromContext(r.Context())
	if e == nil {
		return
	}
	e.Action = "alertpolicy.config"
	t := "alert_policy"
	e.TargetType = &t
	e.TargetID = &id
	if name != "" {
		e.Detail = map[string]string{"name": name}
	}
}
