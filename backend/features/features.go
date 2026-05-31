// Package features is the feature-flag catalog + resolver (FEATURES.md "Feature
// Flags"). Each optional feature has a flag with a built-in default; admins can
// override it. The frontend uses the resolved map to show "Not configured"
// states instead of broken panels.
package features

import (
	"context"
	"sort"

	"github.com/kaylaehman/stratum/backend/db"
)

// Flag is one catalog entry.
type Flag struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Enabled     bool   `json:"enabled"`
	Default     bool   `json:"default"`
	Description string `json:"description"`
}

// catalog is the known flag set. Default reflects whether the feature is built
// and on by default; the two not-yet-implemented features default off.
var catalog = []Flag{
	{Key: "feature.automations", Label: "Automations", Default: true, Description: "Autonomous automation engine (8 configurable automations)."},
	{Key: "feature.config_versions", Label: "Config Versions", Default: true, Description: "Track config file version history and detect drift."},
	{Key: "feature.config_git", Label: "Config Git Backend", Default: false, Description: "Push config snapshots to a remote git repository (not yet implemented)."},
	{Key: "feature.alert_policies", Label: "Alert Policies", Default: true, Description: "Route and suppress alerts via configurable policies."},
	{Key: "feature.reverse_proxy", Label: "Reverse Proxy", Default: true, Description: "Detect and view reverse-proxy rules per node."},
	{Key: "feature.dns_management", Label: "DNS Management", Default: true, Description: "Detect and view DNS records per node."},
	{Key: "feature.cert_management", Label: "Certificate Monitoring", Default: true, Description: "Scan and alert on TLS certificate expiry."},
	{Key: "feature.health_check_editor", Label: "Health Check Editor", Default: true, Description: "Edit container healthchecks."},
	{Key: "feature.wake_on_lan", Label: "Wake-on-LAN", Default: true, Description: "Send WOL magic packets to offline nodes."},
	{Key: "feature.action_2fa", Label: "Step-up 2FA", Default: true, Description: "Require a TOTP confirmation before destructive actions."},
	{Key: "feature.ai_agent", Label: "AI Assistant", Default: true, Description: "In-context AI help and agent memory."},
	{Key: "feature.sso_passthrough", Label: "SSO Passthrough", Default: false, Description: "Add auth in front of containers (not yet implemented)."},
	{Key: "feature.chat_integration", Label: "Chat Integration", Default: false, Description: "Inbound chat commands (not yet implemented)."},
}

// known maps key -> catalog entry for validation.
var known = func() map[string]Flag {
	m := make(map[string]Flag, len(catalog))
	for _, f := range catalog {
		m[f.Key] = f
	}
	return m
}()

// Valid reports whether key is a known flag.
func Valid(key string) bool {
	_, ok := known[key]
	return ok
}

// Service resolves catalog defaults against stored overrides.
type Service struct{ store db.Store }

// New wires the store.
func New(store db.Store) *Service { return &Service{store: store} }

// List returns every catalog flag with its resolved Enabled value (stored
// override or built-in default).
func (s *Service) List(ctx context.Context) ([]Flag, error) {
	overrides, err := s.store.ListFeatureFlags(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Flag, 0, len(catalog))
	for _, f := range catalog {
		f.Enabled = f.Default
		if v, ok := overrides[f.Key]; ok {
			f.Enabled = v
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, nil
}

// Set overrides a flag.
func (s *Service) Set(ctx context.Context, key string, enabled bool) error {
	return s.store.SetFeatureFlag(ctx, key, enabled)
}

// Enabled resolves a single flag (stored override or built-in default). Unknown
// keys are reported disabled.
func (s *Service) Enabled(ctx context.Context, key string) bool {
	def, ok := known[key]
	if !ok {
		return false
	}
	overrides, err := s.store.ListFeatureFlags(ctx)
	if err != nil {
		return def.Default
	}
	if v, ok := overrides[key]; ok {
		return v
	}
	return def.Default
}
