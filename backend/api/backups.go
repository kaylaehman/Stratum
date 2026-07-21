package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/backup"
	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/db"
)

// targetBackup is the activity target type for backup operations.
// The action constants backup.restore and backup.verify are defined in
// activity/actions.go by the orchestrator; referenced here as string literals
// so this file compiles before those constants are merged.
const targetBackup = "backup"

// actionBackupRestore and actionBackupVerify are forward-declared here as
// string literals matching the canonical names the orchestrator will add to
// activity/actions.go. When the orchestrator adds the constants, callers can
// replace these with the typed constants without API changes.
const (
	actionBackupRestore = "backup.restore"
	actionBackupVerify  = "backup.verify"
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

type startGuestBackupBody struct {
	Storage string `json:"storage"`
}

// StartGuestBackup triggers an async vzdump backup for a Proxmox VM or LXC.
// URL params: nodeId (Stratum node), vmid (Proxmox integer VMID).
// Admin-gated + audited.
func (h *Handlers) StartGuestBackup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	vmidStr := chi.URLParam(r, "vmid")

	vmid, err := strconv.Atoi(vmidStr)
	if err != nil || vmid <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_vmid")
		return
	}

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Proxmox {
		writeCapabilityUnavailable(w, "proxmox", "")
		return
	}

	// Look up the VM row to get the pveNode (Proxmox cluster node name).
	vms, err := h.Store.ListVMsByNode(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	pveNode := ""
	for _, v := range vms {
		if v.ProxmoxVMID == vmid {
			pveNode = v.ProxmoxNode
			break
		}
	}
	if pveNode == "" {
		writeError(w, http.StatusNotFound, "vm_not_found")
		return
	}

	var body startGuestBackupBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	id, err := h.Backups.StartGuestBackup(r.Context(), nodeID, pveNode, vmid, body.Storage)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup_failed")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = activity.ActionBackupGuestStart
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
		e.Detail = map[string]any{"vmid": vmid, "pve_node": pveNode, "storage": body.Storage, "backup_id": id}
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"backup_id": id})
}

// restoreVolumeBody is the request body for RestoreVolumeBackup.
type restoreVolumeBody struct {
	ArchivePath string `json:"archive_path"`
	TargetPath  string `json:"target_path"`
}

// RestoreVolumeBackup restores a volume archive to a path on a node.
// This is a DESTRUCTIVE operation that overwrites files in target_path.
// Requires admin + step-up (TOTP confirmation). Audited as backup.restore.
func (h *Handlers) RestoreVolumeBackup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
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
	_ = node // capabilities check not needed for restore (SSH-level operation)

	var body restoreVolumeBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if !backup.ValidArchivePath(body.ArchivePath) || !backup.ValidDestDir(body.TargetPath) {
		writeError(w, http.StatusBadRequest, "invalid_path")
		return
	}

	out, err := h.Backups.RestoreVolume(r.Context(), nodeID, body.ArchivePath, body.TargetPath)
	if errors.Is(err, backup.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "invalid_path")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "restore_failed")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = actionBackupRestore
		e.TargetType = ptr(targetBackup)
		e.TargetID = &nodeID
		e.Detail = map[string]string{
			"archive_path": body.ArchivePath,
			"target_path":  body.TargetPath,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": out})
}

// restoreGuestBody is the request body for RestoreGuestBackup.
type restoreGuestBody struct {
	PVENode       string `json:"pve_node"`
	ArchivePath   string `json:"archive_path"`
	TargetStorage string `json:"target_storage"`
	TargetVMID    int    `json:"target_vmid"` // 0 = auto-assign
}

// RestoreGuestBackup restores a Proxmox vzdump archive to a storage pool.
// This is a DESTRUCTIVE operation. Requires admin + step-up. Audited.
func (h *Handlers) RestoreGuestBackup(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
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
	if !caps.Proxmox {
		writeCapabilityUnavailable(w, "proxmox", "")
		return
	}

	var body restoreGuestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	if body.PVENode == "" || body.TargetStorage == "" {
		writeError(w, http.StatusBadRequest, "pve_node_and_storage_required")
		return
	}
	if !backup.ValidArchivePath(body.ArchivePath) {
		writeError(w, http.StatusBadRequest, "invalid_archive_path")
		return
	}

	upid, err := h.Backups.RestoreGuest(r.Context(), nodeID, body.PVENode, body.ArchivePath, body.TargetStorage, body.TargetVMID)
	if errors.Is(err, backup.ErrInvalidInput) {
		writeError(w, http.StatusBadRequest, "invalid_path")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "restore_failed")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = actionBackupRestore
		e.TargetType = ptr(targetBackup)
		e.TargetID = &nodeID
		e.Detail = map[string]any{
			"kind":           "proxmox",
			"pve_node":       body.PVENode,
			"archive_path":   body.ArchivePath,
			"target_storage": body.TargetStorage,
			"target_vmid":    body.TargetVMID,
			"upid":           upid,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"upid": upid})
}

// VerifyBackup performs a restore-drill on the newest completed volume archive
// for a node: extracts to scratch, stats the result, cleans up, returns pass/fail.
// Requires operator. Audited as backup.verify.
func (h *Handlers) VerifyBackup(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")

	if _, err := h.Store.GetNode(r.Context(), nodeID); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	res, err := h.Backups.VerifyLatest(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "verify_failed")
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = actionBackupVerify
		e.TargetType = ptr(targetBackup)
		e.TargetID = &nodeID
		e.Detail = map[string]any{
			"backup_id":   res.BackupID,
			"passed":      res.Passed,
			"file_count":  res.FileCount,
			"total_bytes": res.TotalBytes,
		}
	}

	status := http.StatusOK
	writeJSON(w, status, res)
}

// ListBackupVerifyResults returns the verify-drill history for a node.
// Admin-gated.
func (h *Handlers) ListBackupVerifyResults(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	if _, err := h.Store.GetNode(r.Context(), nodeID); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	results, err := h.Backups.ListVerifyResults(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
