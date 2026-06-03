package security

import "testing"

func TestGrade(t *testing.T) {
	cases := []struct {
		score int
		want  PostureGrade
	}{
		{100, GradeA},
		{90, GradeA},
		{89, GradeB},
		{75, GradeB},
		{74, GradeC},
		{60, GradeC},
		{59, GradeD},
		{40, GradeD},
		{39, GradeF},
		{0, GradeF},
	}
	for _, tc := range cases {
		if got := grade(tc.score); got != tc.want {
			t.Errorf("grade(%d) = %q, want %q", tc.score, got, tc.want)
		}
	}
}

func TestScoreClean(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: 0, HighCVEs: 0,
		PrivilegedCount: 0, ExposedAllIfaceCount: 0,
		StaleSSHKeyCount: 0, UpdateAvailableCount: 0,
	}
	got := Score(in)
	if got.Score != 100 {
		t.Errorf("all-clean inputs: score = %d, want 100", got.Score)
	}
	if got.Grade != GradeA {
		t.Errorf("all-clean inputs: grade = %q, want A", got.Grade)
	}
	if len(got.Remediation) != 0 {
		t.Errorf("all-clean inputs: remediation not empty: %v", got.Remediation)
	}
}

func TestScoreAllUnavailable(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: -1, HighCVEs: -1,
		PrivilegedCount: -1, ExposedAllIfaceCount: -1,
		StaleSSHKeyCount: -1, UpdateAvailableCount: -1,
	}
	got := Score(in)
	if got.Score != 100 {
		t.Errorf("all-unavailable inputs: score = %d, want 100 (no data = no penalty)", got.Score)
	}
	for k, v := range got.DataSources {
		if v {
			t.Errorf("data_sources[%q] = true, want false when all unavailable", k)
		}
	}
}

func TestScoreCriticalCVEsDegrade(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: 1, HighCVEs: 0,
		PrivilegedCount: -1, ExposedAllIfaceCount: -1,
		StaleSSHKeyCount: -1, UpdateAvailableCount: -1,
	}
	got := Score(in)
	// Only cve factor (weight 35), penalty 15 → scaledPenalty = (15*100)/35 = 42
	// score = 100-42 = 58 → grade D
	if got.Score > 90 {
		t.Errorf("1 critical CVE should degrade score, got %d", got.Score)
	}
	if got.Grade == GradeA {
		t.Errorf("1 critical CVE should not be grade A, got %q", got.Grade)
	}
	if len(got.Remediation) == 0 {
		t.Error("expected remediation items for critical CVE")
	}
	if got.Remediation[0].Severity != SeverityCritical {
		t.Errorf("first remediation should be critical, got %q", got.Remediation[0].Severity)
	}
}

func TestScorePrivilegedDegrades(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: -1, HighCVEs: -1,
		PrivilegedCount: 3, ExposedAllIfaceCount: -1,
		StaleSSHKeyCount: -1, UpdateAvailableCount: -1,
	}
	got := Score(in)
	// Only flags factor (weight 30), penalty = min(30, 30) = 30 → scaledPenalty = 100 → score 0 → F
	if got.Score != 0 {
		t.Errorf("3 privileged containers: score = %d, want 0 (max penalty capped)", got.Score)
	}
	if got.Grade != GradeF {
		t.Errorf("3 privileged containers: grade = %q, want F", got.Grade)
	}
}

func TestScorePartialSources(t *testing.T) {
	// Only ports and updates available, both clean.
	in := PostureInputs{
		CriticalCVEs: -1, HighCVEs: -1,
		PrivilegedCount: -1, ExposedAllIfaceCount: 0,
		StaleSSHKeyCount: -1, UpdateAvailableCount: 0,
	}
	got := Score(in)
	if got.Score != 100 {
		t.Errorf("partial clean: score = %d, want 100", got.Score)
	}
	if !got.DataSources["ports"] || !got.DataSources["updates"] {
		t.Error("ports and updates should be marked available")
	}
	if got.DataSources["cve"] || got.DataSources["flags"] || got.DataSources["sshkeys"] {
		t.Error("cve/flags/sshkeys should not be marked available")
	}
}

func TestScoreRemediationOrdered(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: 1, HighCVEs: 2,
		PrivilegedCount: 1, ExposedAllIfaceCount: 2,
		StaleSSHKeyCount: 1, UpdateAvailableCount: 3,
	}
	got := Score(in)
	if len(got.Remediation) == 0 {
		t.Fatal("expected remediation items")
	}
	severityOrder := map[RemediationSeverity]int{
		SeverityCritical: 0, SeverityHigh: 1, SeverityMedium: 2, SeverityLow: 3,
	}
	for i := 1; i < len(got.Remediation); i++ {
		prev := severityOrder[got.Remediation[i-1].Severity]
		curr := severityOrder[got.Remediation[i].Severity]
		if curr < prev {
			t.Errorf("remediation not sorted at index %d: %q before %q",
				i, got.Remediation[i].Severity, got.Remediation[i-1].Severity)
		}
	}
}

