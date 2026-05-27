package api

import (
	"context"
	"net/http"
	"time"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/docker"
)

// lifecycleTimeout bounds a single lifecycle call (stop can take the daemon's
// graceful-shutdown timeout).
const lifecycleTimeout = 30 * time.Second

// StartContainer starts a container. Admin-gated + audited.
func (h *Handlers) StartContainer(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, activity.ActionContainerStart, func(c *docker.Client, ctx context.Context, id string) error {
		return c.StartContainer(ctx, id)
	})
}

// StopContainer stops a container. Admin-gated + audited.
func (h *Handlers) StopContainer(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, activity.ActionContainerStop, func(c *docker.Client, ctx context.Context, id string) error {
		return c.StopContainer(ctx, id)
	})
}

// RestartContainer restarts a container. Admin-gated + audited.
func (h *Handlers) RestartContainer(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, activity.ActionContainerRestart, func(c *docker.Client, ctx context.Context, id string) error {
		return c.RestartContainer(ctx, id)
	})
}

// lifecycle resolves the container, runs the daemon action against its node, and
// records the audit entry. The inventory poller reconciles the new status on its
// next cycle.
func (h *Handlers) lifecycle(w http.ResponseWriter, r *http.Request, action string, fn func(*docker.Client, context.Context, string) error) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetContainer)
		e.TargetID = &ctr.ID
		e.Detail = map[string]string{"node_id": ctr.NodeID, "container": ctr.Name}
	}

	client, err := h.Docker(r.Context(), ctr.NodeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "node_unreachable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), lifecycleTimeout)
	defer cancel()
	if err := fn(client, ctx, ctr.DockerID); err != nil {
		writeError(w, http.StatusBadGateway, "lifecycle_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
