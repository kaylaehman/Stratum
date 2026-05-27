package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/middleware"
	"github.com/kaylaehman/stratum/backend/security"
)

// ensureSecurityForDockerNodes re-scans every docker-capable node (best-effort).
func (h *Handlers) ensureSecurityForDockerNodes(ctx context.Context) {
	nodes, err := h.Store.ListNodes(ctx)
	if err != nil {
		return
	}
	for _, n := range nodes {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if caps.Docker {
			_ = h.Security.EnsureFresh(ctx, n.ID)
		}
	}
}

func ackKey(flagType, flagKey string) string { return flagType + "|" + flagKey }

// ackedFlags returns the set of acknowledged flag keys for a container.
func ackedFlags(acks []db.SecurityAck, containerID string) map[string]bool {
	m := map[string]bool{}
	for _, a := range acks {
		if a.ContainerID == containerID {
			m[ackKey(a.FlagType, a.FlagKey)] = true
		}
	}
	return m
}

type flagView struct {
	security.Flag
	Acknowledged bool `json:"acknowledged"`
}

func flagViews(row db.ContainerSecurityRow, acked map[string]bool) []flagView {
	out := []flagView{}
	for _, f := range security.FlagsFor(row) {
		out = append(out, flagView{Flag: f, Acknowledged: acked[ackKey(f.Type, f.Key)]})
	}
	return out
}

func hasUnacknowledged(views []flagView) bool {
	for _, v := range views {
		if !v.Acknowledged {
			return true
		}
	}
	return false
}

// Privileged lists containers with security flags + their acknowledge state.
func (h *Handlers) Privileged(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.ensureSecurityForDockerNodes(r.Context())
	rows, err := h.Store.ListContainerSecurity(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	acks, _ := h.Store.ListAcks(r.Context())
	type entry struct {
		ContainerID string     `json:"container_id"`
		NodeID      string     `json:"node_id"`
		Flags       []flagView `json:"flags"`
	}
	out := []entry{}
	for _, row := range rows {
		views := flagViews(row, ackedFlags(acks, row.ContainerID))
		if len(views) == 0 {
			continue
		}
		out = append(out, entry{ContainerID: row.ContainerID, NodeID: row.NodeID, Flags: views})
	}
	writeJSON(w, http.StatusOK, map[string]any{"containers": out})
}

// Ports lists published ports (classified) cross-node + non-Docker host listeners.
func (h *Handlers) Ports(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctx := r.Context()
	h.ensureSecurityForDockerNodes(ctx)
	ports, err := h.Store.ListAllPortExposures(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ports":                 ports,
		"non_docker_listeners": h.nonDockerListeners(ctx, ports),
	})
}

// nonDockerListeners cross-references host ss listeners against Docker-published
// ports per node, suppressing nodes that have a host-network container (their
// in-container listeners legitimately appear at host level).
func (h *Handlers) nonDockerListeners(ctx context.Context, ports []db.PortExposureRow) []map[string]any {
	nodes, err := h.Store.ListNodes(ctx)
	if err != nil {
		return nil
	}
	secRows, _ := h.Store.ListContainerSecurity(ctx)
	out := []map[string]any{}
	for _, n := range nodes {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if !caps.Docker {
			continue
		}
		// Suppress if any container on the node uses host networking.
		hostNet := false
		for _, s := range secRows {
			if s.NodeID == n.ID && s.NetHost {
				hostNet = true
			}
		}
		if hostNet {
			continue
		}
		dockerPorts := map[int]bool{}
		for _, p := range ports {
			if p.NodeID == n.ID {
				dockerPorts[p.HostPort] = true
			}
		}
		res := security.GetListeners(ctx, n.ID, h.Files.Exec)
		if !res.Available {
			continue
		}
		for _, l := range res.Listeners {
			if !dockerPorts[l.Port] {
				out = append(out, map[string]any{
					"node_id": n.ID, "protocol": l.Protocol, "address": l.Address,
					"port": l.Port, "process": l.Process,
				})
			}
		}
	}
	return out
}

// ContainerSecurity returns one container's flags + ports + ack state.
func (h *Handlers) ContainerSecurity(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	_ = h.Security.EnsureFresh(r.Context(), ctr.NodeID)
	row, err := h.Store.GetContainerSecurity(r.Context(), ctr.ID)
	if errors.Is(err, db.ErrNotFound) {
		row = db.ContainerSecurityRow{ContainerID: ctr.ID, NodeID: ctr.NodeID}
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	acks, _ := h.Store.ListAcks(r.Context())
	ports, _ := h.Store.ListPortExposuresByContainer(r.Context(), ctr.ID)
	writeJSON(w, http.StatusOK, map[string]any{
		"flags": flagViews(row, ackedFlags(acks, ctr.ID)),
		"ports": ports,
	})
}

// SecurityBadges returns a sparse {containerID: true} overlay of containers with
// an unacknowledged flag (the SP2 tree merges this client-side). Fast: no rescan.
func (h *Handlers) SecurityBadges(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Store.ListContainerSecurity(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	acks, _ := h.Store.ListAcks(r.Context())
	badges := map[string]bool{}
	for _, row := range rows {
		if hasUnacknowledged(flagViews(row, ackedFlags(acks, row.ContainerID))) {
			badges[row.ContainerID] = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"badges": badges})
}

type ackBody struct {
	ContainerID string `json:"container_id"`
	FlagType    string `json:"flag_type"`
	FlagKey     string `json:"flag_key"`
	Note        string `json:"note"`
}

// AcknowledgeFlag records an acknowledgement, suppressing a flag's badge.
func (h *Handlers) AcknowledgeFlag(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var body ackBody
	if err := decodeJSON(r, &body); err != nil || body.ContainerID == "" || body.FlagType == "" {
		writeError(w, http.StatusBadRequest, "container_id_and_flag_type_required")
		return
	}
	ctr, err := h.Store.GetContainer(r.Context(), body.ContainerID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	user, _ := middleware.UserFromContext(r.Context())
	if err := h.Store.InsertAck(r.Context(), db.SecurityAck{
		ID: uuid.NewString(), NodeID: ctr.NodeID, ContainerID: ctr.ID,
		FlagType: body.FlagType, FlagKey: body.FlagKey, AcknowledgedBy: &user.ID, Note: body.Note,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "ack_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = "security.acknowledge"
		e.TargetType = ptr("container")
		e.TargetID = &ctr.ID
		e.Detail = map[string]string{"flag_type": body.FlagType, "flag_key": body.FlagKey}
	}
	w.WriteHeader(http.StatusCreated)
}

// RevokeAcknowledgement deletes an acknowledgement by id.
func (h *Handlers) RevokeAcknowledgement(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.Store.DeleteAck(r.Context(), id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "revoke_failed")
		return
	}
	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = "security.revoke_acknowledge"
		e.TargetType = ptr("acknowledgement")
		e.TargetID = &id
	}
	w.WriteHeader(http.StatusNoContent)
}

// Rescan forces a re-scan of a node (or all docker nodes).
func (h *Handlers) Rescan(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	node := r.URL.Query().Get("node")
	if node != "" {
		h.Security.Invalidate(node)
		_ = h.Security.EnsureFresh(r.Context(), node)
	} else {
		h.ensureSecurityForDockerNodes(r.Context())
	}
	w.WriteHeader(http.StatusNoContent)
}
