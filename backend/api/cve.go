package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/cve"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
)

type cveScanView struct {
	ImageDigest string `json:"image_digest"`
	Image       string `json:"image"`
	ScannedAt   string `json:"scanned_at"`
	Critical    int    `json:"critical"`
	High        int    `json:"high"`
	Medium      int    `json:"medium"`
	Low         int    `json:"low"`
	Unknown     int    `json:"unknown"`
}

// CVEScans lists cached CVE scan summaries + scanner availability. Admin-gated.
func (h *Handlers) CVEScans(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	rows, err := h.CVE.ListScans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]cveScanView, len(rows))
	for i, s := range rows {
		out[i] = cveScanView{
			ImageDigest: s.ImageDigest, Image: s.Image, ScannedAt: s.ScannedAt.UTC().Format(time.RFC3339),
			Critical: s.Critical, High: s.High, Medium: s.Medium, Low: s.Low, Unknown: s.Unknown,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"available": h.CVE.Available(), "scans": out})
}

// CVEStatus reports scanner presence + vulnerability-DB freshness so the UI can
// show "Vulnerability DB N days old" / "ready" instead of "not available" when
// Trivy is bundled. Admin-gated to match the other CVE endpoints.
func (h *Handlers) CVEStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	st := h.CVE.Status(r.Context())
	out := map[string]any{
		"available": st.Available,
		"path":      st.Path,
		"version":   st.Version,
	}
	if st.DBUpdatedAt != nil {
		out["db_updated_at"] = st.DBUpdatedAt.UTC().Format(time.RFC3339)
	}
	if st.DBAgeDays != nil {
		out["db_age_days"] = *st.DBAgeDays
	}
	writeJSON(w, http.StatusOK, out)
}

// CVEDetail returns the vulnerability list for a scanned image digest.
func (h *Handlers) CVEDetail(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	digest := chi.URLParam(r, "digest")
	vulns, err := h.CVE.Vulns(r.Context(), digest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]map[string]any, len(vulns))
	for i, v := range vulns {
		out[i] = map[string]any{
			"cve_id": v.CVEID, "severity": v.Severity, "package": v.Package,
			"installed_version": v.InstalledVersion, "fixed_version": v.FixedVersion, "title": v.Title,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"vulns": out})
}

// CVEScanContainer scans a container's image on demand. Admin-gated + audited.
func (h *Handlers) CVEScanContainer(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctr, err := h.Store.GetContainer(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionCVEScan
		e.TargetType = ptr(activity.TargetContainer)
		e.TargetID = &ctr.ID
		e.Detail = map[string]string{"image": ctr.Image}
	}
	if err := h.CVE.ScanContainer(r.Context(), ctr); errors.Is(err, cve.ErrUnavailable) {
		writeError(w, http.StatusConflict, "scanner_unavailable")
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "scan_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CVEBulkScan scans a list of containers in one request. Body:
// {"container_ids":["id1","id2",...]}. Admin-gated + audited.
// Returns per-container results even on partial failure (HTTP 200).
func (h *Handlers) CVEBulkScan(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		ContainerIDs []string `json:"container_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.ContainerIDs) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if !h.CVE.Available() {
		writeError(w, http.StatusConflict, "scanner_unavailable")
		return
	}

	containers := make([]db.Container, 0, len(req.ContainerIDs))
	for _, cid := range req.ContainerIDs {
		c, err := h.Store.GetContainer(r.Context(), cid)
		if err != nil {
			continue // skip unknown ids
		}
		containers = append(containers, c)
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionCVEBulkScan
		e.Detail = map[string]any{"count": len(containers)}
	}

	results := h.CVE.ScanBulk(r.Context(), containers)
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// cveScheduleView is the JSON-serialisable view of a CveSchedule.
type cveScheduleView struct {
	ID              string  `json:"id"`
	TargetType      string  `json:"target_type"`
	TargetID        string  `json:"target_id"`
	Label           string  `json:"label"`
	IntervalSeconds int     `json:"interval_seconds"`
	Enabled         bool    `json:"enabled"`
	CreatedBy       string  `json:"created_by"`
	CreatedAt       string  `json:"created_at"`
	LastRunAt       *string `json:"last_run_at,omitempty"`
}

func toCveScheduleView(s db.CveSchedule) cveScheduleView {
	v := cveScheduleView{
		ID:              s.ID,
		TargetType:      s.TargetType,
		TargetID:        s.TargetID,
		Label:           s.Label,
		IntervalSeconds: s.IntervalSeconds,
		Enabled:         s.Enabled,
		CreatedBy:       s.CreatedBy,
		CreatedAt:       s.CreatedAt.UTC().Format(time.RFC3339),
	}
	if s.LastRunAt != nil {
		t := s.LastRunAt.UTC().Format(time.RFC3339)
		v.LastRunAt = &t
	}
	return v
}

// CVEListSchedules returns all CVE scan schedules. Admin-gated.
func (h *Handlers) CVEListSchedules(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	schedules, err := h.CVE.ListSchedules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]cveScheduleView, len(schedules))
	for i, s := range schedules {
		out[i] = toCveScheduleView(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": out})
}

// CVECreateSchedule creates a new recurring CVE scan schedule. Admin-gated + audited.
func (h *Handlers) CVECreateSchedule(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		TargetType      string `json:"target_type"`
		TargetID        string `json:"target_id"`
		Label           string `json:"label"`
		IntervalSeconds int    `json:"interval_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if req.TargetType != "node" && req.TargetType != "container" {
		writeError(w, http.StatusBadRequest, "invalid_target_type")
		return
	}
	if req.TargetID == "" {
		writeError(w, http.StatusBadRequest, "missing_target_id")
		return
	}
	if req.IntervalSeconds < 3600 {
		req.IntervalSeconds = 3600 // minimum 1 hour
	}

	userID := ""
	if u, ok := middleware.UserFromContext(r.Context()); ok {
		userID = u.ID
	}
	sched := db.CveSchedule{
		ID:              uuid.NewString(),
		TargetType:      req.TargetType,
		TargetID:        req.TargetID,
		Label:           req.Label,
		IntervalSeconds: req.IntervalSeconds,
		Enabled:         true,
		CreatedBy:       userID,
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionCVEScheduleCreate
		e.Detail = map[string]any{"target_type": sched.TargetType, "target_id": sched.TargetID}
	}
	if err := h.CVE.CreateSchedule(r.Context(), sched); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusCreated, toCveScheduleView(sched))
}

// CVEToggleSchedule enables or disables a CVE schedule.
// Body: {"enabled": true/false}. Admin-gated (no audit — low-risk toggle).
func (h *Handlers) CVEToggleSchedule(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	id := chi.URLParam(r, "id")
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionCVEScheduleToggle
		e.TargetID = &id
		e.Detail = map[string]any{"enabled": req.Enabled}
	}
	if err := h.CVE.UpdateScheduleEnabled(r.Context(), id, req.Enabled); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CVEDeleteSchedule removes a CVE scan schedule. Admin-gated + audited.
func (h *Handlers) CVEDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionCVEScheduleDelete
		e.TargetID = &id
	}
	if err := h.CVE.DeleteSchedule(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
