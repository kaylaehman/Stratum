package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/backup"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/cve"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/diagnostic"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/forecast"
	"github.com/kaylaehman/stratum/backend/fs"
	"github.com/kaylaehman/stratum/backend/metrics"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/permissions"
	"github.com/kaylaehman/stratum/backend/recreate"
	"github.com/kaylaehman/stratum/backend/remediation"
	"github.com/kaylaehman/stratum/backend/security"
	"github.com/kaylaehman/stratum/backend/updates"
	"github.com/kaylaehman/stratum/backend/volumes"
)

// managerNames joins detected management-tool names for a log/detail line.
func managerNames(tools []updates.ManagementTool) string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	return strings.Join(names, "+")
}

// Deps holds all service references required to build the 15 handlers.
// All fields that are nil cause the relevant handler to return "skipped".
type Deps struct {
	Store       db.Store
	Conn        *nodeconn.Manager
	Security    *security.Scanner
	CVE         *cve.Service
	Volumes     *volumes.Service
	Recreate    *recreate.Service
	Backups     *backup.Service
	Remediation *remediation.Service
	Files       *fs.Service
	Forecast    *forecast.Service
	Notify      func(ctx context.Context, trigger, title, text string)
}

// BuildHandlers constructs the full handler map. Each handler closes over the
// services it needs and reads its live config from the DB on each call.
func BuildHandlers(store db.Store, deps Deps) map[string]Handler {
	return map[string]Handler{
		"restart_unhealthy":         restartUnhealthyHandler(store, deps.Conn),
		"auto_remediate_low":        autoRemediateLowHandler(store, deps.Remediation),
		"auto_pull_updates":         autoPullUpdatesHandler(store, deps.Conn),
		"auto_update_containers":    autoUpdateContainersHandler(store, deps.Conn, deps.Recreate),
		"scheduled_cve_scan":        scheduledCVEScanHandler(store, deps.CVE),
		"security_alerts":           securityAlertsHandler(store, deps.Security, deps.CVE),
		"prune_unused_volumes":      pruneUnusedVolumesHandler(deps.Volumes),
		"scheduled_backups":         scheduledBackupsHandler(store, deps.Backups),
		"restart_on_resource_spike": restartOnResourceSpikeHandler(store, deps.Conn),
		"fix_bind_mount_perms":      fixBindMountPermsHandler(store, deps.Files),
		"run_runbooks_on_alert":     runRunbooksOnAlertHandler(store, deps.Remediation),
		"patch_critical_cves":       patchCriticalCVEsHandler(store, deps.CVE, deps.Recreate, deps.Conn),
		"prune_disk_pressure":       pruneDiskPressureHandler(store, deps.Files),
		"verify_backup":             verifyBackupHandler(store, deps.Backups, deps.Notify),
		"capacity_warn":             capacityWarnHandler(store, deps.Forecast, deps.Notify),
	}
}

// restartUnhealthyHandler restarts containers whose Docker status is "exited"
// or whose health-check state is "unhealthy". Uses the inventory (containers
// table) + Docker client. Best-effort per container; errors are accumulated.
func restartUnhealthyHandler(store db.Store, conn *nodeconn.Manager) Handler {
	return func(ctx context.Context) (string, error) {
		if conn == nil {
			return "skipped: no node connection manager", nil
		}
		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		restarted, errs := 0, []string{}
		for _, n := range nodes {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			clients, err := conn.Get(ctx, n.ID)
			if err != nil || clients.Docker == nil {
				continue
			}
			containers, err := store.ListContainersByNode(ctx, n.ID)
			if err != nil {
				continue
			}
			for _, c := range containers {
				needsRestart := false
				if c.Status == "exited" {
					needsRestart = true
				}
				if !needsRestart {
					// check health-check report
					report, err := clients.Docker.ContainerHealth(ctx, c.DockerID)
					if err == nil && report.Status == "unhealthy" {
						needsRestart = true
					}
				}
				if !needsRestart {
					continue
				}
				if err := clients.Docker.RestartContainer(ctx, c.DockerID); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", c.Name, err))
					continue
				}
				restarted++
			}
		}
		detail := fmt.Sprintf("restarted %d container(s)", restarted)
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d restart error(s)", len(errs))
		}
		return detail, nil
	}
}

