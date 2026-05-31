// Package automation is the automations engine (Wave 6): 8 code-defined
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
		Key:                    "scheduled_cve_scan",
		Label:                  "Scheduled CVE scan",
		Description:            "Run a full CVE scan across all running container images on a schedule.",
		Category:               CategorySecurity,
		DefaultIntervalSeconds: 86400,
		DefaultConfig:          map[string]any{},
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
