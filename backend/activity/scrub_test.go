package activity

import (
	"context"
	"strings"
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
)

func TestScrubSecrets(t *testing.T) {
	in := map[string]any{
		"ssh_user":        "kayla",
		"ssh_password":    "hunter2",
		"ssh_private_key": "-----BEGIN-----",
		"ssh_passphrase":  "phrase",
		"proxmox_secret":  "abcd",
		"docker_tls_key":  "keymaterial",
		"docker_tls_cert": "PUBLIC CERT",
		"api_token":       "t0ken",
		"bearer":          "Bearer xyz",
		"authorization":   "Basic abc",
		"apikey":          "ak_123",
		"nested": map[string]any{
			"jwt_secret": "leaked",
			"path":       "/etc/conf",
		},
		"list": []any{
			map[string]any{"password": "x"},
		},
	}
	out, ok := scrubSecrets(in).(map[string]any)
	if !ok {
		t.Fatal("scrubSecrets did not return a map")
	}

	mustRedact := []string{"ssh_password", "ssh_private_key", "ssh_passphrase", "proxmox_secret", "docker_tls_key", "api_token", "bearer", "authorization", "apikey"}
	for _, k := range mustRedact {
		if out[k] != redacted {
			t.Errorf("%s = %v, want redacted", k, out[k])
		}
	}
	if out["ssh_user"] != "kayla" {
		t.Errorf("ssh_user should be preserved, got %v", out["ssh_user"])
	}
	if out["docker_tls_cert"] != "PUBLIC CERT" {
		t.Errorf("cert is public, should be preserved, got %v", out["docker_tls_cert"])
	}
	nested := out["nested"].(map[string]any)
	if nested["jwt_secret"] != redacted {
		t.Errorf("nested jwt_secret = %v, want redacted", nested["jwt_secret"])
	}
	if nested["path"] != "/etc/conf" {
		t.Errorf("nested path should be preserved, got %v", nested["path"])
	}
	listItem := out["list"].([]any)[0].(map[string]any)
	if listItem["password"] != redacted {
		t.Errorf("password inside list = %v, want redacted", listItem["password"])
	}
}

// fakeAppendStore captures the last appended row so we can assert the scrub ran.
type fakeAppendStore struct {
	db.Store
	last db.ActivityEntry
}

func (f *fakeAppendStore) AppendActivity(_ context.Context, e db.ActivityEntry) error {
	f.last = e
	return nil
}

func TestAppendScrubsSecretsBeforeWrite(t *testing.T) {
	fake := &fakeAppendStore{}
	s := NewStore(fake)
	err := s.Append(context.Background(), Entry{
		Action: "node.create",
		Detail: map[string]any{"ssh_user": "kayla", "ssh_password": "hunter2"},
	})
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	if fake.last.DetailJSON == nil {
		t.Fatal("detail not written")
	}
	d := *fake.last.DetailJSON
	if strings.Contains(d, "hunter2") {
		t.Errorf("password leaked into the audit row: %s", d)
	}
	if !strings.Contains(d, redacted) {
		t.Errorf("expected a redaction marker, got: %s", d)
	}
	if !strings.Contains(d, "kayla") {
		t.Errorf("non-secret field should survive: %s", d)
	}
}