// autoRemediateLowHandler scans for proposed low-risk remediation proposals and
// executes them. SAFETY: only RiskLow proposals are touched; medium/high/
// destructive proposals are never auto-executed.
func autoRemediateLowHandler(store db.Store, remSvc *remediation.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if remSvc == nil {
			return "skipped: remediation service unavailable", nil
		}
		proposals, err := remSvc.List(ctx, "") // all nodes
		if err != nil {
			return "", fmt.Errorf("list proposals: %w", err)
		}
		executed, skipped, errs := 0, 0, []string{}
		for _, p := range proposals {
			if p.Status != remediation.StatusProposed {
				continue
			}
			// Primary safety gate: only low-risk proposals.
			if p.RiskLevel != remediation.RiskLow {
				skipped++
				continue
			}
			// Auto-approve as system, then execute.
			approved, err := remSvc.Approve(ctx, p.ID, "automation")
			if err != nil {
				errs = append(errs, fmt.Sprintf("approve %s: %v", p.ID, err))
				continue
			}
			_ = approved
			if _, err := remSvc.Execute(ctx, p.ID); err != nil {
				errs = append(errs, fmt.Sprintf("execute %s: %v", p.ID, err))
				continue
			}
			executed++
		}
		detail := fmt.Sprintf("executed %d low-risk proposal(s), skipped %d non-low", executed, skipped)
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d execution error(s)", len(errs))
		}
		return detail, nil
	}
}

// autoPullUpdatesHandler pulls the latest image for each running container
// across all docker-capable nodes. Does NOT recreate containers.
func autoPullUpdatesHandler(store db.Store, conn *nodeconn.Manager) Handler {
	return func(ctx context.Context) (string, error) {
		if conn == nil {
			return "skipped: no node connection manager", nil
		}
		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		pulled, errs := 0, []string{}
		for _, n := range nodes {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			clients, err := conn.Get(ctx, n.ID)
			if err != nil || clients.Docker == nil {
				continue
			}
			containers, err := store.ListContainersByNode(ctx, n.ID)
			if err != nil {
				continue
			}
			seen := map[string]bool{}
			for _, c := range containers {
				if c.Status != "running" || c.Image == "" || seen[c.Image] {
					continue
				}
				seen[c.Image] = true
				if err := clients.Docker.PullImage(ctx, c.Image); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", c.Image, err))
					continue
				}
				pulled++
			}
		}
		detail := fmt.Sprintf("pulled %d image(s)", pulled)
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d pull error(s)", len(errs))
		}
		return detail, nil
	}
}

// autoUpdateContainersHandler pulls + recreates containers whose compose
// project is in the config allowlist. OFF by default. Reads live config from DB.
func autoUpdateContainersHandler(store db.Store, conn *nodeconn.Manager, recreateSvc *recreate.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if conn == nil || recreateSvc == nil {
			return "skipped: dependencies unavailable", nil
		}
		// Load live config.
		allowedProjects := allowedProjectsFromDB(ctx, store, "auto_update_containers")
		if len(allowedProjects) == 0 {
			return "skipped: no projects in allowlist (set config.projects)", nil
		}

		containers, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		updated, errs, skipped := 0, []string{}, []string{}
		for _, n := range containers {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			ctrs, err := store.ListContainersByNode(ctx, n.ID)
			if err != nil {
				continue
			}
			// Overlap guard: if this host runs a tool that updates containers on
			// its own (Watchtower), don't fight it — skip the host and report it.
			images := make([]string, 0, len(ctrs))
			for _, c := range ctrs {
				images = append(images, c.Image)
			}
			if managers := updates.DetectManagers(images); updates.HasAutoUpdater(managers) {
				skipped = append(skipped, fmt.Sprintf("%s (runs %s)", n.Name, managerNames(managers)))
				continue
			}
			for _, c := range ctrs {
				if !allowedProjects[c.ComposeProject] {
					continue
				}
				if _, err := recreateSvc.Update(ctx, c.ID); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", c.Name, err))
					continue
				}
				updated++
			}
		}
		detail := fmt.Sprintf("updated %d container(s) in allowlisted projects", updated)
		if len(skipped) > 0 {
			detail += "; skipped hosts with their own updater: " + strings.Join(skipped, ", ")
		}
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d update error(s)", len(errs))
		}
		return detail, nil
	}
}

// cveScanTarget holds one entry from config.targets for scheduled_cve_scan.
// NodeID targets all running containers on a node; ContainerID targets a single
// container. Both may appear in the same list. Empty targets = scan everything.
type cveScanTarget struct {
	NodeID      string `json:"node_id"`
	ContainerID string `json:"container_id"`
}

// cveScanCfgFromDB reads config.targets for scheduled_cve_scan. Returns nil
// (meaning "scan everything") when no config row exists or targets is empty.
func cveScanCfgFromDB(ctx context.Context, store db.Store) []cveScanTarget {
	row, err := store.GetAutomation(ctx, "scheduled_cve_scan")
	if err != nil {
		return nil
	}
	var cfg struct {
		Targets []cveScanTarget `json:"targets"`
	}
	if err := json.Unmarshal([]byte(row.ConfigJSON), &cfg); err != nil {
		return nil
	}
	return cfg.Targets
}

