package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/security"
	"github.com/kaylaehman/stratum/backend/sshkeys"
)

// NodePosture computes a security/health posture score for one node by composing
// data Stratum already stores — no new scanners are invoked. It is read-only and
// returns a partial result when a data source is unavailable (never errors on
// missing capabilities). Admin-gated.
func (h *Handlers) NodePosture(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	node, err := h.Store.GetNode(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))

	in := security.PostureInputs{
		CriticalCVEs:         -1,
		HighCVEs:             -1,
		PrivilegedCount:      -1,
		ExposedAllIfaceCount: -1,
		StaleSSHKeyCount:     -1,
		UpdateAvailableCount: -1,
	}

	// --- CVE counts (requires Docker capability + CVE scanner) ---
	if caps.Docker && h.CVE.Available() {
		in.CriticalCVEs, in.HighCVEs = collectCVECounts(r, h, node.ID)
	}

	// --- Privileged / dangerous container flags ---
	if caps.Docker {
		in.PrivilegedCount = collectPrivilegedCount(r, h, node.ID)
	}

	// --- Ports bound to all interfaces ---
	if caps.Docker {
		in.ExposedAllIfaceCount = collectExposedPortCount(r, h, node.ID)
	}

	// --- SSH key staleness (best-effort over SSH; skip if unavailable) ---
	in.StaleSSHKeyCount = collectStaleSSHKeyCount(r, h, node.ID)

	// --- Containers with available image updates ---
	if caps.Docker {
		in.UpdateAvailableCount = collectUpdateCount(r, h, node.ID)
	}

	result := security.Score(in)
	writeJSON(w, http.StatusOK, result)
}

// collectCVECounts sums critical+high CVEs across all containers on the node
// by joining container image digests with stored scan results.
func collectCVECounts(r *http.Request, h *Handlers, nodeID string) (critical, high int) {
	ctx := r.Context()
	containers, err := h.Store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return 0, 0
	}
	scans, err := h.CVE.ListScans(ctx)
	if err != nil {
		return 0, 0
	}
	scanByDigest := make(map[string]db.ImageScanRow, len(scans))
	for _, s := range scans {
		scanByDigest[s.ImageDigest] = s
	}
	seen := make(map[string]bool)
	for _, c := range containers {
		if c.ImageID == "" || seen[c.ImageID] {
			continue
		}
		seen[c.ImageID] = true
		if s, ok := scanByDigest[c.ImageID]; ok {
			critical += s.Critical
			high += s.High
		}
	}
	return critical, high
}

// collectPrivilegedCount returns the count of containers on the node that have
// at least one unacknowledged security flag.
func collectPrivilegedCount(r *http.Request, h *Handlers, nodeID string) int {
	ctx := r.Context()
	_ = h.Security.EnsureFresh(ctx, nodeID)
	rows, err := h.Store.ListContainerSecurity(ctx)
	if err != nil {
		return 0
	}
	acks, _ := h.Store.ListAcks(ctx)
	count := 0
	for _, row := range rows {
		if row.NodeID != nodeID {
			continue
		}
		views := flagViews(row, ackedFlags(acks, row.ContainerID))
		if hasUnacknowledged(views) {
			count++
		}
	}
	return count
}

// collectExposedPortCount returns the count of port bindings on the node that
// are bound to all interfaces (0.0.0.0 / ::).
func collectExposedPortCount(r *http.Request, h *Handlers, nodeID string) int {
	ctx := r.Context()
	ports, err := h.Store.ListAllPortExposures(ctx)
	if err != nil {
		return 0
	}
	count := 0
	for _, p := range ports {
		if p.NodeID == nodeID && p.InterfaceClass == security.IfaceAll {
			count++
		}
	}
	return count
}

// collectStaleSSHKeyCount audits SSH keys on the node over SSH (best-effort).
// Because Stratum does not store last-used timestamps for keys (that requires
// sshd log parsing which is agent-dependent), we return -1 (unavailable) when
// no usage data is available rather than mis-reporting. If the SSH key audit
// itself fails, we return -1.
func collectStaleSSHKeyCount(r *http.Request, h *Handlers, nodeID string) int {
	// SSH key audit runs over SSH; failure gracefully returns -1.
	keys, err := sshkeys.Audit(r.Context(), nodeID, h.Files.Exec)
	if err != nil {
		return -1
	}
	// Stratum currently has no last-used data source for keys — that data comes
	// from sshd log parsing which is not yet persisted. Return count of all keys
	// as "potentially stale" only when the count is known, but since we can't
	// distinguish age we skip this factor entirely (return -1) rather than
	// reporting false positives. This matches the spec: "best-effort; skip
	// gracefully if data absent."
	_ = keys
	return -1
}

// collectUpdateCount returns the count of containers on the node with status
// "update_available".
func collectUpdateCount(r *http.Request, h *Handlers, nodeID string) int {
	ctx := r.Context()
	_ = h.Updater.EnsureFresh(ctx, nodeID)
	rows, err := h.Updater.ListAll(ctx)
	if err != nil {
		return 0
	}
	count := 0
	for _, row := range rows {
		if row.NodeID == nodeID && row.Status == "update_available" {
			count++
		}
	}
	return count
}
