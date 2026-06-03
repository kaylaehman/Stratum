// Package automation is the automations engine (Wave 6): 13 code-defined
// autonomous automations, independently configurable via DB overrides, with a
// background Run loop and a RunNow path for manual triggers.
//
// Pattern mirrors features (catalog + DB overrides) and the background-loop
// services (Run(ctx) started in main.go). All destructive automations default
// disabled and gate on the same checks as their manual counterpart.
package automation

// Category groups the 8 automations in the UI.
const (
	CategorySelfHeal   = "self_heal"
	CategoryUpdate     = "update"
	CategorySecurity   = "security"
	CategoryMaintenance = "maintenance"
)

// Entry is one catalog definition. DB overrides replace Enabled /
// IntervalSeconds / Config at runtime.
type Entry struct {
	Key                    string         `json:"key"`
	Label                  string         `json:"label"`
	Description            string         `json:"description"`
	Category               string         `json:"category"`
	DefaultIntervalSeconds int            `json:"default_interval_seconds"`
	DefaultConfig          map[string]any `json:"default_config"`
}

// catalog is the canonical, code-defined set. Keys are stable identifiers used
// in the DB and activity log.
var catalog = []Entry{
	{
		Key:                    "restart_unhealthy",
		Label:                  "Restart unhealthy containers",
		Description:            "Automatically restart containers whose health-check is failing or whose status is 'exited'.",
		Category:               CategorySelfHeal,
		DefaultIntervalSeconds: 300, // 5 min
		DefaultConfig:          map[string]any{},
	},
	{
		Key:                    "auto_remediate_low",
		Label:                  "Auto-run low-risk remediation",
		Description:            "Automatically execute remediation proposals whose risk is classified as 'low'. High/medium/destructive proposals are never auto-executed.",
		Category:               CategorySelfHeal,
		DefaultIntervalSeconds: 600,
		DefaultConfig:          map[string]any{},
	},
	{
		Key:                    "auto_pull_updates",
		Label:                  "Auto-pull latest images",
		Description:            "Pull the latest image for every running container. Does NOT recreate containers — applies on next manual restart.",
		Category:               CategoryUpdate,
		DefaultIntervalSeconds: 21600, // 6 h
		DefaultConfig:          map[string]any{},
	},
	{
		Key:   "auto_update_containers",
		Label: "Auto-update containers",
		Description: "Pull the latest image and recreate containers in the configured project allowlist. " +
			"Only projects listed in config.projects are touched. Disabled by default.",
		Category:               CategoryUpdate,
		DefaultIntervalSeconds: 86400, // 24 h
		DefaultConfig:          map[string]any{"projects": []string{}},
	},
	{
		Key:         "scheduled_cve_scan",
		Label:       "Scheduled CVE scan",
		Description: "Run a CVE scan on a schedule. config.targets is a list of {node_id, container_id} objects — empty means scan all running containers.",
		Category:    CategorySecurity,
		DefaultIntervalSeconds: 86400,
		DefaultConfig: map[string]any{"targets": []any{}},
	},
	{
		Key:   "security_alerts",
		Label: "Security change alerts",
		Description: "Send webhook notifications when a new Critical CVE, newly exposed port, " +
			"newly privileged container, or new SSH key is detected.",
		Category:               CategorySecurity,
		DefaultIntervalSeconds: 3600,
		DefaultConfig:          map[string]any{},
	},
	{
		Key:                    "prune_unused_volumes",
		Label:                  "Prune unused volumes",
		Description:            "Remove Docker volumes that are not attached to any container. Disabled by default — destructive.",
		Category:               CategoryMaintenance,
		DefaultIntervalSeconds: 604800, // 7 days
		DefaultConfig:          map[string]any{},
	},
	{
		Key:                    "scheduled_backups",
		Label:                  "Scheduled backups",
		Description:            "Trigger a volume backup on the configured node + destination directory.",
		Category:               CategoryMaintenance,
		DefaultIntervalSeconds: 86400,
		DefaultConfig:          map[string]any{"node_id": "", "volume": "", "dest_dir": ""},
	},
	{
		Key:   "restart_on_resource_spike",
		Label: "Restart on resource spike",
		Description: "Restart a container that exceeds CPU or memory thresholds for a sustained window. " +
			"Config: cpu_pct (default 90), mem_pct (default 90), window_minutes (default 15).",
		Category:               CategorySelfHeal,
		DefaultIntervalSeconds: 300,
		DefaultConfig:          map[string]any{"cpu_pct": float64(90), "mem_pct": float64(90), "window_minutes": float64(15)},
	},
	{
		Key:   "fix_bind_mount_perms",
		Label: "Fix bind-mount permission issues",
		Description: "For containers with a detected bind-mount UID/GID mismatch, apply the diagnostic's suggested " +
			"chmod/chown. Defaults to dry_run=true (logs the fix it would apply without executing).",
		Category:               CategorySelfHeal,
		DefaultIntervalSeconds: 3600,
		DefaultConfig:          map[string]any{"dry_run": true},
	},
	{
		Key:   "run_runbooks_on_alert",
		Label: "Run runbooks on alerts",
		Description: "When a recent incident or activity alert matches a runbook's trigger conditions, " +
			"generate a remediation proposal from that runbook. Config: min_severity (default critical).",
		Category:               CategorySelfHeal,
		DefaultIntervalSeconds: 600,
		DefaultConfig:          map[string]any{"min_severity": "critical"},
	},
	{
		Key:   "patch_critical_cves",
		Label: "Patch containers with critical CVEs",
		Description: "Scan running images; for a container whose image has CRITICAL CVEs with an available fixed " +
			"version, update it to the fixed image via pull+recreate. Defaults to dry_run=true. " +
			"Config: allowlist (compose projects), dry_run.",
		Category:               CategorySecurity,
		DefaultIntervalSeconds: 86400,
		DefaultConfig:          map[string]any{"allowlist": []string{}, "dry_run": true},
	},
	{
		Key:   "prune_disk_pressure",
		Label: "Prune disk pressure",
		Description: "When host free disk falls below min_free_pct (default 10%), prune dangling images and " +
			"Docker builder cache via SSH. Only acts when below threshold.",
		Category:               CategoryMaintenance,
		DefaultIntervalSeconds: 3600,
		DefaultConfig:          map[string]any{"min_free_pct": float64(10)},
	},
	{
		Key:         "verify_backup",
		Label:       "Verify latest backup",
		Description: "Perform a restore-drill on the newest completed volume backup for each node. Notifies via webhook on failure.",
		Category:    CategoryMaintenance,
		DefaultIntervalSeconds: 86400, // daily
		DefaultConfig:          map[string]any{},
	},
	{
		Key:   "capacity_warn",
		Label: "Capacity warning",
		Description: "Check capacity projections for all nodes and send a webhook alert when any container is projected to " +
			"exhaust CPU, memory, or disk writes within horizon_days (default 7).",
		Category:               CategoryMaintenance,
		DefaultIntervalSeconds: 3600, // hourly
		DefaultConfig:          map[string]any{"horizon_days": float64(7)},
	},
}

// Catalog returns the full ordered entry set (read-only).
func Catalog() []Entry { return catalog }

// catalogByKey maps key → Entry for O(1) lookup.
var catalogByKey = func() map[string]Entry {
	m := make(map[string]Entry, len(catalog))
	for _, e := range catalog {
		m[e.Key] = e
	}
	return m
}()

// CatalogEntry returns the catalog entry for key and whether it exists.
func CatalogEntry(key string) (Entry, bool) {
	e, ok := catalogByKey[key]
	return e, ok
}
