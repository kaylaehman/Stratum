// Package proxy implements Reverse Proxy Management (Feature F1): a
// detection-based, plugin-style layer over whatever proxy tool runs on a node.
// Stratum detects the proxy from the node's container images and loads the
// matching adapter; adding a new tool means adding one adapter + registering it,
// with no changes to the API, data model, or UI.
package proxy

import (
	"context"
	"io"
)

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
	// Resolved is the container/resource this rule's TargetURL points at, when
	// Stratum can match it against known container inventory + published ports.
	// Nil when the target could not be resolved (external host, unknown port,
	// or no matching container). Enables a UI deep-link to the target.
	Resolved *ResolvedTarget `json:"resolved,omitempty"`
}

// ResolvedTarget identifies the container a proxy rule's TargetURL points at.
// ContainerID is the inventory Container.ID used by the frontend deep-link
// (/resources?node=<NodeID>&container=<ContainerID>).
type ResolvedTarget struct {
	NodeID      string `json:"node_id"`
	ContainerID string `json:"container_id"` // inventory Container.ID (frontend deep-link id)
	Name        string `json:"name"`
	MatchKind   string `json:"match_kind"` // "container_name" | "localhost_port" | "host_ip_port"
}

// Match-kind values for ResolvedTarget.MatchKind.
const (
	MatchContainerName = "container_name"
	MatchLocalhostPort = "localhost_port"
	MatchHostIPPort    = "host_ip_port"
)

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
// admin/API base URL, an optional auth token, and optional host-filesystem
// access for config-file adapters. The service builds it from the node's
// detected config + the secrets vault.
type Conn struct {
	Endpoint string
	Token    string
	// ReadFile reads a file from the node's host filesystem. Nil when host
	// file access is not available (e.g. no SSH credentials configured).
	ReadFile func(ctx context.Context, path string) (io.ReadCloser, error)
	// MountCandidates carries pre-resolved host-side config file paths derived
	// from the container's bind mounts. Used by file-based adapters
	// (e.g. cloudflared) to locate their config before falling back to defaults.
	MountCandidates []string
	// Config carries non-secret adapter-specific configuration parsed from the
	// node's proxy_config.config_json (e.g. Cloudflare account_id / tunnel_id).
	// Nil for adapters that don't need it.
	Config map[string]string
}
