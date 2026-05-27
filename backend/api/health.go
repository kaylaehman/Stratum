package api

import (
	"context"
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

// ContainerHealth returns a container's healthcheck config + recent probe
// results (read-only). The edit half (modifying the healthcheck) needs the
// recreate machinery and is a separate, later concern.
func (h *Handlers) ContainerHealth(w http.ResponseWriter, r *http.Request) {
	ctr, clients, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	report, err := clients.Docker.ContainerHealth(ctx, ctr.DockerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "inspect_failed")
		return
	}
	writeJSON(w, http.StatusOK, report)
}
