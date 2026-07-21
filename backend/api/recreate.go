package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/middleware"
	"github.com/KAE-Labs/stratum/backend/recreate"
)

// recreateTimeout bounds an image pull + container recreate. Detached from the
// request context so a client disconnect can't abort a destructive op midway.
const recreateTimeout = 15 * time.Minute

// snapshotView is the API shape of a rollback snapshot (never carries spec_json).
type snapshotView struct {
	ID          string `json:"id"`
	Reason      string `json:"reason"`
	ImageRef    string `json:"image_ref"`
	ImageDigest string `json:"image_digest,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func toSnapshotView(s db.Snapshot) snapshotView {
	return snapshotView{
		ID: s.ID, Reason: s.Reason, ImageRef: s.ImageRef, ImageDigest: s.ImageDigest,
		CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339),
	}
}

// ListSnapshots returns a container's rollback snapshots (read-only).
func (h *Handlers) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	snaps, err := h.Recreate.List(r.Context(), id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]snapshotView, 0, len(snaps))
	for _, s := range snaps {
		out = append(out, toSnapshotView(s))
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": out})
}

// SnapshotContainer saves a manual rollback checkpoint (admin). Audited.
func (h *Handlers) SnapshotContainer(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctr, ok := h.resolveContainerRow(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 2*time.Minute)
	defer cancel()
	snap, err := h.Recreate.Snapshot(ctx, ctr.ID, "manual")
	if err != nil {
		h.logRecreate(r, "snapshot", ctr, err)
		h.auditContainer(r, activity.ActionContainerSnapshot, ctr, errResult(err))
		writeError(w, http.StatusBadGateway, "snapshot_failed")
		return
	}
	h.auditContainer(r, activity.ActionContainerSnapshot, ctr, nil)
	writeJSON(w, http.StatusCreated, toSnapshotView(snap))
}

// UpdateContainer pulls the latest image and recreates the container with the
// same config, after taking a pre-update snapshot (admin). Audited.
func (h *Handlers) UpdateContainer(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctr, ok := h.resolveContainerRow(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), recreateTimeout)
	defer cancel()
	newID, err := h.Recreate.Update(ctx, ctr.ID)
	if err != nil {
		h.logRecreate(r, "update", ctr, err)
		h.auditContainer(r, activity.ActionContainerUpdate, ctr, errResult(err))
		writeError(w, http.StatusBadGateway, "update_failed")
		return
	}
	h.afterRecreate(ctr.NodeID)
	h.auditContainer(r, activity.ActionContainerUpdate, ctr, map[string]string{"new_container": shortID(newID)})
	writeJSON(w, http.StatusOK, map[string]string{"new_container_id": newID})
}

// RollbackContainer restores a container from a snapshot (admin + step-up 2FA).
// Audited.
func (h *Handlers) RollbackContainer(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}
	ctr, ok := h.resolveContainerRow(w, r)
	if !ok {
		return
	}
	snapID := chi.URLParam(r, "snap")
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), recreateTimeout)
	defer cancel()
	newID, err := h.Recreate.Rollback(ctx, snapID, ctr.NodeID, ctr.Name)
	if errors.Is(err, db.ErrNotFound) || errors.Is(err, recreate.ErrSnapshotMismatch) {
		// Same 404 for "no such snapshot" and "snapshot belongs to another
		// container" — don't disclose other containers' snapshot ids.
		writeError(w, http.StatusNotFound, "snapshot_not_found")
		return
	} else if err != nil {
		h.logRecreate(r, "rollback", ctr, err)
		h.auditContainer(r, activity.ActionContainerRollback, ctr, errResult(err))
		writeError(w, http.StatusBadGateway, "rollback_failed")
		return
	}
	h.afterRecreate(ctr.NodeID)
	h.auditContainer(r, activity.ActionContainerRollback, ctr, map[string]string{"snapshot": snapID, "new_container": shortID(newID)})
	writeJSON(w, http.StatusOK, map[string]string{"new_container_id": newID})
}

type healthcheckEditRequest struct {
	Disable        bool     `json:"disable"`
	Test           []string `json:"test"`
	IntervalSec    int      `json:"interval_sec"`
	TimeoutSec     int      `json:"timeout_sec"`
	StartPeriodSec int      `json:"start_period_sec"`
	Retries        int      `json:"retries"`
}

// SetHealthcheck edits a container's healthcheck and recreates it (admin). The
// edit (or disable) is applied by recreating the container from a snapshot-backed
// spec. Audited.
func (h *Handlers) SetHealthcheck(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctr, ok := h.resolveContainerRow(w, r)
	if !ok {
		return
	}
	var body healthcheckEditRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if !body.Disable && len(body.Test) == 0 {
		writeError(w, http.StatusBadRequest, "test_required")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), recreateTimeout)
	defer cancel()
	newID, err := h.Recreate.SetHealthcheck(ctx, ctr.ID, recreate.HealthcheckEdit{
		Disable:        body.Disable,
		Test:           body.Test,
		IntervalSec:    body.IntervalSec,
		TimeoutSec:     body.TimeoutSec,
		StartPeriodSec: body.StartPeriodSec,
		Retries:        body.Retries,
	})
	if err != nil {
		h.logRecreate(r, "healthcheck", ctr, err)
		h.auditContainer(r, activity.ActionContainerHealthcheck, ctr, errResult(err))
		writeError(w, http.StatusBadGateway, "healthcheck_failed")
		return
	}
	h.afterRecreate(ctr.NodeID)
	detail := map[string]string{"new_container": shortID(newID)}
	if body.Disable {
		detail["action"] = "disabled"
	}
	h.auditContainer(r, activity.ActionContainerHealthcheck, ctr, detail)
	writeJSON(w, http.StatusOK, map[string]string{"new_container_id": newID})
}

// --- helpers ---

// resolveContainerRow loads the container row (for nodeID/name/audit) and maps a
// missing row to 404.
func (h *Handlers) resolveContainerRow(w http.ResponseWriter, r *http.Request) (db.Container, bool) {
	ctr, err := h.Store.GetContainer(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return db.Container{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return db.Container{}, false
	}
	return ctr, true
}

// afterRecreate invalidates caches that depend on the now-changed container so
// the next read reflects the new image. The inventory poller reconciles the
// container row (new docker id) on its next cycle.
func (h *Handlers) afterRecreate(nodeID string) {
	if h.Updater != nil {
		h.Updater.Invalidate(nodeID)
	}
}

func (h *Handlers) auditContainer(r *http.Request, action string, ctr db.Container, detail map[string]string) {
	e := activity.FromContext(r.Context())
	if e == nil {
		return
	}
	e.Action = action
	e.TargetType = ptr(activity.TargetContainer)
	e.TargetID = &ctr.ID
	base := map[string]string{"node_id": ctr.NodeID, "container": ctr.Name}
	for k, v := range detail {
		base[k] = v
	}
	e.Detail = base
}

func (h *Handlers) logRecreate(r *http.Request, op string, ctr db.Container, err error) {
	if h.Logger != nil {
		uid := ""
		if u, ok := middleware.UserFromContext(r.Context()); ok {
			uid = u.ID
		}
		h.Logger.Warn("recreate op failed", "op", op, "container", ctr.Name, "node", ctr.NodeID, "user", uid, "error", err)
	}
}

func errResult(err error) map[string]string {
	if err == nil {
		return nil
	}
	return map[string]string{"error": err.Error()}
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