// scheduledCVEScanHandler runs a bulk CVE scan over running containers.
// When config.targets is non-empty it limits the scan to those nodes/containers;
// an empty (or absent) targets list scans all running containers.
func scheduledCVEScanHandler(store db.Store, cveSvc *cve.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if cveSvc == nil || !cveSvc.Available() {
			return "skipped: CVE scanner (trivy/grype) not available", nil
		}
		targets := cveScanCfgFromDB(ctx, store)
		var toScan []db.Container

		if len(targets) == 0 {
			// Default: scan all running containers across all docker-capable nodes.
			nodes, err := store.ListNodes(ctx)
			if err != nil {
				return "", fmt.Errorf("list nodes: %w", err)
			}
			for _, n := range nodes {
				caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
				if !caps.Docker {
					continue
				}
				ctrs, err := store.ListContainersByNode(ctx, n.ID)
				if err != nil {
					continue
				}
				for _, c := range ctrs {
					if c.Status == "running" {
						toScan = append(toScan, c)
					}
				}
			}
		} else {
			// Targeted: honour the per-node / per-container list from config.
			for _, t := range targets {
				switch {
				case t.ContainerID != "":
					c, err := store.GetContainer(ctx, t.ContainerID)
					if err != nil || c.Status != "running" {
						continue
					}
					toScan = append(toScan, c)
				case t.NodeID != "":
					ctrs, err := store.ListContainersByNode(ctx, t.NodeID)
					if err != nil {
						continue
					}
					for _, c := range ctrs {
						if c.Status == "running" {
							toScan = append(toScan, c)
						}
					}
				}
			}
		}

		if len(toScan) == 0 {
			return "no running containers to scan", nil
		}
		results := cveSvc.ScanBulk(ctx, toScan)
		scanned, errs := 0, []string{}
		for _, r := range results {
			if r.Error != "" {
				errs = append(errs, fmt.Sprintf("%s: %s", r.Image, r.Error))
			} else {
				scanned++
			}
		}
		detail := fmt.Sprintf("scanned %d container(s)", scanned)
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d scan error(s)", len(errs))
		}
		return detail, nil
	}
}

// securityAlertsHandler refreshes security data and lets the scanner's built-in
// notify callbacks fire. Returns a summary of what was rescanned.
func securityAlertsHandler(store db.Store, secScanner *security.Scanner, cveSvc *cve.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if secScanner == nil {
			return "skipped: security scanner unavailable", nil
		}
		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		rescanned := 0
		for _, n := range nodes {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			// Invalidate cache so EnsureFresh always re-scans.
			secScanner.Invalidate(n.ID)
			if err := secScanner.EnsureFresh(ctx, n.ID); err == nil {
				rescanned++
			}
		}
		return fmt.Sprintf("rescanned security posture for %d node(s)", rescanned), nil
	}
}

// pruneUnusedVolumesHandler removes volumes not attached to any container.
func pruneUnusedVolumesHandler(volumeSvc *volumes.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if volumeSvc == nil {
			return "skipped: volume service unavailable", nil
		}
		results, err := volumeSvc.PruneUnused(ctx, "") // all nodes
		if err != nil {
			return "", fmt.Errorf("prune unused volumes: %w", err)
		}
		removed, failed := 0, 0
		for _, r := range results {
			if r.OK {
				removed++
			} else {
				failed++
			}
		}
		detail := fmt.Sprintf("removed %d volume(s)", removed)
		if failed > 0 {
			detail += fmt.Sprintf(", %d failed", failed)
			return detail, fmt.Errorf("%d volume removal error(s)", failed)
		}
		return detail, nil
	}
}

// scheduledBackupsHandler triggers a volume backup on the configured node.
// Config keys: node_id, volume, dest_dir. All three must be set.
func scheduledBackupsHandler(store db.Store, backupSvc *backup.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if backupSvc == nil {
			return "skipped: backup service unavailable", nil
		}
		row, err := store.GetAutomation(ctx, "scheduled_backups")
		if err != nil {
			return "skipped: no configuration stored", nil
		}
		var cfg struct {
			NodeID  string `json:"node_id"`
			Volume  string `json:"volume"`
			DestDir string `json:"dest_dir"`
		}
		if err := json.Unmarshal([]byte(row.ConfigJSON), &cfg); err != nil || cfg.NodeID == "" || cfg.Volume == "" || cfg.DestDir == "" {
			return "skipped: node_id, volume, and dest_dir must be set in config", nil
		}
		id, err := backupSvc.StartVolumeBackup(ctx, cfg.NodeID, cfg.Volume, cfg.DestDir)
		if err != nil {
			return "", fmt.Errorf("start backup: %w", err)
		}
		return fmt.Sprintf("backup %s started (id=%s)", cfg.Volume, id), nil
	}
}

