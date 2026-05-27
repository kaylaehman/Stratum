// Package proxy implements Reverse Proxy Management (Feature F1): a
// detection-based, plugin-style layer over whatever proxy tool runs on a node.
// Stratum detects the proxy from the node's container images and loads the
// matching adapter; adding a new tool means adding one adapter + registering it,
// with no changes to the API, data model, or UI.
package proxy

import "context"

// Capabilities describes what an adapter can do, so the UI can show read-only
// vs editable affordances.
type Capabilities struct {
	List   bool `json:"list"`   // can enumerate existing rules
	Create bool `json:"create"` // can add a rule
	Update bool `json:"update"` // can modify a rule
	Delete bool `json:"delete"` // can remove a rule
}

// Rule is the tool-agnostic representation of one reverse-proxy route.
type Rule struct {
	ID          string `json:"id"`
	AdapterType string `json:"adapter_type"`
	SourceHost  string `json:"source_host"`
	SourcePath  string `json:"source_path,omitempty"`
	TargetURL   string `json:"target_url"`
	SSLEnabled  bool   `json:"ssl_enabled"`
	CertID      string `json:"cert_id,omitempty"`
	AuthEnabled bool   `json:"auth_enabled"`
}

// CertInfo is a certificate surfaced by a proxy adapter (feeds Feature F4).
type CertInfo struct {
	Domain   string `json:"domain"`
	Issuer   string `json:"issuer,omitempty"`
	NotAfter string `json:"not_after,omitempty"` // RFC3339
}

// Adapter is one proxy tool's integration. Detection is by container image
// pattern (see Match). Endpoint/credentials, where needed, are bound per call
// by the service from node config + the secrets vault.
type Adapter interface {
	// Name is the stable adapter identifier (e.g. "caddy", "traefik").
	Name() string
	// ImagePatterns are substrings matched against a container image ref to
	// detect this tool (e.g. "caddy", "jc21/nginx-proxy-manager").
	ImagePatterns() []string
	// Capabilities reports what operations this adapter supports.
	Capabilities() Capabilities
	// ListRules returns the proxy's current routes. endpoint is the adapter's
	// admin/API base URL resolved by the service (may be ""); token is an
	// optional secret. Adapters that can't list return ErrUnsupported.
	ListRules(ctx context.Context, conn Conn) ([]Rule, error)
	// CreateRule / UpdateRule / DeleteRule mutate routes; unsupported adapters
	// return ErrUnsupported.
	CreateRule(ctx context.Context, conn Conn, r Rule) (Rule, error)
	UpdateRule(ctx context.Context, conn Conn, id string, r Rule) error
	DeleteRule(ctx context.Context, conn Conn, id string) error
}

// Conn carries everything an adapter needs to reach its tool for one call: the
// admin/API base URL and an optional auth token. The service builds it from the
// node's detected config + the secrets vault.
type Conn struct {
	Endpoint string
	Token    string
}
