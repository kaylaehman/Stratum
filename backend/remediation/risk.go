// Package remediation implements the agentic-remediation workflow:
// generate a structured proposal → explicit approval (with step-up for
// destructive risk) → execute via SSH → capture output → audit everything.
//
// SAFETY GUARANTEES (enforced here and in the API layer):
//   - Auto-execution is impossible: generate and execute are distinct operations
//     separated by an explicit approval stored in the DB.
//   - Destructive proposals additionally require TOTP step-up auth before
//     the execute endpoint is reached (enforced by the handler via requireStepUp).
//   - All state transitions are written to the activity log.
//   - Secret material is never included in proposal commands or log detail.
package remediation

import (
	"regexp"
	"strings"
)

// Risk levels, ordered least to most dangerous.
const (
	RiskLow         = "low"
	RiskMedium      = "medium"
	RiskHigh        = "high"
	RiskDestructive = "destructive"
)

// Status values for a proposal's lifecycle.
const (
	StatusProposed = "proposed"
	StatusApproved = "approved"
	StatusRejected = "rejected"
	StatusExecuted = "executed"
	StatusFailed   = "failed"
)

// Source values indicating what generated the proposal.
const (
	SourceDiagnostic = "diagnostic"
	SourceRunbook    = "runbook"
	SourceAI         = "ai"
)

// destructivePatterns are compiled once: a command matching any of these is
// classified as RiskDestructive regardless of other signals.
var destructivePatterns = []*regexp.Regexp{
	// Destructive filesystem ops
	regexp.MustCompile(`\brm\s+-[a-z]*r`),        // rm -rf, rm -r
	regexp.MustCompile(`\bchmod\s+-R\b`),          // chmod -R (recursive)
	regexp.MustCompile(`\bchown\s+-R\b`),          // chown -R (recursive)
	regexp.MustCompile(`\bdd\b`),                  // dd (disk write)
	regexp.MustCompile(`\bmkfs\b`),                // mkfs (format)
	regexp.MustCompile(`\bfdisk\b`),               // fdisk
	regexp.MustCompile(`\bparted\b`),              // parted
	regexp.MustCompile(`\bshred\b`),               // shred
	regexp.MustCompile(`\btruncate\b`),            // truncate
	regexp.MustCompile(`>\s*/dev/`),               // redirect to /dev/
	regexp.MustCompile(`\bshutdown\b`),            // shutdown
	regexp.MustCompile(`\breboot\b`),              // reboot
	regexp.MustCompile(`\bpoweroff\b`),            // poweroff
	regexp.MustCompile(`\binit\s+0\b`),            // init 0
	regexp.MustCompile(`\bsystemctl\s+.*halt\b`),  // systemctl halt
	regexp.MustCompile(`\bkillall\b`),             // killall
}

// highRiskPatterns: matched when no destructive pattern fires.
var highRiskPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bchmod\b`),   // any chmod
	regexp.MustCompile(`\bchown\b`),   // any chown
	regexp.MustCompile(`\bsetfacl\b`), // ACL modification
	regexp.MustCompile(`\bkill\b`),    // process kill
	regexp.MustCompile(`\biptables\b`),
	regexp.MustCompile(`\bufw\b`),
	regexp.MustCompile(`\bsystemctl\s+(stop|disable|mask)\b`),
}

// ClassifyRisk returns the highest risk level across all commands.
// This is authoritative: the API handler and tests verify that destructive
// commands are classified correctly and trigger step-up enforcement.
func ClassifyRisk(commands []string) string {
	worst := RiskLow
	for _, cmd := range commands {
		r := classifyOne(strings.TrimSpace(cmd))
		if riskRank(r) > riskRank(worst) {
			worst = r
		}
	}
	return worst
}

func classifyOne(cmd string) string {
	for _, p := range destructivePatterns {
		if p.MatchString(cmd) {
			return RiskDestructive
		}
	}
	for _, p := range highRiskPatterns {
		if p.MatchString(cmd) {
			return RiskHigh
		}
	}
	// Comment-only or empty commands
	if cmd == "" || strings.HasPrefix(cmd, "#") {
		return RiskLow
	}
	return RiskMedium
}

func riskRank(r string) int {
	switch r {
	case RiskLow:
		return 1
	case RiskMedium:
		return 2
	case RiskHigh:
		return 3
	case RiskDestructive:
		return 4
	default:
		return 0
	}
}

// RequiresStepUp returns true when the risk level demands TOTP re-auth before
// execution. Currently only RiskDestructive qualifies.
func RequiresStepUp(riskLevel string) bool {
	return riskLevel == RiskDestructive
}