// restartOnResourceSpikeHandler restarts containers that have exceeded cpu_pct
// or mem_pct thresholds for at least window_minutes consecutive samples.
// Wires to metrics.DetectSpikes (same logic as the incident timeline) and the
// same Docker RestartContainer path as restartUnhealthy.
func restartOnResourceSpikeHandler(store db.Store, conn *nodeconn.Manager) Handler {
	return func(ctx context.Context) (string, error) {
		if conn == nil {
			return "skipped: no node connection manager", nil
		}
		cfg := resourceSpikeCfgFromDB(ctx, store)
		windowDur := time.Duration(cfg.windowMinutes) * time.Minute
		now := time.Now()
		from := now.Add(-windowDur)

		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		restarted, skipped, errs := 0, 0, []string{}
		for _, n := range nodes {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			clients, err := conn.Get(ctx, n.ID)
			if err != nil || clients.Docker == nil {
				continue
			}
			ctrs, err := store.ListContainersByNode(ctx, n.ID)
			if err != nil {
				continue
			}
			for _, c := range ctrs {
				if c.Status != "running" {
					continue
				}
				samples, err := store.ListResourceSamples(ctx, c.ID, from, now)
				if err != nil || len(samples) == 0 {
					skipped++
					continue
				}
				if !spikeExceedsWindow(samples, cfg, windowDur) {
					skipped++
					continue
				}
				if err := clients.Docker.RestartContainer(ctx, c.DockerID); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", c.Name, err))
					continue
				}
				restarted++
			}
		}
		detail := fmt.Sprintf("restarted %d container(s) due to resource spike, skipped %d (no spike data or below threshold)", restarted, skipped)
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d restart error(s)", len(errs))
		}
		return detail, nil
	}
}

type resourceSpikeCfg struct {
	cpuPct        float64
	memPct        float64
	windowMinutes float64
}

func resourceSpikeCfgFromDB(ctx context.Context, store db.Store) resourceSpikeCfg {
	cfg := resourceSpikeCfg{cpuPct: 90, memPct: 90, windowMinutes: 15}
	row, err := store.GetAutomation(ctx, "restart_on_resource_spike")
	if err != nil {
		return cfg
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(row.ConfigJSON), &m); err != nil {
		return cfg
	}
	if v, ok := m["cpu_pct"].(float64); ok && v > 0 {
		cfg.cpuPct = v
	}
	if v, ok := m["mem_pct"].(float64); ok && v > 0 {
		cfg.memPct = v
	}
	if v, ok := m["window_minutes"].(float64); ok && v > 0 {
		cfg.windowMinutes = v
	}
	return cfg
}

// spikeExceedsWindow returns true when ALL samples in the window exceed either
// the cpu or mem threshold, i.e. a sustained (not momentary) spike.
func spikeExceedsWindow(samples []db.ResourceSample, cfg resourceSpikeCfg, window time.Duration) bool {
	if len(samples) == 0 {
		return false
	}
	// The window must cover at least 2 samples to avoid single-point noise.
	if len(samples) < 2 {
		return false
	}
	// Check whether the spike detection reports at least one spike whose
	// duration spans the configured window.
	for _, spike := range metrics.DetectSpikes(samples) {
		dur := spike.To.Sub(spike.From)
		if dur < window {
			continue
		}
		// spike metric is "cpu" or "mem"; map to configured threshold.
		if spike.Metric == "cpu" && cfg.cpuPct > 0 && spike.Peak >= cfg.cpuPct {
			return true
		}
		if spike.Metric == "mem" && cfg.memPct > 0 {
			// mem spike.Peak is bytes; we need the fraction vs limit.
			// DetectSpikes uses MemSpikeFraction (0.90). If the operator
			// configures mem_pct=90, that maps to the same threshold.
			// Accept the spike when the configured threshold <= the built-in
			// MemSpikeFraction*100 (both expressed as percent).
			if cfg.memPct <= metrics.MemSpikeFraction*100 {
				return true
			}
		}
	}
	return false
}

