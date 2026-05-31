package secrets

import (
	"strings"
	"testing"
)

func TestScanText_FlagsPasswordKey(t *testing.T) {
	content := `
DB_PASSWORD=s3cr3tpassword123
APP_PORT=8080
`
	findings := ScanText("/opt/myapp/.env", content)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for DB_PASSWORD")
	}
	found := false
	for _, f := range findings {
		if f.KeyName == "DB_PASSWORD" {
			found = true
			// CRITICAL: findings must never include the value
			if strings.Contains(f.Reason, "s3cr3tpassword123") {
				t.Error("finding reason must not contain secret value")
			}
		}
		// The value must NEVER appear in any finding field
		if strings.Contains(f.Path, "s3cr3tpassword123") ||
			strings.Contains(f.KeyName, "s3cr3tpassword123") ||
			strings.Contains(f.Reason, "s3cr3tpassword123") {
			t.Errorf("secret value leaked into finding: %+v", f)
		}
	}
	if !found {
		t.Error("expected finding with KeyName=DB_PASSWORD")
	}
}

func TestScanText_FlagsAPIKey(t *testing.T) {
	content := `API_KEY=AKIAIOSFODNN7EXAMPLE`
	findings := ScanText("/app/.env", content)
	if len(findings) == 0 {
		t.Fatal("expected finding for API_KEY")
	}
}

func TestScanText_FlagsToken(t *testing.T) {
	content := `GITHUB_TOKEN=ghp_abcdefghijklmnopqrstuvwxyz0123456789`
	findings := ScanText("/srv/deploy/.env", content)
	if len(findings) == 0 {
		t.Fatal("expected finding for GITHUB_TOKEN")
	}
	// Value must not appear in any finding
	for _, f := range findings {
		if strings.Contains(f.Reason, "ghp_") ||
			strings.Contains(f.KeyName, "ghp_") {
			t.Errorf("value leaked into finding: %+v", f)
		}
	}
}

func TestScanText_DetectsRSAPrivateKeyHeader(t *testing.T) {
	content := `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`
	findings := ScanText("/home/user/.ssh/id_rsa", content)
	if len(findings) == 0 {
		t.Fatal("expected finding for RSA private key header")
	}
	for _, f := range findings {
		if strings.Contains(f.Reason, "MIIEowIBAAKCAQEA") {
			t.Error("key material must not appear in finding reason")
		}
	}
}

func TestScanText_DetectsOpenSSHKey(t *testing.T) {
	content := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEA...\n-----END OPENSSH PRIVATE KEY-----"
	findings := ScanText("/root/.ssh/deploy_key", content)
	if len(findings) == 0 {
		t.Fatal("expected finding for OPENSSH PRIVATE KEY header")
	}
}

func TestScanText_DoesNotFlagBenignVars(t *testing.T) {
	content := `
TZ=UTC
APP_PORT=8080
LOG_LEVEL=info
APP_NAME=myapp
DEBUG=false
`
	findings := ScanText("/app/.env", content)
	for _, f := range findings {
		t.Errorf("unexpected finding for benign var %q: %s", f.KeyName, f.Reason)
	}
}

func TestScanText_DoesNotFlagURLs(t *testing.T) {
	content := `
BASE_URL=https://api.example.com/v1/endpoint/path
REDIRECT_URL=https://auth.provider.com/oauth/callback
`
	findings := ScanText("/app/.env", content)
	// URLs should not be flagged as high-entropy secrets
	for _, f := range findings {
		if f.Reason == "value has high entropy, may be a secret token" {
			t.Errorf("URL should not be flagged as high-entropy secret: %+v", f)
		}
	}
}

func TestScanText_NoValueLeak_MultipleRules(t *testing.T) {
	// This value would trigger BOTH the key-name rule and the entropy rule.
	secretVal := "xK9$mP2#nQ7@rL5&vW3!yB8^tD4%jF6*"
	content := "API_SECRET=" + secretVal
	findings := ScanText("/etc/app/config.env", content)
	if len(findings) == 0 {
		t.Fatal("expected findings for API_SECRET with high-entropy value")
	}
	for _, f := range findings {
		// Assert value never leaks into any finding field
		if strings.Contains(f.Path, secretVal) ||
			strings.Contains(f.KeyName, secretVal) ||
			strings.Contains(f.Reason, secretVal) {
			t.Errorf("SECRET VALUE LEAKED into finding: %+v", f)
		}
	}
}

func TestScanText_SkipsBlankAndComments(t *testing.T) {
	content := `
# This is a comment
# PASSWORD=shouldbeskipped

PORT=3000
`
	findings := ScanText("/app/.env", content)
	for _, f := range findings {
		if strings.Contains(f.Reason, "shouldbeskipped") {
			t.Error("comment content must not appear in findings")
		}
	}
}

func TestScanText_SkipsEmptyValues(t *testing.T) {
	content := `DB_PASSWORD=`
	findings := ScanText("/app/.env", content)
	// Empty value = no secret material present; key-name match on an empty
	// value should not produce a finding.
	for _, f := range findings {
		t.Errorf("empty value should not produce findings, got: %+v", f)
	}
}
