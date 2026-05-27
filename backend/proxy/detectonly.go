package proxy

import "context"

func init() {
	Register(&detectOnly{name: "caddy", patterns: []string{"caddy"}})
	Register(&detectOnly{name: "cloudflared", patterns: []string{"cloudflare/cloudflared", "cloudflared"}})
	Register(&detectOnly{name: "haproxy", patterns: []string{"haproxy"}})
	Register(&detectOnly{name: "nginx", patterns: []string{"nginx"}})
}

// detectOnly is an adapter that recognises a proxy tool but manages it only via
// its config file (read/write through the filesystem browser) — there is no
// supported API for structured rule CRUD yet. It surfaces "detected" in the UI
// with no editable rule set. Per-tool config-file parsing is an additive
// follow-on (a richer adapter can replace the detect-only registration).
//
// Note: "nginx" is registered LAST so the more specific
// "jc21/nginx-proxy-manager" (NPM) wins detection for that image.
type detectOnly struct {
	name     string
	patterns []string
}

func (d *detectOnly) Name() string                 { return d.name }
func (d *detectOnly) ImagePatterns() []string       { return d.patterns }
func (d *detectOnly) Capabilities() Capabilities     { return Capabilities{} } // detection only
func (d *detectOnly) ListRules(context.Context, Conn) ([]Rule, error) {
	return nil, ErrUnsupported
}
func (d *detectOnly) CreateRule(context.Context, Conn, Rule) (Rule, error) {
	return Rule{}, ErrUnsupported
}
func (d *detectOnly) UpdateRule(context.Context, Conn, string, Rule) error { return ErrUnsupported }
func (d *detectOnly) DeleteRule(context.Context, Conn, string) error       { return ErrUnsupported }