// fixBindMountPermsHandler scans running containers for bind-mount UID/GID
// permission problems using the diagnostic engine. When dry_run=true (default)
// it logs the fix it would apply without executing. When dry_run=false it
// creates a low-risk remediation proposal via the remediation store so the
// existing approval flow applies.
func fixBindMountPermsHandler(store db.Store, filesSvc *fs.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if filesSvc == nil {
			return "skipped: filesystem service unavailable", nil
		}
		dryRun := bindMountDryRunFromDB(ctx, store)

		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		found, acted, errs := 0, 0, []string{}
		for _, n := range nodes {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			mounts, err := store.ListMountsByNode(ctx, n.ID)
			if err != nil {
				continue
			}
			for _, m := range mounts {
				if m.Type != "bind" || m.Source == "" {
					continue
				}
				entry, err := filesSvc.StatEntry(ctx, n.ID, m.Source)
				if err != nil {
					continue
				}
				// Build minimal Inputs: we only have host-side stat here (no
				// container process UID at this layer). A mismatch is flagged
				// when UID 0 (root) does not own the file and permissions are
				// restrictive. Use Fixes() with a placeholder identity to derive
				// if a concrete command is available.
				secRow, serr := store.GetContainerSecurity(ctx, m.ContainerID)
				if serr != nil {
					continue
				}
				// Skip root containers – they can read anything.
				if secRow.RunsAsRoot {
					continue
				}
				fileUID := entry.UID
				runUID := secRow.RunUID
				// Only flag when the container UID differs from the file owner
				// and the file is not world-readable (mode has no 'other' read).
				if fileUID == runUID {
					continue
				}
				if !fileIsRestrictedForOther(entry.ModeOctal) {
					continue
				}
				found++
				in := buildDiagInputs(entry, m, secRow)
				fixes := diagnostic.Fixes(in)
				if len(fixes) == 0 {
					continue
				}
				// Only act on fixes that produce a concrete command (not comments).
				fix := firstExecutableFix(fixes)
				if fix == "" {
					continue
				}
				if dryRun {
					acted++
					errs = append(errs, fmt.Sprintf("[dry-run] would run: %s (container %s, path %s)", fix, m.ContainerID, m.Source))
					continue
				}
				// Create a low-risk remediation proposal so the approval flow applies.
				if err := store.CreateProposal(ctx, db.RemediationProposal{
					ID:          uuid.NewString(),
					Source:      "runbook",
					Title:       fmt.Sprintf("Fix bind-mount permission: %s", m.Source),
					Rationale:   fmt.Sprintf("Container UID %d cannot read host path %s (owner UID %d)", runUID, m.Source, fileUID),
					NodeID:      n.ID,
					ContainerID: m.ContainerID,
					Commands:    []string{fix},
					RiskLevel:   remediation.RiskLow,
					Status:      remediation.StatusProposed,
					CreatedBy:   "automation",
					CreatedAt:   time.Now(),
				}); err != nil {
					errs = append(errs, fmt.Sprintf("create proposal %s: %v", m.Source, err))
					continue
				}
				acted++
			}
		}
		detail := fmt.Sprintf("found %d bind-mount permission issue(s), acted on %d", found, acted)
		if len(errs) > 0 {
			detail += "; details: " + strings.Join(errs, "; ")
		}
		return detail, nil
	}
}

func bindMountDryRunFromDB(ctx context.Context, store db.Store) bool {
	row, err := store.GetAutomation(ctx, "fix_bind_mount_perms")
	if err != nil {
		return true // default: dry run
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(row.ConfigJSON), &m); err != nil {
		return true
	}
	v, ok := m["dry_run"].(bool)
	if !ok {
		return true
	}
	return v
}

// fileIsRestrictedForOther returns true when the mode string (e.g. "0640")
// does not grant read to the "other" class.
func fileIsRestrictedForOther(mode string) bool {
	if len(mode) < 4 {
		return false
	}
	// Octal mode string like "0640": last digit is 'other'. Bit 2 = read.
	last := mode[len(mode)-1]
	n := int(last - '0')
	return n&4 == 0
}

// buildDiagInputs assembles a minimal diagnostic.Inputs for a bind-mount
// permission check. ACL data is omitted (would require an agent exec call);
// this gives enough context for Fixes() to emit a concrete suggestion.
func buildDiagInputs(entry fs.Entry, m db.MountRow, sec db.ContainerSecurityRow) diagnostic.Inputs {
	verdict := permissions.FileAnalysis(
		permissions.FileFacts{
			UID:       entry.UID,
			GID:       entry.GID,
			ModeOctal: entry.ModeOctal,
		},
		permissions.Identity{UID: sec.RunUID, GID: 0},
		nil, nil, nil,
	)
	ea := diagnostic.Reconcile(verdict, permissions.ACLResult{Available: false})
	exposure := docker.Exposure{
		Exposed:   true,
		RW:        m.RW,
		ViaSource: m.Source,
		ViaDest:   m.Destination,
	}
	return diagnostic.Inputs{
		HostPath:  m.Source,
		FileUID:   entry.UID,
		FileGID:   entry.GID,
		FileMode:  entry.ModeOctal,
		RunUID:  sec.RunUID,
		RunGID:  0, // ContainerSecurityRow does not store GID; default 0
		IsRoot:  sec.RunsAsRoot,
		Exposure:  exposure,
		Effective: ea,
	}
}

// firstExecutableFix returns the first fix command that isn't a comment/comment-only line.
func firstExecutableFix(fixes []diagnostic.SuggestedFix) string {
	for _, f := range fixes {
		if f.Command != "" && !strings.HasPrefix(strings.TrimSpace(f.Command), "#") {
			return f.Command
		}
	}
	return ""
}

