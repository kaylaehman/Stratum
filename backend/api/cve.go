package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/cve"
	"github.com/kaylaehman/stratum/backend/db"
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
