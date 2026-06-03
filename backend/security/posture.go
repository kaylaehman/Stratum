// Package-level scoring reference (posture.go):
//
// # Security Posture Score — scoring inputs, weights, and grade boundaries
//
// Score() computes a 0-100 score and an A-F grade for one node. Each signal
// contributes a weighted penalty. When a data source is unavailable (-1), its
// weight is excluded from the pool so the score is never artificially deflated.
//
// ## Factor table
//
//   Factor               Max weight  Penalty per unit  Cap
//   ─────────────────────────────────────────────────────────
//   CVE (critical)       35 pts      15 per CVE        35
//   CVE (high)           35 pts      5 per CVE         35 (shared with critical)
//   Privileged flags     30 pts      10 per container  30
//   Exposed ports        15 pts      3 per port        15
//   Stale SSH keys       10 pts      5 per key         10
//   Pending updates      10 pts      2 per container   10
//   ─────────────────────────────────────────────────────────
//   Total                100 pts
//
// Scaling: scaledPenalty = (totalPenalty × 100) / totalAvailableWeight; capped at 100.
// Final score = 100 − scaledPenalty.
//
// ## Grade thresholds
//
//   A  90–100   B  75–89   C  60–74   D  40–59   F  0–39
//
// ## Remediation severity map
//
//   Metric    Severity
//   cve       critical (CriticalCVEs > 0) / high (HighCVEs > 0 only)
//   flags     critical
//   ports     high
//   sshkeys   medium
//   updates   low
//
// See posture_test.go for table-driven tests that lock every boundary.

package security

// PostureGrade is an A-F letter grade.
type PostureGrade string

const (
	GradeA PostureGrade = "A"
	GradeB PostureGrade = "B"
	GradeC PostureGrade = "C"
	GradeD PostureGrade = "D"
	GradeF PostureGrade = "F"
)

// RemediationSeverity classifies urgency of a remediation item.
type RemediationSeverity string

const (
	SeverityCritical RemediationSeverity = "critical"
	SeverityHigh     RemediationSeverity = "high"
	SeverityMedium   RemediationSeverity = "medium"
	SeverityLow      RemediationSeverity = "low"
)

// RemediationItem is one ranked action item the operator should address.
type RemediationItem struct {
	Title    string              `json:"title"`
	Severity RemediationSeverity `json:"severity"`
	Metric   string              `json:"metric"`
	Action   string              `json:"action"`
}

// PostureInputs carries all signal fed to Score. Fields set to -1 indicate
// "data unavailable — factor omitted from score".
type PostureInputs struct {
	// CVE counts across all containers on the node. -1 = scanner unavailable.
	CriticalCVEs int
	HighCVEs     int

	// PrivilegedCount is the count of unacknowledged privileged/dangerous
	// containers. -1 = docker unavailable.
	PrivilegedCount int

	// ExposedAllIfaceCount is the count of ports bound to 0.0.0.0/::.
	// -1 = docker unavailable.
	ExposedAllIfaceCount int

	// StaleSSHKeyCount is the count of SSH keys unused for 90+ days.
	// -1 = audit unavailable (SSH-only without agent, or key age unknown).
	StaleSSHKeyCount int

	// UpdateAvailableCount is containers with an available image update.
	// -1 = registry check not yet run.
	UpdateAvailableCount int
}

// PostureResult is the full computed posture for one node.
type PostureResult struct {
	Score       int               `json:"score"` // 0-100
	Grade       PostureGrade      `json:"grade"`
	Remediation []RemediationItem `json:"remediation"`
	// DataSources describes which factors contributed (key = metric name, value =
	// whether that source had data).
	DataSources map[string]bool `json:"data_sources"`
}

// Score computes a 0-100 posture score from PostureInputs and returns the
// full PostureResult. Missing data sources are skipped gracefully — the
// weight pool is only drawn from available signals.
func Score(in PostureInputs) PostureResult {
	sources := map[string]bool{
		"cve":     in.CriticalCVEs >= 0,
		"flags":   in.PrivilegedCount >= 0,
		"ports":   in.ExposedAllIfaceCount >= 0,
		"sshkeys": in.StaleSSHKeyCount >= 0,
		"updates": in.UpdateAvailableCount >= 0,
	}

	type factor struct {
		available bool
		weight    int // out of 100 total weight pool
		penalty   int // deduction if finding present
		items     func() []RemediationItem
	}

	factors := []factor{
		{
			available: sources["cve"],
			weight:    35,
			penalty:   penaltyCVE(in.CriticalCVEs, in.HighCVEs),
			items:     func() []RemediationItem { return cveRemediation(in.CriticalCVEs, in.HighCVEs) },
		},
		{
			available: sources["flags"],
			weight:    30,
			penalty:   penaltyFlags(in.PrivilegedCount),
			items:     func() []RemediationItem { return flagsRemediation(in.PrivilegedCount) },
		},
		{
			available: sources["ports"],
			weight:    15,
			penalty:   penaltyPorts(in.ExposedAllIfaceCount),
			items:     func() []RemediationItem { return portsRemediation(in.ExposedAllIfaceCount) },
		},
		{
			available: sources["sshkeys"],
			weight:    10,
			penalty:   penaltySSHKeys(in.StaleSSHKeyCount),
			items:     func() []RemediationItem { return sshKeyRemediation(in.StaleSSHKeyCount) },
		},
		{
			available: sources["updates"],
			weight:    10,
			penalty:   penaltyUpdates(in.UpdateAvailableCount),
			items:     func() []RemediationItem { return updatesRemediation(in.UpdateAvailableCount) },
		},
	}

	totalWeight, totalPenalty := 0, 0
	var remediations []RemediationItem
	for _, f := range factors {
		if !f.available {
			continue
		}
		totalWeight += f.weight
		totalPenalty += f.penalty
		remediations = append(remediations, f.items()...)
	}

	score := 100
	if totalWeight > 0 && totalPenalty > 0 {
		// Scale penalty to 0-100 relative to actual available weight pool.
		scaledPenalty := (totalPenalty * 100) / totalWeight
		if scaledPenalty > 100 {
			scaledPenalty = 100
		}
		score = 100 - scaledPenalty
	}

	return PostureResult{
		Score:       score,
		Grade:       grade(score),
		Remediation: sortRemediation(remediations),
		DataSources: sources,
	}
}