// runRunbooksOnAlertHandler checks recent activity/incidents for severity
// matching min_severity; for each matching event it loads runbooks and, when
// a runbook's TriggerConditions contain any keyword from the event summary,
// creates a remediation proposal from that runbook.
func runRunbooksOnAlertHandler(store db.Store, remSvc *remediation.Service) Handler {
	return func(ctx context.Context) (string, error) {
		minSev := runbookMinSevFromDB(ctx, store)
		// Load recent activity that could represent an alert (last 10 minutes).
		since := time.Now().Add(-10 * time.Minute)
		rows, err := store.QueryActivityLog(ctx, db.ActivityQuery{
			Result: strPtr("error"),
			From:   &since,
			Limit:  100,
		})
		if err != nil {
			return "", fmt.Errorf("query activity: %w", err)
		}

		runbooks, err := store.ListRunbooks(ctx)
		if err != nil {
			return "", fmt.Errorf("list runbooks: %w", err)
		}
		if len(runbooks) == 0 {
			return "skipped: no runbooks configured", nil
		}

		matched, errs := 0, []string{}
		for _, row := range rows {
			if !activityMeetsMinSev(row, minSev) {
				continue
			}
			for _, rb := range runbooks {
				if !runbookMatchesActivity(rb, row) {
					continue
				}
				if remSvc == nil {
					matched++
					continue
				}
				// Resolve a node ID for the proposal. Only generate when the
				// activity target is a node; other target types are logged but
				// do not yield a proposal (node_id is required).
				nodeID := ""
				if ptrStrVal(row.TargetType) == "node" {
					nodeID = ptrStrVal(row.TargetID)
				}
				if nodeID == "" {
					// Log the match but skip proposal generation — insufficient context.
					matched++
					errs = append(errs, fmt.Sprintf("[info] runbook %q matched activity %q but no node_id resolvable; skipping proposal", rb.Name, row.Action))
					continue
				}
				_, genErr := remSvc.Generate(ctx, remediation.GenerateRequest{
					Source:    "runbook",
					Title:     fmt.Sprintf("Runbook: %s (triggered by %s)", rb.Name, row.Action),
					Rationale: rb.Description,
					NodeID:    nodeID,
					Commands:  rb.Steps,
				}, "automation")
				if genErr != nil {
					errs = append(errs, fmt.Sprintf("generate proposal for runbook %s: %v", rb.ID, genErr))
					continue
				}
				matched++
			}
		}
		detail := fmt.Sprintf("matched %d runbook(s) to recent alert(s)", matched)
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d proposal generation error(s)", len(errs))
		}
		return detail, nil
	}
}

func runbookMinSevFromDB(ctx context.Context, store db.Store) string {
	row, err := store.GetAutomation(ctx, "run_runbooks_on_alert")
	if err != nil {
		return "critical"
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(row.ConfigJSON), &m); err != nil {
		return "critical"
	}
	if v, ok := m["min_severity"].(string); ok && v != "" {
		return v
	}
	return "critical"
}

// activityMeetsMinSev maps the DB result field to a severity and compares.
// Currently only "error" result rows qualify as "critical"-level events.
func activityMeetsMinSev(row db.ActivityEntry, minSev string) bool {
	switch minSev {
	case "critical":
		return row.Result == "error"
	case "warning", "info":
		return true
	}
	return row.Result == "error"
}

// runbookMatchesActivity returns true when any of the runbook's
// TriggerConditions appears as a substring in the activity action or target.
func runbookMatchesActivity(rb db.Runbook, row db.ActivityEntry) bool {
	summary := strings.ToLower(row.Action + " " + ptrStrVal(row.TargetType) + " " + ptrStrVal(row.TargetID))
	for _, tc := range rb.TriggerConditions {
		if strings.Contains(summary, strings.ToLower(tc)) {
			return true
		}
	}
	return false
}

func ptrStrVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// patchCriticalCVEsHandler scans running images; for any container whose image
// has CRITICAL CVEs with a non-empty FixedVersion, it pulls+recreates the
// container (same path as auto_update_containers). When dry_run=true (default)
// only logs what it would do. Respects the projects allowlist.
func patchCriticalCVEsHandler(store db.Store, cveSvc *cve.Service, recreateSvc *recreate.Service, conn *nodeconn.Manager) Handler {
	return func(ctx context.Context) (string, error) {
		if cveSvc == nil {
			return "skipped: CVE service unavailable", nil
		}
		if recreateSvc == nil {
			return "skipped: recreate service unavailable", nil
		}
		dryRun, allowlist := patchCVECfgFromDB(ctx, store)

		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		would, patched, errs := 0, 0, []string{}
		for _, n := range nodes {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			ctrs, err := store.ListContainersByNode(ctx, n.ID)
			if err != nil {
				continue
			}
			for _, c := range ctrs {
				if c.Status != "running" {
					continue
				}
				if len(allowlist) > 0 && !allowlist[c.ComposeProject] {
					continue
				}
				// Resolve the scan key for this container.
				digest := resolveCVEDigest(c)
				if digest == "" {
					continue
				}
				vulns, err := store.ListCVEResults(ctx, digest)
				if err != nil {
					continue
				}
				if !hasCriticalWithFix(vulns) {
					continue
				}
				would++
				if dryRun {
					errs = append(errs, fmt.Sprintf("[dry-run] would pull+recreate %s (image %s, has critical CVE with fix)", c.Name, c.Image))
					continue
				}
				if _, err := recreateSvc.Update(ctx, c.ID); err != nil {
					errs = append(errs, fmt.Sprintf("update %s: %v", c.Name, err))
					continue
				}
				patched++
			}
		}
		var detail string
		if dryRun {
			detail = fmt.Sprintf("dry-run: %d container(s) would be updated for critical CVE patches", would)
		} else {
			detail = fmt.Sprintf("patched %d container(s) with critical CVEs", patched)
		}
		if len(errs) > 0 {
			detail += "; details: " + strings.Join(errs, "; ")
		}
		return detail, nil
	}
}

func patchCVECfgFromDB(ctx context.Context, store db.Store) (dryRun bool, allowlist map[string]bool) {
	dryRun = true
	row, err := store.GetAutomation(ctx, "patch_critical_cves")
	if err != nil {
		return
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(row.ConfigJSON), &m); err != nil {
		return
	}
	if v, ok := m["dry_run"].(bool); ok {
		dryRun = v
	}
	if raw, ok := m["allowlist"].([]any); ok && len(raw) > 0 {
		allowlist = make(map[string]bool, len(raw))
		for _, p := range raw {
			if s, ok := p.(string); ok && s != "" {
				allowlist[s] = true
			}
		}
	}
	return
}

// resolveCVEDigest returns the best available scan-cache key for a container
// (mirrors cve.Service.resolveDigest but without a docker client).
func resolveCVEDigest(c db.Container) string {
	if c.ImageID != "" {
		return c.ImageID
	}
	return c.Image
}

// hasCriticalWithFix returns true when any CRITICAL vuln has a non-empty
// FixedVersion, meaning an upgrade is available.
func hasCriticalWithFix(vulns []db.CVEResultRow) bool {
	for _, v := range vulns {
		if v.Severity == cve.SevCritical && v.FixedVersion != "" {
			return true
		}
	}
	return false
}

// pruneDiskPressureHandler checks host free disk via `df` over SSH; when free
// disk percentage falls below min_free_pct it prunes dangling Docker images and
// builder cache. Only acts when below the threshold.
func pruneDiskPressureHandler(store db.Store, filesSvc *fs.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if filesSvc == nil {
			return "skipped: filesystem service unavailable", nil
		}
		minFreePct := diskPressureCfgFromDB(ctx, store)

		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		pruned, skipped, errs := 0, 0, []string{}
		for _, n := range nodes {
			freePct, dfErr := queryDiskFreePct(ctx, filesSvc, n.ID)
			if dfErr != nil {
				skipped++
				continue
			}
			if freePct >= minFreePct {
				skipped++
				continue
			}
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				skipped++
				continue
			}
			// Prune dangling images.
			if _, err := filesSvc.Exec(ctx, n.ID, "docker", "image", "prune", "-f"); err != nil {
				errs = append(errs, fmt.Sprintf("node %s image prune: %v", n.ID, err))
			}
			// Prune builder cache.
			if _, err := filesSvc.Exec(ctx, n.ID, "docker", "builder", "prune", "-f"); err != nil {
				// Builder prune is best-effort; older Docker versions may not support it.
				errs = append(errs, fmt.Sprintf("node %s builder prune (best-effort): %v", n.ID, err))
			}
			pruned++
		}
		detail := fmt.Sprintf("pruned %d node(s) below %.0f%% free disk, skipped %d", pruned, minFreePct, skipped)
		if len(errs) > 0 {
			detail += "; details: " + strings.Join(errs, "; ")
		}
		return detail, nil
	}
}

func diskPressureCfgFromDB(ctx context.Context, store db.Store) float64 {
	const defaultMinFreePct = 10.0
	row, err := store.GetAutomation(ctx, "prune_disk_pressure")
	if err != nil {
		return defaultMinFreePct
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(row.ConfigJSON), &m); err != nil {
		return defaultMinFreePct
	}
	if v, ok := m["min_free_pct"].(float64); ok && v > 0 {
		return v
	}
	return defaultMinFreePct
}

