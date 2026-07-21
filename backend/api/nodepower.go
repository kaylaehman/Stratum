package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/db"
)

// nodePowerTimeout bounds a single Proxmox host power-state change. Proxmox
// queues the action and responds with a UPID synchronously, so this only covers
// the API round-trip, not the host reboot/shutdown itself.
const nodePowerTimeout = 15 * time.Second

// nodePowerActions is the allow-list of host power actions. No "start" —
// a powered-off physical host cannot be started via its own API.
var nodePowerActions = map[string]string{
	"shutdown": activity.ActionNodeShutdown,
	"reboot":   activity.ActionNodeReboot,
}

// NodePowerAction handles POST /api/nodes/{id}/power/{action}.
// Admin-only (physical host power-off is the most destructive possible action).
// Audited. Gated on node.type == proxmox + confirmed proxmox auth.
func (h *Handlers) NodePowerAction(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	action := chi.URLParam(r, "action")
	auditAction, ok := nodePowerActions[action]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_action")
		return
	}

	nodeID := chi.URLParam(r, "id")

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

	// Enrich the audit entry before the action runs so it is recorded even on failure.
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = auditAction
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
		e.Detail = map[string]any{
			"node_id": nodeID,
			"name":    node.Name,
			"host":    node.Host,
			"action":  action,
		}
	}

	clients, err := h.Conn.Get(r.Context(), nodeID)
	if err != nil || clients.Proxmox == nil {
		writeError(w, http.StatusBadGateway, "proxmox_unreachable")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), nodePowerTimeout)
	defer cancel()

	// Resolve the Proxmox cluster node name for this endpoint. We use
	// LocalNodeName (cluster/status) rather than storing it on the Node row
	// because vms.go reads it from VM rows which are populated by the poller;
	// for a host power action we need the authoritative name from the API.
	pveNode, err := clients.Proxmox.LocalNodeName(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, "proxmox_node_name_unknown")
		return
	}

	upid, err := clients.Proxmox.NodePowerAction(ctx, pveNode, action)
	if err != nil {
		writeError(w, http.StatusBadGateway, "proxmox_action_failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"upid": upid})
}