func grade(score int) PostureGrade {
	switch {
	case score >= 90:
		return GradeA
	case score >= 75:
		return GradeB
	case score >= 60:
		return GradeC
	case score >= 40:
		return GradeD
	default:
		return GradeF
	}
}

// penaltyCVE returns a penalty out of 35 points.
func penaltyCVE(critical, high int) int {
	penalty := critical*15 + high*5
	if penalty > 35 {
		return 35
	}
	return penalty
}

// penaltyFlags returns a penalty out of 30 points.
func penaltyFlags(privileged int) int {
	if privileged <= 0 {
		return 0
	}
	penalty := privileged * 10
	if penalty > 30 {
		return 30
	}
	return penalty
}

// penaltyPorts returns a penalty out of 15 points.
func penaltyPorts(exposed int) int {
	if exposed <= 0 {
		return 0
	}
	penalty := exposed * 3
	if penalty > 15 {
		return 15
	}
	return penalty
}

// penaltySSHKeys returns a penalty out of 10 points.
func penaltySSHKeys(stale int) int {
	if stale <= 0 {
		return 0
	}
	penalty := stale * 5
	if penalty > 10 {
		return 10
	}
	return penalty
}

// penaltyUpdates returns a penalty out of 10 points.
func penaltyUpdates(available int) int {
	if available <= 0 {
		return 0
	}
	penalty := available * 2
	if penalty > 10 {
		return 10
	}
	return penalty
}

func cveRemediation(critical, high int) []RemediationItem {
	var items []RemediationItem
	if critical > 0 {
		items = append(items, RemediationItem{
			Title:    itoa(critical) + " critical CVE(s) in running images",
			Severity: SeverityCritical,
			Metric:   "cve",
			Action:   "Review CVE details at /cve and update affected images",
		})
	}
	if high > 0 {
		items = append(items, RemediationItem{
			Title:    itoa(high) + " high CVE(s) in running images",
			Severity: SeverityHigh,
			Metric:   "cve",
			Action:   "Review CVE details at /cve and update affected images",
		})
	}
	return items
}

func flagsRemediation(privileged int) []RemediationItem {
	if privileged <= 0 {
		return nil
	}
	return []RemediationItem{{
		Title:    itoa(privileged) + " container(s) with dangerous security flags",
		Severity: SeverityCritical,
		Metric:   "flags",
		Action:   "Review privileged containers at /security and acknowledge or remediate",
	}}
}

func portsRemediation(exposed int) []RemediationItem {
	if exposed <= 0 {
		return nil
	}
	return []RemediationItem{{
		Title:    itoa(exposed) + " port(s) bound to all interfaces (0.0.0.0)",
		Severity: SeverityHigh,
		Metric:   "ports",
		Action:   "Review port exposures at /security and bind to 127.0.0.1 where external access is not required",
	}}
}

func sshKeyRemediation(stale int) []RemediationItem {
	if stale <= 0 {
		return nil
	}
	return []RemediationItem{{
		Title:    itoa(stale) + " SSH key(s) unused for 90+ days",
		Severity: SeverityMedium,
		Metric:   "sshkeys",
		Action:   "Review SSH keys under node settings and remove unused keys",
	}}
}

func updatesRemediation(available int) []RemediationItem {
	if available <= 0 {
		return nil
	}
	return []RemediationItem{{
		Title:    itoa(available) + " container image(s) with available update",
		Severity: SeverityLow,
		Metric:   "updates",
		Action:   "Update containers at /updates",
	}}
}

// sortRemediation sorts by severity: critical > high > medium > low.
func sortRemediation(items []RemediationItem) []RemediationItem {
	order := map[RemediationSeverity]int{
		SeverityCritical: 0,
		SeverityHigh:     1,
		SeverityMedium:   2,
		SeverityLow:      3,
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && order[items[j].Severity] < order[items[j-1].Severity]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	return items
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
