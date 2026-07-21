package agent

import (
	"encoding/json"
	"testing"

	"github.com/KAE-Labs/stratum/backend/capabilities"
)

func TestMergeCaps(t *testing.T) {
	t.Run("sets agent true and preserves proxmox_auth_status", func(t *testing.T) {
		existing := `{"proxmox":true,"docker":true,"agent":false,"proxmox_auth_status":"confirmed"}`
		newCaps := capabilities.Set{Proxmox: true, Docker: true, Agent: true}

		out, err := mergeCaps(existing, newCaps)
		if err != nil {
			t.Fatalf("mergeCaps: %v", err)
		}

		var got capsEnvelope
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if !got.Agent {
			t.Error("want Agent=true")
		}
		if !got.Proxmox {
			t.Error("want Proxmox=true preserved")
		}
		if got.ProxmoxAuthStatus != "confirmed" {
			t.Errorf("ProxmoxAuthStatus = %q, want %q", got.ProxmoxAuthStatus, "confirmed")
		}
	})

	t.Run("clears agent on empty existing", func(t *testing.T) {
		out, err := mergeCaps("", capabilities.Set{Docker: true, Agent: false})
		if err != nil {
			t.Fatalf("mergeCaps: %v", err)
		}
		var got capsEnvelope
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Agent {
			t.Error("want Agent=false")
		}
		if !got.Docker {
			t.Error("want Docker=true")
		}
	})

	t.Run("invalid existing JSON is tolerated", func(t *testing.T) {
		_, err := mergeCaps("not-json", capabilities.Set{Agent: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
