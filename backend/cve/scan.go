// Package cve integrates an image vulnerability scanner (Trivy) (Feature 20).
// Trivy is optional: when the binary is absent the feature reports
// "unavailable" rather than failing. Scans are keyed by image digest and cached
// in the DB (re-scan only when the digest changes or on demand). The Trivy JSON
// parser is pure and unit-tested.
package cve

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"time"
)

// Severity values (Trivy's uppercase set).
const (
	SevCritical = "CRITICAL"
	SevHigh     = "HIGH"
	SevMedium   = "MEDIUM"
	SevLow      = "LOW"
	SevUnknown  = "UNKNOWN"
)

// Vuln is one parsed vulnerability.
type Vuln struct {
	CVEID            string `json:"cve_id"`
	Severity         string `json:"severity"`
	Package          string `json:"package"`
	InstalledVersion string `json:"installed_version"`
	FixedVersion     string `json:"fixed_version"`
	Title            string `json:"title"`
}

// Scanner runs Trivy. The binary path is resolved from $TRIVY_PATH or the
// system PATH; an empty path means Trivy is unavailable.
type Scanner struct {
	bin     string
	timeout time.Duration
}

// NewScanner resolves the Trivy binary ($TRIVY_PATH first, then PATH).
func NewScanner() *Scanner {
	bin := os.Getenv("TRIVY_PATH")
	if bin == "" {
		if p, err := exec.LookPath("trivy"); err == nil {
			bin = p
		}
	}
	return &Scanner{bin: bin, timeout: 4 * time.Minute}
}

// Available reports whether Trivy is installed.
func (s *Scanner) Available() bool { return s.bin != "" }

// Scan runs Trivy against an image reference and returns the parsed
// vulnerabilities. Returns ErrUnavailable when Trivy isn't installed.
func (s *Scanner) Scan(ctx context.Context, imageRef string) ([]Vuln, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	// --quiet suppresses progress; --scanners vuln limits to CVE scanning.
	out, err := exec.CommandContext(ctx, s.bin, "image", "--format", "json", "--quiet", "--scanners", "vuln", imageRef).Output()
	if err != nil {
		return nil, err
	}
	return ParseTrivyJSON(out)
}

// ParseTrivyJSON extracts vulnerabilities from Trivy's `image --format json`
// output. Pure — unit-tested with sample reports.
func ParseTrivyJSON(data []byte) ([]Vuln, error) {
	var report struct {
		Results []struct {
			Vulnerabilities []struct {
				VulnerabilityID  string `json:"VulnerabilityID"`
				PkgName          string `json:"PkgName"`
				InstalledVersion string `json:"InstalledVersion"`
				FixedVersion     string `json:"FixedVersion"`
				Severity         string `json:"Severity"`
				Title            string `json:"Title"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	out := []Vuln{}
	for _, r := range report.Results {
		for _, v := range r.Vulnerabilities {
			out = append(out, Vuln{
				CVEID: v.VulnerabilityID, Severity: v.Severity, Package: v.PkgName,
				InstalledVersion: v.InstalledVersion, FixedVersion: v.FixedVersion, Title: v.Title,
			})
		}
	}
	return out, nil
}

// SeverityCounts tallies vulns by severity.
func SeverityCounts(vulns []Vuln) map[string]int {
	counts := map[string]int{}
	for _, v := range vulns {
		counts[v.Severity]++
	}
	return counts
}
