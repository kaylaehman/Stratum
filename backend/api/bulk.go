package api

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	cerrdefs "github.com/containerd/errdefs"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/bulk"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/middleware"
)

const (
	bulkConcurrency   = 8
	bulkOpTimeout     = 30 * time.Second
	bulkMaxContainers = 500 // cap per request to bound DB lookups + goroutines
)

type bulkRequest struct {
	Action       string   `json:"action"`
	ContainerIDs []string `json:"container_ids"`
	DryRun       bool     `json:"dry_run"`
}

type bulkResultItem struct {
	bulk.Item
	Result string `json:"result"` // planned | ok | error | skipped | not_found
	Error  string `json:"error,omitempty"`
}

// dockerActionFor returns the per-container daemon call + audit action for a bulk
// action.
func dockerActionFor(action string) (string, func(*docker.Client, context.Context, string) error) {
	switch action {
	case bulk.ActionStart:
		return activity.ActionContainerStart, func(c *docker.Client, ctx context.Context, id string) error { return c.StartContainer(ctx, id) }
	case bulk.ActionStop:
		return activity.ActionContainerStop, func(c *docker.Client, ctx context.Context, id string) error { return c.StopContainer(ctx, id) }
	case bulk.ActionRestart:
		return activity.ActionContainerRestart, func(c *docker.Client, ctx context.Context, id string) error { return c.RestartContainer(ctx, id) }
	case bulk.ActionRemove:
		return activity.ActionContainerRemove, func(c *docker.Client, ctx context.Context, id string) error { return c.RemoveContainer(ctx, id, false) }
	default:
		return "", nil
	}
}

// BulkContainers performs a bulk lifecycle action across many containers in
// parallel. Non-destructive actions (start/stop/restart) are operator-gated;
// the destructive remove is admin-only. dry_run returns the plan without
// executing. Each executed container is individually audited; the middleware's
// single entry is suppressed in favor of the per-container entries.
func (h *Handlers) BulkContainers(w http.ResponseWriter, r *http.Request) {
	var body bulkRequest
	if err := decodeJSON(r, &body); err != nil || !bulk.Valid(body.Action) || len(body.ContainerIDs) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_bulk_request")
		return
	}
	// Authorize by action: remove is destructive (admin); the rest are operator.
	if body.Action == bulk.ActionRemove {
		if !h.requireAdmin(w, r) {
			return
		}
	} else if !h.requireOperator(w, r) {
		return
	}
	// Step-up 2FA for a wide blast radius (F7: bulk affecting 3+ containers).
	if len(body.ContainerIDs) >= 3 && !h.requireStepUp(w, r) {
		return
	}
	if len(body.ContainerIDs) > bulkMaxContainers {
		writeError(w, http.StatusBadRequest, "too_many_containers")
		return
	}

	// Resolve each id; collect found containers + not-found results (order kept).
	var found []db.Container
	notFound := map[string]bool{}
	for _, id := range body.ContainerIDs {
		c, err := h.Store.GetContainer(r.Context(), id)
		if errors.Is(err, db.ErrNotFound) {
			notFound[id] = true
			continue
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error")
			return
		}
		found = append(found, c)
	}

	plan := bulk.Plan(body.Action, found)
	results := make([]bulkResultItem, 0, len(body.ContainerIDs))
	for id := range notFound {
		results = append(results, bulkResultItem{Item: bulk.Item{ContainerID: id}, Result: "not_found"})
	}

	if body.DryRun {
		for _, it := range plan {
			res := "planned"
			if it.Skip {
				res = "skipped"
			}
			results = append(results, bulkResultItem{Item: it, Result: res})
		}
		// Audit the dry-run as a single, non-mutating summary entry.
		if e := activity.FromContext(r.Context()); e != nil {
			e.Suppressed = true
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results, "dry_run": true})
		return
	}

	auditAction, do := dockerActionFor(body.Action)
	if do == nil {
		// Defensive: bulk.Valid accepted the action but dockerActionFor has no
		// mapping for it — a code-level divergence, not client input. Fail loud
		// rather than dereference a nil func.
		writeError(w, http.StatusBadRequest, "invalid_bulk_request")
		return
	}
	executed := h.runBulk(r, plan, do, auditAction)
	results = append(results, executed...)

	// Per-container entries are written directly; suppress the middleware's row.
	if e := activity.FromContext(r.Context()); e != nil {
		e.Suppressed = true
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "dry_run": false})
}

// runBulk executes the planned actions in parallel (bounded), auditing each
// attempt, and returns per-container results in plan order.
func (h *Handlers) runBulk(r *http.Request, plan []bulk.Item, do func(*docker.Client, context.Context, string) error, auditAction string) []bulkResultItem {
	results := make([]bulkResultItem, len(plan))
	var userID *string
	if u, ok := middleware.UserFromContext(r.Context()); ok {
		userID = &u.ID
	}

	sem := make(chan struct{}, bulkConcurrency)
	var wg sync.WaitGroup
	for i, it := range plan {
		if it.Skip {
			results[i] = bulkResultItem{Item: it, Result: "skipped"}
			continue
		}
		wg.Add(1)
		go func(i int, it bulk.Item) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = h.runBulkOne(r.Context(), it, do, auditAction, userID)
		}(i, it)
	}
	wg.Wait()
	return results
}

func (h *Handlers) runBulkOne(ctx context.Context, it bulk.Item, do func(*docker.Client, context.Context, string) error, auditAction string, userID *string) bulkResultItem {
	res := bulkResultItem{Item: it, Result: "ok"}
	clients, err := h.Conn.Get(ctx, it.NodeID)
	if err != nil || clients.Docker == nil {
		res.Result, res.Error = "error", "node_unreachable"
		h.auditBulk(ctx, auditAction, it, userID, activity.ResultError)
		return res
	}
	opCtx, cancel := context.WithTimeout(ctx, bulkOpTimeout)
	defer cancel()
	if err := do(clients.Docker, opCtx, it.DockerID); err != nil {
		// A conflict (e.g. remove of a container that's actually running, or an
		// already-in-state lifecycle action) is a state problem, not a failure.
		errCode := "action_failed"
		if cerrdefs.IsConflict(err) {
			errCode = "conflict"
		}
		res.Result, res.Error = "error", errCode
		h.auditBulk(ctx, auditAction, it, userID, activity.ResultError)
		return res
	}
	h.auditBulk(ctx, auditAction, it, userID, activity.ResultSuccess)
	return res
}

func (h *Handlers) auditBulk(ctx context.Context, action string, it bulk.Item, userID *string, result string) {
	// Detach from request cancellation: the daemon action may have already
	// executed (a removed container can't be un-removed), so its audit row MUST
	// be written even if the client disconnected mid-batch. Mirrors the activity
	// middleware's context.WithoutCancel handling.
	ctx = context.WithoutCancel(ctx)
	_ = h.Activity.Append(ctx, activity.Entry{
		UserID:     userID,
		Action:     action,
		TargetType: ptr(activity.TargetContainer),
		TargetID:   &it.ContainerID,
		Detail:     map[string]string{"node_id": it.NodeID, "container": it.Name, "bulk": "true"},
		Result:     result,
	})
}
