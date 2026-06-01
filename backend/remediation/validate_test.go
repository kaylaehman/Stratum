package remediation

import (
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
)

func TestValidateRunbook_EmptyStep(t *testing.T) {
	rb := db.Runbook{Name: "test", Steps: []string{"docker restart plex", ""}, RequiresApproval: false}
	res := ValidateRunbook(rb)
	if res.Valid {
		t.Error("want invalid due to empty step")
	}
	found := false
	for _, e := range res.Errors {
		if e.StepIndex == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected error for step index 1 (empty)")
	}
}

func TestValidateRunbook_DestructiveRequiresApproval(t *testing.T) {
	rb := db.Runbook{
		Name:             "wipe",
		Steps:            []string{"rm -rf /var/log/app"},
		RequiresApproval: false, // missing — should be an error
	}
	res := ValidateRunbook(rb)
	if res.Valid {
		t.Error("want invalid: destructive step without requires_approval")
	}
	if len(res.Errors) == 0 {
		t.Error("expected at least one error")
	}
	if res.StepRisks[0] != RiskDestructive {
		t.Errorf("step risk = %q; want %q", res.StepRisks[0], RiskDestructive)
	}
}

func TestValidateRunbook_DestructiveWithApproval_Valid(t *testing.T) {
	rb := db.Runbook{
		Name:             "wipe-approved",
		Steps:            []string{"rm -rf /var/log/app"},
		RequiresApproval: true,
	}
	res := ValidateRunbook(rb)
	if !res.Valid {
		t.Errorf("want valid; errors=%v", res.Errors)
	}
}

func TestValidateRunbook_HighRiskWarning(t *testing.T) {
	rb := db.Runbook{
		Name:             "chown",
		Steps:            []string{"chown 999 /data/config"},
		RequiresApproval: false,
	}
	res := ValidateRunbook(rb)
	// high-risk without approval is a WARNING, not an error
	if !res.Valid {
		t.Error("want valid (high-risk is a warning, not error)")
	}
	if len(res.Warnings) == 0 {
		t.Error("expected at least one warning for high-risk step")
	}
}

func TestValidateRunbook_LowRiskNoApproval_Valid(t *testing.T) {
	rb := db.Runbook{
		Name:             "restart",
		Steps:            []string{"docker restart plex", "systemctl status nginx"},
		RequiresApproval: false,
	}
	res := ValidateRunbook(rb)
	if !res.Valid {
		t.Errorf("want valid; errors=%v", res.Errors)
	}
	if len(res.Errors) != 0 {
		t.Errorf("want no errors; got %v", res.Errors)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("want no warnings; got %v", res.Warnings)
	}
}

func TestValidateRunbook_MixedRisks_HighestWins(t *testing.T) {
	rb := db.Runbook{
		Name: "mixed",
		Steps: []string{
			"docker restart plex",   // low
			"chown 999 /data",       // high
			"rm -rf /tmp/junk",      // destructive
		},
		RequiresApproval: false,
	}
	res := ValidateRunbook(rb)
	if res.Valid {
		t.Error("want invalid: destructive step present without requires_approval")
	}
	// step 0: low → no issue
	// step 1: high → warning
	// step 2: destructive → error
	if res.StepRisks[0] != RiskLow {
		t.Errorf("step 0 risk = %q; want low", res.StepRisks[0])
	}
	if res.StepRisks[1] != RiskHigh {
		t.Errorf("step 1 risk = %q; want high", res.StepRisks[1])
	}
	if res.StepRisks[2] != RiskDestructive {
		t.Errorf("step 2 risk = %q; want destructive", res.StepRisks[2])
	}
}

func TestValidateRunbook_EmptyRunbook_Valid(t *testing.T) {
	rb := db.Runbook{Name: "empty", Steps: []string{}}
	res := ValidateRunbook(rb)
	if !res.Valid {
		t.Errorf("empty runbook should be valid; errors=%v", res.Errors)
	}
}
