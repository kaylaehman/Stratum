package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/inventory"
)

type treeNode struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	Type              string                    `json:"type"`
	Host              string                    `json:"host"`
	Status            string                    `json:"status"`
	Capabilities      capabilities.Set          `json:"capabilities"`
	ProxmoxAuthStatus string                    `json:"proxmox_auth_status"`
	Seq               uint64                    `json:"seq"`
	VMs               []inventory.VMView        `json:"vms"`
	Containers        []inventory.ContainerView `json:"containers"`
}

func parseCaps(capabilitiesJSON string) (capabilities.Set, string) {
	caps, _ := capabilities.Parse([]byte(capabilitiesJSON))
	var env struct {
		ProxmoxAuthStatus string `json:"proxmox_auth_status"`
	}
	_ = json.Unmarshal([]byte(capabilitiesJSON), &env)
	if env.ProxmoxAuthStatus == "" {
		env.ProxmoxAuthStatus = "none"
	}
	return caps, env.ProxmoxAuthStatus
}

func (h *Handlers) seqFor(nodeID string) uint64 {
	if h.Poller == nil {
		return 0
	}
	return h.Poller.CurrentSeq(nodeID)
}

// Tree returns the whole forest (cached DB rows) plus each node's current seq
// for client resync. First paint; the poller keeps it fresh over the hub.
func (h *Handlers) Tree(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.Store.ListNodes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	out := make([]treeNode, 0, len(nodes))
	for _, n := range nodes {
		caps, authStatus := parseCaps(n.CapabilitiesJSON)
		tn := treeNode{
			ID: n.ID, Name: n.Name, Type: n.Type, Host: n.Host, Status: n.Status,
			Capabilities: caps, ProxmoxAuthStatus: authStatus, Seq: h.seqFor(n.ID),
			VMs: []inventory.VMView{}, Containers: []inventory.ContainerView{},
		}
		if vms, err := h.Store.ListVMsByNode(r.Context(), n.ID); err == nil {
			for _, v := range vms {
				tn.VMs = append(tn.VMs, inventory.FromVM(v))
			}
		}
		if cs, err := h.Store.ListContainersByNode(r.Context(), n.ID); err == nil {
			for _, c := range cs {
				tn.Containers = append(tn.Containers, inventory.FromContainer(c))
			}
		}
		out = append(out, tn)
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": out})
}

// NodeVMs lists a node's Proxmox guests; 422 if the node isn't a confirmed
// Proxmox node.
func (h *Handlers) NodeVMs(w http.ResponseWriter, r *http.Request) {
	n, err := h.Store.GetNode(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	caps, authStatus := parseCaps(n.CapabilitiesJSON)
	if !caps.Proxmox {
		writeCapabilityUnavailable(w, "proxmox", "")
		return
	}
	if authStatus != "confirmed" {
		writeCapabilityUnavailable(w, "proxmox", "proxmox_auth_status="+authStatus)
		return
	}
	vms, err := h.Store.ListVMsByNode(r.Context(), n.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	views := make([]inventory.VMView, 0, len(vms))
	for _, v := range vms {
		views = append(views, inventory.FromVM(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"vms": views})
}

// NodeContainers lists a node's Docker containers; 422 if the node has no Docker.
func (h *Handlers) NodeContainers(w http.ResponseWriter, r *http.Request) {
	n, err := h.Store.GetNode(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	caps, _ := parseCaps(n.CapabilitiesJSON)
	if !caps.Docker {
		writeCapabilityUnavailable(w, "docker", "")
		return
	}
	cs, err := h.Store.ListContainersByNode(r.Context(), n.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	views := make([]inventory.ContainerView, 0, len(cs))
	for _, c := range cs {
		views = append(views, inventory.FromContainer(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": views})
}

func writeCapabilityUnavailable(w http.ResponseWriter, capability, detail string) {
	body := map[string]string{"error": "capability_unavailable", "capability": capability}
	if detail != "" {
		body["detail"] = detail
	}
	writeJSON(w, http.StatusUnprocessableEntity, body)
}
