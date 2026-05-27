package cve

import (
	"context"
	"testing"
)

const sampleTrivyJSON = `{
  "Results": [
    {
      "Target": "nginx:latest (debian 12.4)",
      "Vulnerabilities": [
        {"VulnerabilityID":"CVE-2024-0001","PkgName":"libc6","InstalledVersion":"2.36-9","FixedVersion":"2.36-10","Severity":"CRITICAL","Title":"libc overflow"},
        {"VulnerabilityID":"CVE-2024-0002","PkgName":"openssl","InstalledVersion":"3.0.11","FixedVersion":"","Severity":"HIGH","Title":"openssl issue"}
      ]
    },
    {
      "Target": "app",
      "Vulnerabilities": [
        {"VulnerabilityID":"CVE-2024-0003","PkgName":"zlib","InstalledVersion":"1.2","Severity":"MEDIUM","Title":"zlib"}
      ]
    },
    { "Target": "no-vulns", "Vulnerabilities": null }
  ]
}`

func TestParseTrivyJSON(t *testing.T) {
	vulns, err := ParseTrivyJSON([]byte(sampleTrivyJSON))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vulns) != 3 {
		t.Fatalf("got %d vulns, want 3", len(vulns))
	}
	if vulns[0].CVEID != "CVE-2024-0001" || vulns[0].Severity != "CRITICAL" || vulns[0].FixedVersion != "2.36-10" {
		t.Errorf("vuln0 = %+v", vulns[0])
	}
	counts := SeverityCounts(vulns)
	if counts[SevCritical] != 1 || counts[SevHigh] != 1 || counts[SevMedium] != 1 {
		t.Errorf("counts = %+v", counts)
	}
}

func TestParseTrivyEmpty(t *testing.T) {
	vulns, err := ParseTrivyJSON([]byte(`{"Results":[]}`))
	if err != nil || len(vulns) != 0 {
		t.Errorf("empty report: %d vulns, err %v", len(vulns), err)
	}
}

func TestScannerUnavailable(t *testing.T) {
	s := &Scanner{bin: ""} // no trivy
	if s.Available() {
		t.Error("empty bin should be unavailable")
	}
	if _, err := s.Scan(context.TODO(), "nginx:latest"); err != ErrUnavailable {
		t.Errorf("Scan without trivy = %v, want ErrUnavailable", err)
	}
}
