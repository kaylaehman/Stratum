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