func TestScoreCapsPenaltyCap(t *testing.T) {
	// Even with many critical CVEs, penalty should be capped to weight (35).
	in := PostureInputs{
		CriticalCVEs: 100, HighCVEs: 100,
		PrivilegedCount: -1, ExposedAllIfaceCount: -1,
		StaleSSHKeyCount: -1, UpdateAvailableCount: -1,
	}
	got := Score(in)
	// penalty capped at 35, scaled = 100, score = 0
	if got.Score != 0 {
		t.Errorf("extreme CVE count: score = %d, want 0", got.Score)
	}
}

func TestItoaBasic(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{100, "100"},
		{1234, "1234"},
	}
	for _, tc := range cases {
		if got := itoa(tc.in); got != tc.want {
			t.Errorf("itoa(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- Per-penalty boundary tests ---

// TestPenaltyCVEBoundaries verifies CVE penalties cap at 35 and each unit
// contributes correctly: critical×15, high×5.
func TestPenaltyCVEBoundaries(t *testing.T) {
	cases := []struct {
		critical, high, want int
	}{
		{0, 0, 0},
		{1, 0, 15},
		{0, 1, 5},
		{1, 1, 20},
		{2, 1, 35}, // 30+5=35 = cap
		{3, 0, 35}, // 45 → capped at 35
		{0, 8, 35}, // 40 → capped at 35
		{10, 10, 35},
	}
	for _, tc := range cases {
		if got := penaltyCVE(tc.critical, tc.high); got != tc.want {
			t.Errorf("penaltyCVE(%d,%d) = %d, want %d", tc.critical, tc.high, got, tc.want)
		}
	}
}

// TestPenaltyFlagsBoundaries verifies privileged-container penalty: 10 per
// container, capped at 30. Negative values treated as zero.
func TestPenaltyFlagsBoundaries(t *testing.T) {
	cases := []struct {
		privileged, want int
	}{
		{-5, 0}, {-1, 0}, {0, 0},
		{1, 10}, {2, 20}, {3, 30}, {4, 30}, {10, 30},
	}
	for _, tc := range cases {
		if got := penaltyFlags(tc.privileged); got != tc.want {
			t.Errorf("penaltyFlags(%d) = %d, want %d", tc.privileged, got, tc.want)
		}
	}
}

// TestPenaltyPortsBoundaries verifies exposed-port penalty: 3 per port, capped at 15.
func TestPenaltyPortsBoundaries(t *testing.T) {
	cases := []struct {
		exposed, want int
	}{
		{-1, 0}, {0, 0},
		{1, 3}, {4, 12}, {5, 15}, {6, 15}, {100, 15},
	}
	for _, tc := range cases {
		if got := penaltyPorts(tc.exposed); got != tc.want {
			t.Errorf("penaltyPorts(%d) = %d, want %d", tc.exposed, got, tc.want)
		}
	}
}

// TestPenaltySSHKeysBoundaries verifies stale-key penalty: 5 per key, capped at 10.
func TestPenaltySSHKeysBoundaries(t *testing.T) {
	cases := []struct {
		stale, want int
	}{
		{-1, 0}, {0, 0},
		{1, 5}, {2, 10}, {3, 10}, {10, 10},
	}
	for _, tc := range cases {
		if got := penaltySSHKeys(tc.stale); got != tc.want {
			t.Errorf("penaltySSHKeys(%d) = %d, want %d", tc.stale, got, tc.want)
		}
	}
}

// TestPenaltyUpdatesBoundaries verifies available-update penalty: 2 per container, capped at 10.
func TestPenaltyUpdatesBoundaries(t *testing.T) {
	cases := []struct {
		available, want int
	}{
		{-1, 0}, {0, 0},
		{1, 2}, {5, 10}, {6, 10}, {100, 10},
	}
	for _, tc := range cases {
		if got := penaltyUpdates(tc.available); got != tc.want {
			t.Errorf("penaltyUpdates(%d) = %d, want %d", tc.available, got, tc.want)
		}
	}
}

// --- Score scaling accuracy ---

// TestScoreScaledPenaltyMath verifies the weighted-penalty rescaling formula
// when only a subset of data sources is available.
//
// Only cve (weight=35) has data, penalty=15 (1 critical CVE).
// scaledPenalty = (15*100)/35 = 42; score = 100-42 = 58 → grade D.
func TestScoreScaledPenaltyMath(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: 1, HighCVEs: 0,
		PrivilegedCount: -1, ExposedAllIfaceCount: -1,
		StaleSSHKeyCount: -1, UpdateAvailableCount: -1,
	}
	got := Score(in)
	wantScore := 100 - (15*100)/35 // = 58
	if got.Score != wantScore {
		t.Errorf("1 critical CVE (cve-only): score = %d, want %d", got.Score, wantScore)
	}
	if got.Grade != GradeD {
		t.Errorf("grade for score %d: got %q, want D", got.Score, got.Grade)
	}
}

// TestScoreDataSourcesMap verifies the DataSources map correctly reflects which
// inputs had data (-1 = unavailable).
func TestScoreDataSourcesMap(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: 0, HighCVEs: 0, // cve available
		PrivilegedCount:      -1, // flags unavailable
		ExposedAllIfaceCount: 2,  // ports available
		StaleSSHKeyCount:     -1, // sshkeys unavailable
		UpdateAvailableCount: 1,  // updates available
	}
	got := Score(in)
	for key, wantAvail := range map[string]bool{
		"cve": true, "flags": false, "ports": true, "sshkeys": false, "updates": true,
	} {
		if got.DataSources[key] != wantAvail {
			t.Errorf("DataSources[%q] = %v, want %v", key, got.DataSources[key], wantAvail)
		}
	}
}

// TestScoreRemediationMetricSeverities verifies that each metric produces
// remediation items with the expected severities when triggered.
// Note: the cve metric can produce two items (critical + high) when both
// CriticalCVEs and HighCVEs are non-zero.
func TestScoreRemediationMetricSeverities(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: 1, HighCVEs: 1,
		PrivilegedCount: 1, ExposedAllIfaceCount: 1,
		StaleSSHKeyCount: 1, UpdateAvailableCount: 1,
	}
	got := Score(in)

	// Collect ALL items per metric (cve can produce two: critical + high).
	byMetric := map[string][]RemediationItem{}
	for _, item := range got.Remediation {
		byMetric[item.Metric] = append(byMetric[item.Metric], item)
	}

	// Each metric must produce at least one item.
	for _, metric := range []string{"cve", "flags", "ports", "sshkeys", "updates"} {
		items, ok := byMetric[metric]
		if !ok || len(items) == 0 {
			t.Errorf("missing remediation for metric=%q", metric)
			continue
		}
		for _, item := range items {
			if item.Action == "" {
				t.Errorf("metric=%q: Action is empty", metric)
			}
		}
	}

	// cve with CriticalCVEs>0 must include a critical item.
	var hasCriticalCVE bool
	for _, item := range byMetric["cve"] {
		if item.Severity == SeverityCritical {
			hasCriticalCVE = true
		}
	}
	if !hasCriticalCVE {
		t.Error("cve metric: expected at least one critical item when CriticalCVEs > 0")
	}

	// flags with PrivilegedCount>0 must be critical.
	for _, item := range byMetric["flags"] {
		if item.Severity != SeverityCritical {
			t.Errorf("flags: severity=%q, want critical", item.Severity)
		}
	}
	// ports must be high.
	for _, item := range byMetric["ports"] {
		if item.Severity != SeverityHigh {
			t.Errorf("ports: severity=%q, want high", item.Severity)
		}
	}
	// sshkeys must be medium.
	for _, item := range byMetric["sshkeys"] {
		if item.Severity != SeverityMedium {
			t.Errorf("sshkeys: severity=%q, want medium", item.Severity)
		}
	}
	// updates must be low.
	for _, item := range byMetric["updates"] {
		if item.Severity != SeverityLow {
			t.Errorf("updates: severity=%q, want low", item.Severity)
		}
	}
}

