// Package cve integrates an image vulnerability scanner (Trivy) (Feature 20).
// Trivy is bundled in the backend image, so it works out of the box; when the
// binary is absent the feature reports "unavailable" rather than failing. Scans
// are keyed by image digest and cached in the DB (re-scan only when the digest
// changes or on demand). The Trivy JSON parser is pure and unit-tested.
//
// Cache/DB: the scanner points Trivy at a persistent, writable cache directory
// (TRIVY_CACHE_DIR, defaulting to /data/trivy-cache — the mounted data volume,
// owned by uid 65532 in the distroless runtime). The first scan downloads the
// vulnerability DB from mirror.gcr.io/aquasec/trivy-db, so the deploy needs
// egress to that mirror for CVE data to populate.
package cve

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

// defaultCacheDir is the persistent, writable Trivy cache location. /data is the
// mounted volume (writable by uid 65532), so the vuln DB survives restarts and
// the first scan's download is reused across subsequent scans.
const defaultCacheDir = "/data/trivy-cache"

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
// system PATH; an empty path means Trivy is unavailable. cacheDir is the
// persistent cache/DB location passed to Trivy via TRIVY_CACHE_DIR.
type Scanner struct {
	bin      string
	cacheDir string
	timeout  time.Duration
}

// NewScanner resolves the Trivy binary ($TRIVY_PATH first, then PATH — so the
// bundled /usr/local/bin/trivy is found with zero config, and an explicit
// TRIVY_PATH still wins) and the cache dir ($TRIVY_CACHE_DIR, else
// /data/trivy-cache).
func NewScanner() *Scanner {
	bin := os.Getenv("TRIVY_PATH")
	if bin == "" {
		if p, err := exec.LookPath("trivy"); err == nil {
			bin = p
		}
	}
	cacheDir := os.Getenv("TRIVY_CACHE_DIR")
	if cacheDir == "" {
		cacheDir = defaultCacheDir
	}
	return &Scanner{bin: bin, cacheDir: cacheDir, timeout: 4 * time.Minute}
}

// Available reports whether Trivy is installed.
func (s *Scanner) Available() bool { return s.bin != "" }

// env returns the subprocess environment with TRIVY_CACHE_DIR set to the
// persistent cache dir, creating the dir if missing (best-effort — Trivy will
// surface a clear error if the path is unwritable).
func (s *Scanner) env() []string {
	if s.cacheDir != "" {
		_ = os.MkdirAll(s.cacheDir, 0o755)
	}
	return append(os.Environ(), "TRIVY_CACHE_DIR="+s.cacheDir)
}

// Scan runs Trivy against an image reference and returns the parsed
// vulnerabilities. Returns ErrUnavailable when Trivy isn't installed.
//
// This is the registry path: Trivy resolves <imageRef> from a registry/local
// store. For locally-built/unpublished images use ScanTarball instead — a bare
// registry resolve fails for images that were never pushed/pulled.
func (s *Scanner) Scan(ctx context.Context, imageRef string) ([]Vuln, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	// --quiet suppresses progress; --scanners vuln limits to CVE scanning.
	// `--` ends option parsing so an image ref beginning with `-` can't be
	// misread as a flag (defense in depth; argv already avoids shell injection).
	cmd := exec.CommandContext(ctx, s.bin, "image", "--format", "json", "--quiet", "--scanners", "vuln", "--", imageRef)
	cmd.Env = s.env()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return ParseTrivyJSON(out)
}

// ScanTarball runs Trivy against an exported image tarball (`--input <tar>`),
// the path used for locally-built/unpublished images. tarPath is a Docker-format
// image archive (as produced by `docker save` / Engine API GET /images/get).
func (s *Scanner) ScanTarball(ctx context.Context, tarPath string) ([]Vuln, error) {
	if !s.Available() {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.bin, "image", "--input", tarPath, "--format", "json", "--quiet", "--scanners", "vuln")
	cmd.Env = s.env()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return ParseTrivyJSON(out)
}

// Status describes scanner readiness for the UI.
type Status struct {
	Available   bool       `json:"available"`
	Path        string     `json:"path"`
	Version     string     `json:"version"`
	DBUpdatedAt *time.Time `json:"db_updated_at"`
	DBAgeDays   *int       `json:"db_age_days"`
}

// trivyVersionJSON is the subset of `trivy version --format json` we read.
type trivyVersionJSON struct {
	Version         string `json:"Version"`
	VulnerabilityDB *struct {
		UpdatedAt time.Time `json:"UpdatedAt"`
	} `json:"VulnerabilityDB"`
}

// StatusInfo returns Trivy presence + vulnerability-DB freshness for the UI. It
// shells out to `trivy version --format json` (with the persistent cache dir so
// DB metadata is read from the right place). Errors degrade gracefully to a
// present-but-version-unknown status.
func (s *Scanner) StatusInfo(ctx context.Context) Status {
	st := Status{Available: s.Available(), Path: s.bin}
	if !s.Available() {
		return st
	}
	vctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(vctx, s.bin, "version", "--format", "json")
	cmd.Env = s.env()
	out, err := cmd.Output()
	if err != nil {
		return st
	}
	var v trivyVersionJSON
	if err := json.Unmarshal(out, &v); err != nil {
		return st
	}
	st.Version = v.Version
	if v.VulnerabilityDB != nil && !v.VulnerabilityDB.UpdatedAt.IsZero() {
		t := v.VulnerabilityDB.UpdatedAt
		st.DBUpdatedAt = &t
		days := int(time.Since(t).Hours() / 24)
		if days < 0 {
			days = 0
		}
		st.DBAgeDays = &days
	}
	return st
}

// WarmDB best-effort downloads the Trivy vulnerability DB so the first user scan
// isn't slow. It is non-fatal and time-bounded: offline deploys (no egress to
// the trivy-db mirror) still boot. Call once at startup in a goroutine.
func (s *Scanner) WarmDB(ctx context.Context) error {
	if !s.Available() {
		return ErrUnavailable
	}
	wctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(wctx, s.bin, "image", "--download-db-only", "--quiet")
	cmd.Env = s.env()
	return cmd.Run()
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

// tempTarPath returns a unique temp path for an exported image tarball.
func tempTarPath() string {
	return filepath.Join(os.TempDir(), "stratum-cve-"+time.Now().Format("20060102150405.000000000")+".tar")
}
