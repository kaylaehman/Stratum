package api

import (
	"context"
	"net/http"
	"time"

	cerrdefs "github.com/containerd/errdefs"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/docker"
)

// lifecycleTimeout bounds a single lifecycle call (stop can take the daemon's
// graceful-shutdown timeout).
const lifecycleTimeout = 30 * time.Second

// StartContainer starts a container. Operator-gated + audited.
func (h *Handlers) StartContainer(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, activity.ActionContainerStart, func(c *docker.Client, ctx context.Context, id string) error {
		return c.StartContainer(ctx, id)
	})
}

// StopContainer stops a container. Operator-gated + audited.
func (h *Handlers) StopContainer(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, activity.ActionContainerStop, func(c *docker.Client, ctx context.Context, id string) error {
		return c.StopContainer(ctx, id)
	})
}

// RestartContainer restarts a container. Operator-gated + audited.
func (h *Handlers) RestartContainer(w http.ResponseWriter, r *http.Request) {
	h.lifecycle(w, r, activity.ActionContainerRestart, func(c *docker.Client, ctx context.Context, id string) error {
		return c.RestartContainer(ctx, id)
	})
}

// lifecycle resolves the container (which also yields its node's docker client),
// runs the daemon action, and records the audit entry. The inventory poller
// reconciles the new status on its next cycle.
func (h *Handlers) lifecycle(w http.ResponseWriter, r *http.Request, action string, fn func(*docker.Client, context.Context, string) error) {
	// Non-destructive lifecycle (start/stop/restart) is allowed for operators;
	// destructive ops (remove, recreate) remain admin-only.
	if !h.requireOperator(w, r) {
		return
	}
	ctr, clients, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = action
		e.TargetType = ptr(activity.TargetContainer)
		e.TargetID = &ctr.ID
		e.Detail = map[string]string{"node_id": ctr.NodeID, "container": ctr.Name}
	}

	ctx, cancel := context.WithTimeout(r.Context(), lifecycleTimeout)
	defer cancel()
	if err := fn(clients.Docker, ctx, ctr.DockerID); err != nil {
		// A conflict (e.g. starting an already-running container) is a state
		// problem, not a gateway failure — surface it as 409, not 502.
		if cerrdefs.IsConflict(err) {
			writeError(w, http.StatusConflict, "container_already_in_state")
			return
		}
		writeError(w, http.StatusBadGateway, "lifecycle_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
