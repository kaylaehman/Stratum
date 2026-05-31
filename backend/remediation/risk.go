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

// ci compiles a case-insensitive pattern. All command classification is
// case-insensitive so that e.g. `RM -RF` cannot evade the denylist.
func ci(pat string) *regexp.Regexp { return regexp.MustCompile(`(?i)` + pat) }

// shellMetacharacters matches command separators / substitution that allow an
// approved-looking command to chain into an arbitrary one (`; rm -rf /`,
// `... | sh`, "$(curl evil)", backtick substitution). We cannot statically
// reason about what such a command does, so it fails safe to RiskDestructive.
var shellMetacharacters = regexp.MustCompile("[;&|`\\n]|\\$\\(")

// sensitivePath matches references to system paths where any mutation is
// high-blast-radius. Combined with a mutating verb this escalates to
// RiskDestructive (see classifyOne).
var sensitivePath = ci(`(^|\s)/(etc|boot|usr|bin|sbin|lib|lib64|dev|sys|proc|root|var/lib|var/run|run)(/|\s|$)|\s/(\s|$)`)

// mutatingVerb matches commands that write/alter state (as opposed to read-only
// inspection). Used only to decide whether a sensitivePath reference is risky.
var mutatingVerb = ci(`\b(rm|mv|cp|ln|chmod|chown|chattr|setfacl|tee|truncate|install|mkdir|rmdir|sed\s+-i|dd|mkfs|mount|umount|systemctl|service)\b|>>?`)

// destructivePatterns are compiled once: a command matching any of these is
// classified as RiskDestructive regardless of other signals.
var destructivePatterns = []*regexp.Regexp{
	// Destructive filesystem ops (long- and short-flag forms)
	ci(`\brm\s+(-[a-z]*r|--recursive|--force.*-[a-z]*f)`), // rm -rf, rm -r, rm --recursive
	ci(`\brm\s+-[a-z]*f`),                                 // rm -f / -fr
	ci(`\bchmod\s+(-[a-z]*R|--recursive)`),                // chmod -R / --recursive
	ci(`\bchown\s+(-[a-z]*R|--recursive)`),                // chown -R / --recursive
	ci(`\bfind\b.*\s(-delete|-exec)\b`),                   // find ... -delete / -exec
	ci(`\bdd\b`),                                          // dd (disk write)
	ci(`\bmkfs`),                                          // mkfs / mkfs.ext4 (format)
	ci(`\bfdisk\b`), ci(`\bparted\b`), ci(`\bwipefs\b`),
	ci(`\bshred\b`), ci(`\btruncate\b`),
	ci(`>\s*/dev/`),                  // redirect into a device
	ci(`\bmv\b.*\s/dev/`),            // mv onto a device path
	ci(`\bshutdown\b`), ci(`\breboot\b`), ci(`\bpoweroff\b`), ci(`\bhalt\b`),
	ci(`\binit\s+[06]\b`),                                       // init 0 / init 6
	ci(`\bsystemctl\s+(isolate|halt|poweroff|reboot|kexec)\b`), // power-state systemd verbs
	ci(`\bkillall\b`),
}

// highRiskPatterns: matched when no destructive pattern fires.
var highRiskPatterns = []*regexp.Regexp{
	ci(`\bsudo\b`),    // privilege escalation
	ci(`\bchmod\b`),   // any chmod
	ci(`\bchown\b`),   // any chown
	ci(`\bsetfacl\b`), // ACL modification
	ci(`\bchattr\b`),
	ci(`\bkill\b`), // process kill
	ci(`\biptables\b`), ci(`\bnft\b`), ci(`\bufw\b`),
	ci(`\bsystemctl\s+(stop|disable|mask)\b`),
	ci(`\bservice\s+\S+\s+(stop|restart)\b`),
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
	// Comment-only or empty commands are inert. Checked first so a comment
	// body containing punctuation can't trip the metacharacter rule below.
	if cmd == "" || strings.HasPrefix(cmd, "#") {
		return RiskLow
	}
	// Fail-safe: a command we cannot statically analyze (it chains or
	// substitutes other commands) is treated as destructive. This is the
	// primary defense against denylist evasion like `echo ok; rm -rf /`.
	if shellMetacharacters.MatchString(cmd) {
		return RiskDestructive
	}
	for _, p := range destructivePatterns {
		if p.MatchString(cmd) {
			return RiskDestructive
		}
	}
	// A mutating verb aimed at a system path is destructive even if the verb
	// itself is otherwise only high-risk (e.g. `chmod 600 /etc/shadow`).
	if mutatingVerb.MatchString(cmd) && sensitivePath.MatchString(cmd) {
		return RiskDestructive
	}
	for _, p := range highRiskPatterns {
		if p.MatchString(cmd) {
			return RiskHigh
		}
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
