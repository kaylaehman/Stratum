package api

import (
	"net/http"
	"time"
)

// Health is an unauthenticated liveness endpoint. It confirms the database is
// reachable by running a trivial query.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	dbOK := true
	if _, err := h.Store.CountUsers(r.Context()); err != nil {
		dbOK = false
	}
	status := "ok"
	code := http.StatusOK
	if !dbOK {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"status":         status,
		"db":             dbOK,
		"uptime_seconds": int(time.Since(h.StartedAt).Seconds()),
	})
}
