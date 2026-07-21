package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/KAE-Labs/stratum/backend/activity"
	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/middleware"
	"github.com/KAE-Labs/stratum/backend/security"
)

// securityFreshenBudget bounds how long a security GET waits for an in-progress
// refresh before serving cached data. A cold full scan (serial Docker inspects)
// can take many seconds; without this bound the first page load blocked on it.
const securityFreshenBudget = 1500 * time.Millisecond

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

// freshenDockerSecurity refreshes all docker nodes' security data WITHOUT
// blocking the request on a cold full scan. The scan is detached to a background
// context so it completes (and warms the cache) even after the response is sent;
// the request waits only securityFreshenBudget. A warm/fast scan returns inline
// with fresh data; a cold scan returns cached now and finishes in the background,
// so the next load is instant and fresh. Fixes the ~10s first-load hang.
func (h *Handlers) freshenDockerSecurity() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.ensureSecurityForDockerNodes(context.Background())
	}()
	select {
	case <-done:
	case <-time.After(securityFreshenBudget):
	}
}

// freshenNodeSecurity is freshenDockerSecurity for a single node: it kicks the
// security + image-update refreshes detached and waits only the shared budget.
func (h *Handlers) freshenNodeSecurity(nodeID string) {
	done := make(chan struct{}, 2)
	go func() { _ = h.Security.EnsureFresh(context.Background(), nodeID); done <- struct{}{} }()
	go func() { _ = h.Updater.EnsureFresh(context.Background(), nodeID); done <- struct{}{} }()
	deadline := time.After(securityFreshenBudget)
	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-deadline:
			return
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
	h.freshenDockerSecurity()
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
	h.freshenDockerSecurity()
	ports, err := h.Store.ListAllPortExposures(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	// A nil slice marshals to JSON null; the UI expects an array. Coerce so the
	// client never has to defend against `ports: null`.
	if ports == nil {
		ports = []db.PortExposureRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ports":                ports,
		"non_docker_listeners": h.nonDockerListeners(ctx, ports),
	})
}

// nonDockerListeners cross-references host ss listeners against Docker-published
// ports per node and reports every listener NOT explained by a Docker port
// mapping. We deliberately do NOT suppress a node because it runs a host-network
// container: that would hide genuinely rogue listeners (a false negative is the
// dangerous outcome for a security audit). The cost is that a host-network
// container's own listeners surface here — a safe over-report the admin can
// recognize, not a hidden risk.
func (h *Handlers) nonDockerListeners(ctx context.Context, ports []db.PortExposureRow) []map[string]any {
	nodes, err := h.Store.ListNodes(ctx)
	if err != nil {
		return nil
	}
	out := []map[string]any{}
	for _, n := range nodes {
		caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
		if !caps.Docker {
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
	if !h.requireAdmin(w, r) {
		return
	}
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
