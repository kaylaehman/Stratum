package automation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kaylaehman/stratum/backend/backup"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/cve"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/recreate"
	"github.com/kaylaehman/stratum/backend/remediation"
	"github.com/kaylaehman/stratum/backend/security"
	"github.com/kaylaehman/stratum/backend/volumes"
)

// Deps holds all service references required to build the 8 handlers.
// All fields that are nil cause the relevant handler to return "skipped".
type Deps struct {
	Store     db.Store
	Conn      *nodeconn.Manager
	Security  *security.Scanner
	CVE       *cve.Service
	Volumes   *volumes.Service
	Recreate  *recreate.Service
	Backups   *backup.Service
	Remediation *remediation.Service
}

// BuildHandlers constructs the full handler map. Each handler closes over the
// services it needs and reads its live config from the DB on each call.
func BuildHandlers(store db.Store, deps Deps) map[string]Handler {
	return map[string]Handler{
		"restart_unhealthy":    restartUnhealthyHandler(store, deps.Conn),
		"auto_remediate_low":   autoRemediateLowHandler(store, deps.Remediation),
		"auto_pull_updates":    autoPullUpdatesHandler(store, deps.Conn),
		"auto_update_containers": autoUpdateContainersHandler(store, deps.Conn, deps.Recreate),
		"scheduled_cve_scan":   scheduledCVEScanHandler(store, deps.CVE),
		"security_alerts":      securityAlertsHandler(store, deps.Security, deps.CVE),
		"prune_unused_volumes": pruneUnusedVolumesHandler(deps.Volumes),
		"scheduled_backups":    scheduledBackupsHandler(store, deps.Backups),
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
		updated, errs := 0, []string{}
		for _, n := range containers {
			caps, _ := capabilities.Parse([]byte(n.CapabilitiesJSON))
			if !caps.Docker {
				continue
			}
			ctrs, err := store.ListContainersByNode(ctx, n.ID)
			if err != nil {
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
		if len(errs) > 0 {
			detail += "; errors: " + strings.Join(errs, "; ")
			return detail, fmt.Errorf("%d update error(s)", len(errs))
		}
		return detail, nil
	}
}

// scheduledCVEScanHandler runs a bulk CVE scan over all running containers.
func scheduledCVEScanHandler(store db.Store, cveSvc *cve.Service) Handler {
	return func(ctx context.Context) (string, error) {
		if cveSvc == nil || !cveSvc.Available() {
			return "skipped: CVE scanner (trivy/grype) not available", nil
		}
		nodes, err := store.ListNodes(ctx)
		if err != nil {
			return "", fmt.Errorf("list nodes: %w", err)
		}
		var toScan []db.Container
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
