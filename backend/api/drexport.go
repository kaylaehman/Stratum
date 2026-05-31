package api

import (
	"fmt"
	"net/http"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/drexport"
)

// DRExport handles GET /api/dr-export?format=json|yaml|md.
// Requires admin role. Audited as dr.export.
func (h *Handlers) DRExport(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	m, err := h.DRExportSvc.Build(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionDRExport
		e.TargetType = ptr(activity.TargetDR)
		e.Detail = map[string]string{"format": format}
	}

	ts := m.GeneratedAt.Format("20060102-150405")

	switch format {
	case "yaml":
		out, rerr := drexport.RenderYAML(m)
		if rerr != nil {
			writeError(w, http.StatusInternalServerError, "render_error")
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="stratum-dr-%s.yaml"`, ts))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(out)

	case "md":
		out := drexport.RenderMarkdown(m)
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="stratum-dr-%s.md"`, ts))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(out)

	default: // "json"
		out, rerr := drexport.RenderJSON(m)
		if rerr != nil {
			writeError(w, http.StatusInternalServerError, "render_error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition",
			fmt.Sprintf(`attachment; filename="stratum-dr-%s.json"`, ts))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(out)
	}
}