// queryDiskFreePct runs `df -P /` over SSH and returns the free percentage.
// Returns an error when SSH is unavailable or parsing fails.
func queryDiskFreePct(ctx context.Context, filesSvc *fs.Service, nodeID string) (float64, error) {
	out, err := filesSvc.Exec(ctx, nodeID, "df", "-P", "/")
	if err != nil {
		return 0, err
	}
	// df -P output (POSIX): header + one data line per filesystem.
	// Filesystem  1024-blocks  Used  Available  Capacity%  Mounted
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// fields[4] is "Use%" like "42%"; we want 100-42=58% free.
		usedStr := strings.TrimSuffix(fields[4], "%")
		var usedPct float64
		if _, err := fmt.Sscan(usedStr, &usedPct); err != nil {
			continue
		}
		return 100.0 - usedPct, nil
	}
	return 0, fmt.Errorf("prune_disk_pressure: could not parse df output")
}

// verifyBackupHandler performs a restore-drill on the newest volume backup per
// node. Notifies via webhook on failure. Skips nodes with no completed backups.
func verifyBackupHandler(store db.Store, backupSvc *backup.Service, notify func(ctx context.Context, trigger, title, text string)) Handler {
	return func(ctx context.Context) (string, error) {
		if backupSvc == nil {
			return "skipped: backup service unavailable", nil
		}
		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		passed, failed, skipped := 0, 0, 0
		var errs []string
		for _, n := range nodes {
			res, err := backupSvc.VerifyLatest(ctx, n.ID)
			if err != nil {
				errs = append(errs, fmt.Sprintf("node %s: infra error: %v", n.ID, err))
				failed++
				continue
			}
			if res.BackupID == "" {
				skipped++
				continue
			}
			if res.Passed {
				passed++
			} else {
				failed++
				msg := fmt.Sprintf("node %s: verify failed: %s (archive %s)", n.ID, res.Error, res.ArchivePath)
				errs = append(errs, msg)
				if notify != nil {
					notify(ctx, "backup.verify_failed", "Backup verification failed", msg)
				}
			}
		}
		detail := fmt.Sprintf("passed %d, failed %d, skipped %d (no backup)", passed, failed, skipped)
		if len(errs) > 0 {
			detail += "; details: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d backup verification failure(s)", failed)
		}
		return detail, nil
	}
}

// capacityWarnHandler checks capacity projections for all nodes and notifies
// when any container is projected to exhaust a resource within horizon_days.
func capacityWarnHandler(store db.Store, forecastSvc *forecast.Service, notify func(ctx context.Context, trigger, title, text string)) Handler {
	return func(ctx context.Context) (string, error) {
		if forecastSvc == nil {
			return "skipped: forecast service unavailable", nil
		}
		horizonDays := capacityHorizonFromDB(ctx, store)
		horizonSecs := horizonDays * 24 * 3600

		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		warned, checked := 0, 0
		for _, n := range nodes {
			projections, err := forecastSvc.ForNode(ctx, n.ID)
			if err != nil {
				continue
			}
			for cID, projs := range projections {
				for _, p := range projs {
					if p.EtaSeconds < 0 || p.EtaSeconds > float64(horizonSecs) {
						continue
					}
					checked++
					msg := fmt.Sprintf("node %s container %s: %s projected to reach threshold in %.0f hours (slope %.2e/s)",
						n.ID, cID, p.Metric, p.EtaSeconds/3600, p.Slope)
					if notify != nil {
						notify(ctx, "capacity.warn", "Capacity warning", msg)
					}
					warned++
				}
			}
		}
		return fmt.Sprintf("checked %d projections, warned on %d approaching threshold within %d days", checked, warned, horizonDays), nil
	}
}

func capacityHorizonFromDB(ctx context.Context, store db.Store) int {
	const defaultHorizon = 7
	row, err := store.GetAutomation(ctx, "capacity_warn")
	if err != nil {
		return defaultHorizon
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(row.ConfigJSON), &m); err != nil {
		return defaultHorizon
	}
	if v, ok := m["horizon_days"].(float64); ok && v > 0 {
		return int(v)
	}
	return defaultHorizon
}

// allowedProjectsFromDB reads the "projects" array from a key's stored config.
func allowedProjectsFromDB(ctx context.Context, store db.Store, key string) map[string]bool {
	row, err := store.GetAutomation(ctx, key)
	if err != nil {
		return nil
	}
	var cfg struct {
		Projects []string `json:"projects"`
	}
	if err := json.Unmarshal([]byte(row.ConfigJSON), &cfg); err != nil {
		return nil
	}
	m := make(map[string]bool, len(cfg.Projects))
	for _, p := range cfg.Projects {
		if p != "" {
			m[p] = true
		}
	}
	return m
}
