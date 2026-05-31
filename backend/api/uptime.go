package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/uptime"
)

// monitorView is the JSON representation of a monitor returned by the API.
type monitorView struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Target          string  `json:"target"`
	IntervalSeconds int     `json:"interval_seconds"`
	TimeoutMs       int     `json:"timeout_ms"`
	Expected        string  `json:"expected"`
	Enabled         bool    `json:"enabled"`
	NodeID          *string `json:"node_id,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type monitorWithStats struct {
	monitorView
	uptime.Stats
}

func toMonitorView(m db.UptimeMonitor) monitorView {
	return monitorView{
		ID:              m.ID,
		Name:            m.Name,
		Type:            m.Type,
		Target:          m.Target,
		IntervalSeconds: m.IntervalSeconds,
		TimeoutMs:       m.TimeoutMs,
		Expected:        m.Expected,
		Enabled:         m.Enabled,
		NodeID:          m.NodeID,
		CreatedAt:       m.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       m.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

type resultView struct {
	ID             string `json:"id"`
	MonitorID      string `json:"monitor_id"`
	CheckedAt      string `json:"checked_at"`
	Status         string `json:"status"`
	ResponseTimeMs int    `json:"response_time_ms"`
	Error          string `json:"error,omitempty"`
}

func toResultView(r db.UptimeResult) resultView {
	return resultView{
		ID:             r.ID,
		MonitorID:      r.MonitorID,
		CheckedAt:      r.CheckedAt.UTC().Format(time.RFC3339),
		Status:         r.Status,
		ResponseTimeMs: r.ResponseTimeMs,
		Error:          r.Error,
	}
}

type monitorBody struct {
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Target          string  `json:"target"`
	IntervalSeconds int     `json:"interval_seconds"`
	TimeoutMs       int     `json:"timeout_ms"`
	Expected        string  `json:"expected"`
	Enabled         bool    `json:"enabled"`
	NodeID          *string `json:"node_id,omitempty"`
}

func (b monitorBody) validate() string {
	if b.Name == "" {
		return "name_required"
	}
	switch b.Type {
	case "http", "tcp", "icmp":
	default:
		return "type_must_be_http_tcp_or_icmp"
	}
	if b.Target == "" {
		return "target_required"
	}
	if b.IntervalSeconds <= 0 {
		return "interval_seconds_must_be_positive"
	}
	if b.TimeoutMs <= 0 {
		return "timeout_ms_must_be_positive"
	}
	return ""
}

// ListUptimeMonitors returns all monitors with their current stats. Read-only.
func (h *Handlers) ListUptimeMonitors(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.Store.ListUptimeMonitors(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	now := time.Now()
	from30d := now.Add(-30 * 24 * time.Hour)
	out := make([]monitorWithStats, len(monitors))
	for i, m := range monitors {
		results, _ := h.Store.ListUptimeResults(r.Context(), m.ID, from30d, now)
		out[i] = monitorWithStats{
			monitorView: toMonitorView(m),
			Stats:       uptime.ComputeStats(m.ID, results),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"monitors": out})
}

// GetUptimeMonitor returns one monitor with its stats. Read-only.
func (h *Handlers) GetUptimeMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	m, err := h.Store.GetUptimeMonitor(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	now := time.Now()
	results, _ := h.Store.ListUptimeResults(r.Context(), id, now.Add(-30*24*time.Hour), now)
	mws := monitorWithStats{
		monitorView: toMonitorView(m),
		Stats:       uptime.ComputeStats(id, results),
	}
	writeJSON(w, http.StatusOK, mws)
}

// CreateUptimeMonitor adds a new monitor. Audited.
func (h *Handlers) CreateUptimeMonitor(w http.ResponseWriter, r *http.Request) {
	var body monitorBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if code := body.validate(); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	if body.IntervalSeconds == 0 {
		body.IntervalSeconds = 60
	}
	if body.TimeoutMs == 0 {
		body.TimeoutMs = 5000
	}
	// Capability gate: node-scoped checks require a registered node.
	if body.NodeID != nil {
		if _, err := h.Store.GetNode(r.Context(), *body.NodeID); errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "node_not_found")
			return
		}
	}
	m := db.UptimeMonitor{
		ID:              uuid.NewString(),
		Name:            body.Name,
		Type:            body.Type,
		Target:          body.Target,
		IntervalSeconds: body.IntervalSeconds,
		TimeoutMs:       body.TimeoutMs,
		Expected:        body.Expected,
		Enabled:         body.Enabled,
		NodeID:          body.NodeID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := h.Store.CreateUptimeMonitor(r.Context(), m); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed")
		return
	}
	auditUptime(r, activity.ActionUptimeCreate, m.ID, m.Name)
	writeJSON(w, http.StatusCreated, toMonitorView(m))
}

// UpdateUptimeMonitor edits a monitor. Audited.
func (h *Handlers) UpdateUptimeMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body monitorBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if code := body.validate(); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	if body.NodeID != nil {
		if _, err := h.Store.GetNode(r.Context(), *body.NodeID); errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "node_not_found")
			return
		}
	}
	m := db.UptimeMonitor{
		ID:              id,
		Name:            body.Name,
		Type:            body.Type,
		Target:          body.Target,
		IntervalSeconds: body.IntervalSeconds,
		TimeoutMs:       body.TimeoutMs,
		Expected:        body.Expected,
		Enabled:         body.Enabled,
		NodeID:          body.NodeID,
	}
	if err := h.Store.UpdateUptimeMonitor(r.Context(), m); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed")
		return
	}
	auditUptime(r, activity.ActionUptimeUpdate, id, body.Name)
	w.WriteHeader(http.StatusNoContent)
}

// DeleteUptimeMonitor removes a monitor and its history (CASCADE). Audited.
func (h *Handlers) DeleteUptimeMonitor(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteUptimeMonitor(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	auditUptime(r, activity.ActionUptimeDelete, id, "")
	w.WriteHeader(http.StatusNoContent)
}

// UptimeMonitorHistory returns the time-series results for a monitor.
func (h *Handlers) UptimeMonitorHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := h.Store.GetUptimeMonitor(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	to := time.Now()
	from := to.Add(-uptimeHistoryRange(r.URL.Query().Get("range")))
	results, err := h.Store.ListUptimeResults(r.Context(), id, from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	views := make([]resultView, len(results))
	for i, res := range results {
		views[i] = toResultView(res)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": views})
}

func uptimeHistoryRange(v string) time.Duration {
	switch v {
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func auditUptime(r *http.Request, action, id, name string) {
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetUptime)
		e.TargetID = &id
		if name != "" {
			e.Detail = map[string]string{"name": name}
		}
	}
}
