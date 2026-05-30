package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
)

// vmPowerTimeout bounds a single Proxmox guest power-state change. Proxmox
// queues the action and responds with a UPID synchronously, so this only covers
// the API round-trip, not the guest boot/shutdown itself.
const vmPowerTimeout = 15 * time.Second

// vmActions is the allow-list of Proxmox guest power actions the API accepts.
// Matches the Proxmox API surface: qemu and lxc both support these four.
var vmActions = map[string]string{
	"start":    activity.ActionVMStart,
	"stop":     activity.ActionVMStop,
	"shutdown": activity.ActionVMShutdown,
	"reboot":   activity.ActionVMReboot,
}

// VMPowerAction handles POST /api/nodes/{id}/vms/{vmid}/{action}.
// It is operator-gated, audited, and gated on node.type == proxmox.
func (h *Handlers) VMPowerAction(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}

	action := chi.URLParam(r, "action")
	auditAction, ok := vmActions[action]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_action")
		return
	}

	nodeID := chi.URLParam(r, "id")
	vmidStr := chi.URLParam(r, "vmid")
	vmid, err := strconv.Atoi(vmidStr)
	if err != nil || vmid < 1 {
		writeError(w, http.StatusBadRequest, "invalid_vmid")
		return
	}

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	// Gate on proxmox capability — never call the Proxmox API for non-proxmox nodes.
	caps, authStatus := parseCaps(node.CapabilitiesJSON)
	if !caps.Proxmox {
		writeCapabilityUnavailable(w, "proxmox", "")
		return
	}
	if authStatus != "confirmed" {
		writeCapabilityUnavailable(w, "proxmox", "proxmox_auth_status="+authStatus)
		return
	}

	// Resolve the VM from the DB so we can record its name + kind in the audit log.
	vms, err := h.Store.ListVMsByNode(r.Context(), nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	var target *db.VM
	for i := range vms {
		if vms[i].ProxmoxVMID == vmid {
			target = &vms[i]
			break
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}

	// Enrich the audit entry before the action runs so it is recorded even on failure.
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = auditAction
		e.TargetType = ptr(activity.TargetVM)
		e.TargetID = &target.ID
		e.Detail = map[string]any{
			"node_id":      nodeID,
			"proxmox_vmid": vmid,
			"kind":         target.Kind,
			"name":         target.Name,
		}
	}

	clients, err := h.Conn.Get(r.Context(), nodeID)
	if err != nil || clients.Proxmox == nil {
		writeError(w, http.StatusBadGateway, "proxmox_unreachable")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), vmPowerTimeout)
	defer cancel()

	upid, err := clients.Proxmox.GuestPowerAction(ctx, target.ProxmoxNode, target.Kind, vmid, action)
	if err != nil {
		writeError(w, http.StatusBadGateway, "proxmox_action_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"upid": upid})
}
