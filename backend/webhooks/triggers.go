package webhooks

// Trigger key constants. Event sources import these to call Notify.
// Keys are the stable wire values stored in WebhookConfig.Triggers.
const (
	// Existing triggers (unchanged keys — no regression).
	TriggerPortNew         = "port.new"
	TriggerContainerCrash  = "container.crash"
	TriggerCVECritical     = "cve.critical"
	TriggerSSHKeyAdded     = "sshkey.added"
	TriggerFileChange      = "file.change"
	TriggerAgentDisconnect = "agent.disconnect"
	TriggerCPUThreshold    = "cpu.threshold"

	// New triggers added by this feature.
	TriggerContainerOOM        = "container.oom"
	TriggerContainerUnhealthy  = "container.unhealthy"
	TriggerVolumeThreshold     = "volume.threshold"
	TriggerImageUpdateAvail    = "image.update_available"
	TriggerPostureGradeDrop    = "posture.grade_drop"
	TriggerBackupFailed        = "backup.failed"
)

// AllTriggers is the legacy flat slice kept for any callers that imported it
// before the registry was introduced. It is rebuilt from the registry on init.
// New code should call AllTriggerKeys() or Registered() instead.
var AllTriggers []string

func init() {
	// ---- Existing triggers ----

	Register(TriggerDef{
		Key:         TriggerPortNew,
		Label:       "New exposed port",
		Description: "Fires when a container publishes a port that was not seen in the previous scan.",
	})
	Register(TriggerDef{
		Key:         TriggerContainerCrash,
		Label:       "Container crash",
		Description: "Fires when a container exits with a non-zero exit code.",
	})
	Register(TriggerDef{
		Key:         TriggerCVECritical,
		Label:       "Critical CVE found",
		Description: "Fires when a Trivy/Grype scan finds a new Critical-severity CVE in a running image.",
	})
	Register(TriggerDef{
		Key:                TriggerSSHKeyAdded,
		Label:              "SSH key added",
		Description:        "Fires when a new entry is detected in any authorized_keys file on a monitored host.",
		RequiresCapability: "agent",
	})
	Register(TriggerDef{
		Key:                TriggerFileChange,
		Label:              "File change detected",
		Description:        "Fires when a file in a watched path is created, modified, deleted, or renamed.",
		RequiresCapability: "agent",
	})
	Register(TriggerDef{
		Key:         TriggerAgentDisconnect,
		Label:       "Agent disconnected",
		Description: "Fires when the Stratum agent on a node stops responding.",
	})
	Register(TriggerDef{
		Key:         TriggerCPUThreshold,
		Label:       "CPU threshold exceeded",
		Description: "Fires when a container's CPU usage stays above the configured threshold for the configured duration.",
		ConfigSchema: []TriggerConfigField{
			{Key: "threshold_pct", Label: "CPU % threshold", Type: "number", Default: "80"},
			{Key: "duration_min", Label: "Duration (minutes)", Type: "number", Default: "15"},
		},
	})

	// ---- New triggers ----

	Register(TriggerDef{
		Key:         TriggerContainerOOM,
		Label:       "Container OOM-killed or unhealthy",
		Description: "Fires when a container is killed by the kernel OOM killer or its Docker healthcheck transitions to unhealthy.",
	})
	Register(TriggerDef{
		// Kept as a separate constant so callers can distinguish health-only
		// events; evaluated together with TriggerContainerOOM by the poller.
		Key:         TriggerContainerUnhealthy,
		Label:       "Container healthcheck failed",
		Description: "Fires when a container's Docker healthcheck status transitions to unhealthy.",
	})
	Register(TriggerDef{
		Key:         TriggerVolumeThreshold,
		Label:       "Volume size threshold crossed",
		Description: "Fires when a Docker volume grows past its configured size limit.",
		ConfigSchema: []TriggerConfigField{
			{Key: "threshold_mb", Label: "Size threshold (MB)", Type: "number", Default: "1024"},
		},
	})
	Register(TriggerDef{
		Key:         TriggerImageUpdateAvail,
		Label:       "Image update available",
		Description: "Fires when the Update Assistant finds a newer digest for a running container's image.",
	})
	Register(TriggerDef{
		// TODO(feat/posture-score): wire evaluator once posture-score data lands.
		// The trigger is registered and the UI renders it; evaluation is a no-op
		// until the posture-score branch merges and calls Notify with this key.
		Key:         TriggerPostureGradeDrop,
		Label:       "Node posture grade dropped",
		Description: "Fires when a node's security posture grade drops below the configured letter grade.",
		ConfigSchema: []TriggerConfigField{
			{
				Key:     "min_grade",
				Label:   "Minimum acceptable grade",
				Type:    "select",
				Default: "C",
				Options: []string{"A", "B", "C", "D"},
			},
		},
	})
	Register(TriggerDef{
		Key:         TriggerBackupFailed,
		Label:       "Backup job failed",
		Description: "Fires when a volume backup or Proxmox guest backup finishes with an error status.",
	})

	// Rebuild the legacy AllTriggers slice from the registry so existing callers
	// (e.g. tests that range over it) still work without modification.
	AllTriggers = AllTriggerKeys()
}
