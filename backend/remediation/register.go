package remediation

import "github.com/KAE-Labs/stratum/backend/webhooks"

func init() {
	webhooks.Register(webhooks.TriggerDef{
		Key:         "remediation.executed",
		Label:       "Remediation executed",
		Description: "Fires when an approved remediation proposal is executed on a host.",
	})
	webhooks.Register(webhooks.TriggerDef{
		Key:         "remediation.failed",
		Label:       "Remediation failed",
		Description: "Fires when a remediation proposal execution fails.",
	})
}
