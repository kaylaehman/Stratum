package api

import (
	"net/http"

	"github.com/KAE-Labs/stratum/backend/diagnostic"
	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/permissions"
)

type diagnosticBody struct {
	HostPath string `json:"host_path"`
}

type diagnosticResult struct {
	HostPath  string                    `json:"host_path"`
	FileUID   int                       `json:"file_uid"`
	FileGID   int                       `json:"file_gid"`
	FileMode  string                    `json:"file_mode"`
	RunUID    int                       `json:"run_uid"`
	RunGID    int                       `json:"run_gid"`
	IsRoot    bool                      `json:"is_root"`
	Exposure  docker.Exposure           `json:"bind_mount"`
	Verdict   permissions.Verdict       `json:"verdict"`
	ACL       permissions.ACLResult     `json:"acl"`
	Effective diagnostic.EffectiveAccess `json:"effective_access"`
	Steps     []diagnostic.Step         `json:"steps"`
	Fixes     []diagnostic.SuggestedFix `json:"fixes"`
}

// Diagnostic explains whether a container can access a host file, with a
// step-by-step narrative and suggested fixes ("Why is this broken?").
func (h *Handlers) Diagnostic(w http.ResponseWriter, r *http.Request) {
	// The diagnostic stats arbitrary host paths (container-vs-host access
	// analysis) — host reconnaissance, so operator-gated like the host-FS reads.
	if !h.requireOperator(w, r) {
		return
	}
	ctr, clients, ok := h.resolveContainer(w, r)
	if !ok {
		return
	}
	var body diagnosticBody
	if err := decodeJSON(r, &body); err != nil || body.HostPath == "" {
		writeError(w, http.StatusBadRequest, "host_path_required")
		return
	}

	entry, err := h.Files.StatEntry(r.Context(), ctr.NodeID, body.HostPath)
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

	exposure := docker.Forward(body.HostPath, info.Mounts)
	verdict := permissions.FileAnalysis(
		permissions.FileFacts{UID: entry.UID, GID: entry.GID, ModeOctal: entry.ModeOctal},
		id, hostMaps.UIDToName, hostMaps.GIDToName, cu.UIDToName)

	// getfacl over SSH (quoted, -- terminated). Missing getfacl -> not evaluated.
	acl := permissions.GetACL(r.Context(), ctr.NodeID, body.HostPath, h.Files.Exec)
	effective := diagnostic.Reconcile(verdict, acl)

	inputs := diagnostic.Inputs{
		HostPath: body.HostPath, FileUID: entry.UID, FileGID: entry.GID, FileMode: entry.ModeOctal,
		HostOwner: entry.Owner, HostGroup: entry.Group,
		RunUID: id.UID, RunGID: id.GID, RunName: cu.UIDToName[id.UID], IsRoot: id.IsRoot,
		Exposure: exposure, ACL: acl, Effective: effective,
	}

	writeJSON(w, http.StatusOK, diagnosticResult{
		HostPath: body.HostPath, FileUID: entry.UID, FileGID: entry.GID, FileMode: entry.ModeOctal,
		RunUID: id.UID, RunGID: id.GID, IsRoot: id.IsRoot,
		Exposure: exposure, Verdict: verdict, ACL: acl, Effective: effective,
		Steps: diagnostic.Narrative(inputs), Fixes: diagnostic.Fixes(inputs),
	})
}