// TestScoreMaxPenaltyAllSources verifies that the absolute worst case —
// all factors at maximum penalty — yields score=0 and grade=F.
func TestScoreMaxPenaltyAllSources(t *testing.T) {
	in := PostureInputs{10, 10, 10, 10, 10, 10}
	got := Score(in)
	if got.Score != 0 || got.Grade != GradeF {
		t.Errorf("max-penalty: score=%d grade=%q, want 0/F", got.Score, got.Grade)
	}
}

// TestScorePartialUnavailableScalesWeight verifies that when some data sources
// are unavailable, the available weight pool is used for scaling so the score
// does not artificially deflate.
//
// Only flags (w=30) and ports (w=15) available; both clean: score should be 100.
// Add 1 exposed port (penalty=3 out of available 45): scaledPenalty=(3×100)/45=6; score=94.
func TestScorePartialUnavailableScalesWeight(t *testing.T) {
	in := PostureInputs{
		CriticalCVEs: -1, HighCVEs: -1,
		PrivilegedCount: 0, ExposedAllIfaceCount: 0,
		StaleSSHKeyCount: -1, UpdateAvailableCount: -1,
	}
	got := Score(in)
	if got.Score != 100 {
		t.Errorf("partial clean: score = %d, want 100", got.Score)
	}

	in.ExposedAllIfaceCount = 1
	got = Score(in)
	wantScore := 100 - (3*100)/45 // = 93 (integer division: 300/45=6, 100-6=94)
	// Recompute exactly for the test.
	wantScore = 100 - (penaltyPorts(1)*100)/(30+15) // = 100 - (3*100)/45 = 93
	if got.Score != wantScore {
		t.Errorf("partial+1port: score = %d, want %d", got.Score, wantScore)
	}
}
