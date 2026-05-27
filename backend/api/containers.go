package api

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/permissions"
)

// resolveContainer loads the container row + its node's docker client, enforcing
// the docker capability gate. Writes the error response and returns ok=false on
// failure.
func (h *Handlers) resolveContainer(w http.ResponseWriter, r *http.Request) (db.Container, *nodeconn.Clients, bool) {
	ctr, err := h.Store.GetContainer(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return db.Container{}, nil, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return db.Container{}, nil, false
	}
	node, err := h.Store.GetNode(r.Context(), ctr.NodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return db.Container{}, nil, false
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if err := capabilities.Require(caps, capabilities.Docker); err != nil {
		writeCapabilityUnavailable(w, "docker", "")
		return db.Container{}, nil, false
	}
	clients, err := h.Conn.Get(r.Context(), ctr.NodeID)
	if err != nil || clients.Docker == nil {
		writeError(w, http.StatusBadGateway, "docker_unreachable")
		return db.Container{}, nil, false
	}
	return ctr, clients, true
}

type inspectResponse struct {
	docker.InspectInfo
	RunUID            int   `json:"run_uid"`
	RunGID            int   `json:"run_gid"`
	SupplementaryGIDs []int `json:"supplementary_gids"`
	IsRoot            bool  `json:"is_root"`
}

// InspectContainer returns the container inspect subset plus the resolved
// effective run identity.
func (h *Handlers) InspectContainer(w http.ResponseWriter, r *http.Request) {
	ctr, clients, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	info, err := clients.Docker.Inspect(r.Context(), ctr.DockerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "inspect_failed")
		return
	}
	cu, _ := h.ContainerUsers.ResolveContainer(r.Context(), ctr.NodeID, ctr.DockerID)
	id := permissions.EffectiveIdentity(info.ConfigUser, cu.Passwd, cu.Group)
	writeJSON(w, http.StatusOK, inspectResponse{
		InspectInfo: info, RunUID: id.UID, RunGID: id.GID,
		SupplementaryGIDs: id.SupplementaryGIDs, IsRoot: id.IsRoot,
	})
}

// HostUsers returns a node's host user/group tables.
func (h *Handlers) HostUsers(w http.ResponseWriter, r *http.Request) {
	maps, err := h.Files.ResolveUsers(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadGateway, "host_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"uids": maps.UIDToName, "gids": maps.GIDToName})
}

// ContainerUsersHandler returns a container's user/group tables.
func (h *Handlers) ContainerUsersHandler(w http.ResponseWriter, r *http.Request) {
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	cu, _ := h.ContainerUsers.ResolveContainer(r.Context(), ctr.NodeID, ctr.DockerID)
	writeJSON(w, http.StatusOK, map[string]any{"uids": cu.UIDToName, "gids": cu.GIDToName})
}

func (h *Handlers) computeAnalysis(ctx context.Context, ctr db.Container) (uidRows, gidRows []permissions.Row, legend map[string]int, err error) {
	hostMaps, err := h.Files.ResolveUsers(ctx, ctr.NodeID)
	if err != nil {
		return nil, nil, nil, err
	}
	cu, _ := h.ContainerUsers.ResolveContainer(ctx, ctr.NodeID, ctr.DockerID)
	uidRows = permissions.Mismatch(hostMaps.UIDToName, cu.UIDToName)
	gidRows = permissions.Mismatch(hostMaps.GIDToName, cu.GIDToName)
	legend = map[string]int{}
	for _, row := range append(append([]permissions.Row{}, uidRows...), gidRows...) {
		legend[row.Class]++
	}
	return uidRows, gidRows, legend, nil
}

// UIDAnalysis returns the host-vs-container mismatch rows + legend counts.
func (h *Handlers) UIDAnalysis(w http.ResponseWriter, r *http.Request) {
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	uidRows, gidRows, legend, err := h.computeAnalysis(r.Context(), ctr)
	if err != nil {
		writeError(w, http.StatusBadGateway, "host_unreachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"uid_rows": uidRows, "gid_rows": gidRows, "legend": legend})
}

// UIDAnalysisCSV downloads the mismatch report as CSV.
func (h *Handlers) UIDAnalysisCSV(w http.ResponseWriter, r *http.Request) {
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	uidRows, gidRows, _, err := h.computeAnalysis(r.Context(), ctr)
	if err != nil {
		writeError(w, http.StatusBadGateway, "host_unreachable")
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=uid-analysis.csv")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"kind", "id", "host_name", "container_name", "on_host", "on_container", "class"})
	writeRows := func(kind string, rows []permissions.Row) {
		for _, row := range rows {
			_ = cw.Write([]string{kind, strconv.Itoa(row.ID), row.HostName, row.ContainerName,
				strconv.FormatBool(row.OnHost), strconv.FormatBool(row.OnContainer), row.Class})
		}
	}
	writeRows("uid", uidRows)
	writeRows("gid", gidRows)
	cw.Flush()
}

// UIDAnalysisJSON downloads the mismatch report as a JSON file.
func (h *Handlers) UIDAnalysisJSON(w http.ResponseWriter, r *http.Request) {
	ctr, _, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	uidRows, gidRows, legend, err := h.computeAnalysis(r.Context(), ctr)
	if err != nil {
		writeError(w, http.StatusBadGateway, "host_unreachable")
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=uid-analysis.json")
	writeJSON(w, http.StatusOK, map[string]any{"uid_rows": uidRows, "gid_rows": gidRows, "legend": legend})
}

// FileUID returns the full DAC access verdict for a host file vs the container's
// effective identity.
func (h *Handlers) FileUID(w http.ResponseWriter, r *http.Request) {
	ctr, clients, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	hostPath := r.URL.Query().Get("host_path")
	if hostPath == "" {
		writeError(w, http.StatusBadRequest, "host_path_required")
		return
	}
	entry, err := h.Files.StatEntry(r.Context(), ctr.NodeID, hostPath)
	if err != nil {
		writeFSError(w, err)
		return
	}
	info, err := clients.Docker.Inspect(r.Context(), ctr.DockerID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "inspect_failed")
		return
	}
	cu, _ := h.ContainerUsers.ResolveContainer(r.Context(), ctr.NodeID, ctr.DockerID)
	id := permissions.EffectiveIdentity(info.ConfigUser, cu.Passwd, cu.Group)
	hostMaps, err := h.Files.ResolveUsers(r.Context(), ctr.NodeID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "host_unreachable")
		return
	}
	verdict := permissions.FileAnalysis(
		permissions.FileFacts{UID: entry.UID, GID: entry.GID, ModeOctal: entry.ModeOctal},
		id, hostMaps.UIDToName, hostMaps.GIDToName, cu.UIDToName)
	writeJSON(w, http.StatusOK, verdict)
}
