package uptime

import "github.com/kaylaehman/stratum/backend/webhooks"

// TriggerUptimeDown is the webhook trigger key fired on UP→DOWN transitions.
const TriggerUptimeDown = "uptime.down"

func init() {
	webhooks.Register(webhooks.TriggerDef{
		Key:         TriggerUptimeDown,
		Label:       "Uptime monitor down",
		Description: "Fires when a monitored endpoint transitions from up to down.",
		ConfigSchema: []webhooks.TriggerConfigField{
			{Key: "consecutive_failures", Label: "Consecutive failures before alert", Type: "number", Default: "1"},
		},
	})
}
