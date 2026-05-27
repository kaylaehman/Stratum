package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/backup"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
)

type backupView struct {
	ID         string  `json:"id"`
	NodeID     string  `json:"node_id"`
	Kind       string  `json:"kind"`
	Target     string  `json:"target"`
	DestPath   string  `json:"dest_path"`
	SizeBytes  int64   `json:"size_bytes"`
	Status     string  `json:"status"`
	Error      string  `json:"error,omitempty"`
	StartedAt  string  `json:"started_at"`
	FinishedAt *string `json:"finished_at,omitempty"`
}

func toBackupView(b db.BackupRow) backupView {
	v := backupView{
		ID: b.ID, NodeID: b.NodeID, Kind: b.Kind, Target: b.Target, DestPath: b.DestPath,
		SizeBytes: b.SizeBytes, Status: b.Status, Error: b.Error,
		StartedAt: b.StartedAt.UTC().Format(time.RFC3339),
	}
	if b.FinishedAt != nil {
		f := b.FinishedAt.UTC().Format(time.RFC3339)
		v.FinishedAt = &f
	}
	return v
}

// ListBackups returns the backup history. Admin-gated.
func (h *Handlers) ListBackups(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	rows, err := h.Backups.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]backupView, len(rows))
	for i, b := range rows {
		out[i] = toBackupView(b)
	}
	writeJSON(w, http.StatusOK, map[string]any{"backups": out})
}

type startBackupBody struct {
	Volume  string `json:"volume"`
	DestDir string `json:"dest_dir"`
}

// StartBackup kicks off an async volume backup on a node. Admin-gated + audited.
func (h *Handlers) StartBackup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Docker {
		writeError(w, http.StatusConflict, "docker_not_available")
		return
	}
	var body startBackupBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	id, err := h.Backups.StartVolumeBackup(r.Context(), nodeID, body.Volume, body.DestDir)
	if errors.Is(err, backup.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "invalid_volume_or_dest")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionBackupStart
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
		e.Detail = map[string]string{"volume": body.Volume, "dest": body.DestDir, "backup_id": id}
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"backup_id": id})
}
