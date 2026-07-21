package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/security"
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

	// Freshen the security + image-update caches without blocking on a cold full
	// scan, so the posture page doesn't hang ~10s on first load. The collectors
	// below then read cached data; a cold scan finishes in the background and
	// warms the next load.
	if caps.Docker {
		h.freshenNodeSecurity(node.ID)
	}

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

	// --- SSH key staleness ---
	// Left at -1 (unavailable): Stratum has no last-used data source for keys yet
	// (that needs sshd-log parsing, not persisted). The previous code did a real
	// SSH round-trip per node here and discarded the result, only adding latency
	// to the posture hot path — so it is intentionally not invoked.

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
	// Cache is freshened up-front by NodePosture (freshenNodeSecurity); read only.
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

// collectUpdateCount returns the count of containers on the node with status
// "update_available".
func collectUpdateCount(r *http.Request, h *Handlers, nodeID string) int {
	ctx := r.Context()
	// Cache is freshened up-front by NodePosture (freshenNodeSecurity); read only.
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
