// Package capabilities provides a typed view over a node's capabilities_json
// and a single gate (Require) every node-specific handler calls before using a
// Docker/Proxmox/agent API. CLAUDE.md mandates this check so the platform
// degrades gracefully instead of assuming a capability is present.
package capabilities

import (
	"encoding/json"
	"fmt"
)

// Capability names a node feature that may or may not be present.
type Capability string

const (
	Proxmox Capability = "proxmox"
	Docker  Capability = "docker"
	Agent   Capability = "agent"
	Systemd Capability = "systemd"
	Cron    Capability = "cron"
)

// Set is the decoded capabilities_json for a node.
type Set struct {
	Proxmox bool `json:"proxmox"`
	Docker  bool `json:"docker"`
	Agent   bool `json:"agent"`
	Systemd bool `json:"systemd"`
	Cron    bool `json:"cron"`
}

// Parse decodes capabilities_json. An empty or nil input yields a zero Set
// (all capabilities absent), which is the correct conservative default.
func Parse(data []byte) (Set, error) {
	var s Set
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Set{}, fmt.Errorf("capabilities: parse: %w", err)
	}
	return s, nil
}

// Has reports whether the named capability is present.
func (s Set) Has(c Capability) bool {
	switch c {
	case Proxmox:
		return s.Proxmox
	case Docker:
		return s.Docker
	case Agent:
		return s.Agent
	case Systemd:
		return s.Systemd
	case Cron:
		return s.Cron
	default:
		return false
	}
}

// ErrCapabilityUnavailable is returned by Require when the node lacks the
// requested capability. The API layer maps it to a 422 response carrying the
// capability name.
type ErrCapabilityUnavailable struct {
	Capability Capability
}

func (e *ErrCapabilityUnavailable) Error() string {
	return fmt.Sprintf("capability unavailable: %s", e.Capability)
}

// Require returns nil if the capability is present, otherwise an
// *ErrCapabilityUnavailable naming the missing capability.
func Require(s Set, c Capability) error {
	if s.Has(c) {
		return nil
	}
	return &ErrCapabilityUnavailable{Capability: c}
}
