package remediation

import (
	"fmt"
	"strings"

	"github.com/kaylaehman/stratum/backend/db"
)

// LintError is a single lint violation discovered while validating a runbook.
type LintError struct {
	StepIndex int    `json:"step_index"`
	Step      string `json:"step"`
	Risk      string `json:"risk"`
	Message   string `json:"message"`
}

// ValidationResult is the outcome of ValidateRunbook.
type ValidationResult struct {
	Valid    bool        `json:"valid"`
	Errors   []LintError `json:"errors"`
	Warnings []LintError `json:"warnings"`
	// StepRisks holds the classified risk per step (index-aligned with rb.Steps).
	StepRisks []string `json:"step_risks"`
}

// ValidateRunbook lints a runbook's steps:
//   - Each step is classified via ClassifyRisk.
//   - A destructive step without rb.RequiresApproval set is a LINT ERROR
//     (a runbook that contains destructive commands must be flagged as
//     requiring approval before the API accepts it).
//   - A high-risk step without rb.RequiresApproval set is a WARNING.
//   - Empty steps are flagged as errors.
func ValidateRunbook(rb db.Runbook) ValidationResult {
	res := ValidationResult{
		Valid:     true,
		Errors:    []LintError{},
		Warnings:  []LintError{},
		StepRisks: make([]string, len(rb.Steps)),
	}

	for i, step := range rb.Steps {
		step = strings.TrimSpace(step)

		if step == "" {
			res.Valid = false
			res.Errors = append(res.Errors, LintError{
				StepIndex: i,
				Step:      step,
				Risk:      "",
				Message:   fmt.Sprintf("step %d is empty", i+1),
			})
			res.StepRisks[i] = ""
			continue
		}

		risk := ClassifyRisk([]string{step})
		res.StepRisks[i] = risk

		switch risk {
		case RiskDestructive:
			if !rb.RequiresApproval {
				res.Valid = false
				res.Errors = append(res.Errors, LintError{
					StepIndex: i,
					Step:      step,
					Risk:      risk,
					Message: fmt.Sprintf(
						"step %d contains a destructive command (%q) but the runbook does not require approval — set requires_approval: true",
						i+1, truncate(step, 60),
					),
				})
			}
		case RiskHigh:
			if !rb.RequiresApproval {
				res.Warnings = append(res.Warnings, LintError{
					StepIndex: i,
					Step:      step,
					Risk:      risk,
					Message: fmt.Sprintf(
						"step %d is high-risk (%q); consider setting requires_approval: true",
						i+1, truncate(step, 60),
					),
				})
			}
		}
	}

	return res
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
