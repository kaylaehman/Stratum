// Package drexport builds a portable disaster-recovery manifest of the
// homelab's wiring: nodes, stacks, volumes, DNS records, reverse-proxy routes,
// certificate metadata, and secret references. It is read-only — it never
// mutates any store — and is security-critical by design:
//
//   - Secret VALUES are NEVER included. Only group + key names appear.
//   - Certificate PRIVATE KEYS are never included. Only domain, SANs, issuer,
//     expiry date, and node path appear.
//   - Node CREDENTIALS (SSH passwords, API tokens) are never included.
//
// All three properties are enforced structurally (the types carry no such
// fields) and verified by the acceptance test in manifest_test.go.
package drexport

import "time"

// Manifest is the top-level export document.
type Manifest struct {
	GeneratedAt time.Time        `json:"generated_at"          yaml:"generated_at"`
	Version     int              `json:"version"               yaml:"version"`
	Nodes       []NodeEntry      `json:"nodes"                 yaml:"nodes"`
	Stacks      []StackEntry     `json:"stacks"                yaml:"stacks"`
	Volumes     []VolumeEntry    `json:"volumes"               yaml:"volumes"`
	DNS         []DNSEntry       `json:"dns"                   yaml:"dns"`
	ProxyRoutes []ProxyEntry     `json:"proxy_routes"          yaml:"proxy_routes"`
	Certs       []CertEntry      `json:"certs"                 yaml:"certs"`
	Secrets     []SecretRef      `json:"secret_references"     yaml:"secret_references"`
}

// NodeEntry is the public face of a registered host. Credentials and host keys
// are intentionally absent.
type NodeEntry struct {
	ID           string         `json:"id"             yaml:"id"`
	Name         string         `json:"name"           yaml:"name"`
	Host         string         `json:"host"           yaml:"host"`
	Port         int            `json:"port"           yaml:"port"`
	Type         string         `json:"type"           yaml:"type"`
	OSType       string         `json:"os_type"        yaml:"os_type"`
	AuthMethod   string         `json:"auth_method"    yaml:"auth_method"`
	Capabilities map[string]any `json:"capabilities"   yaml:"capabilities"`
	Status       string         `json:"status"         yaml:"status"`
}

// StackEntry describes one live Compose project. Compose YAML and secret
// values are omitted; only the path and masked env-key names are included.
type StackEntry struct {
	NodeID      string   `json:"node_id"      yaml:"node_id"`
	NodeName    string   `json:"node_name"    yaml:"node_name"`
	Project     string   `json:"project"      yaml:"project"`
	ComposePath string   `json:"compose_path" yaml:"compose_path"`
	// EnvKeys lists the names of environment variables configured for this
	// stack — values are NEVER included.
	EnvKeys     []string `json:"env_keys"     yaml:"env_keys"`
}

// VolumeEntry is one Docker volume across any node.
type VolumeEntry struct {
	NodeID     string `json:"node_id"    yaml:"node_id"`
	NodeName   string `json:"node_name"  yaml:"node_name"`
	Name       string `json:"name"       yaml:"name"`
	Driver     string `json:"driver"     yaml:"driver"`
	Mountpoint string `json:"mountpoint" yaml:"mountpoint"`
	Status     string `json:"status"     yaml:"status"`
	SizeBytes  int64  `json:"size_bytes" yaml:"size_bytes"`
}

// DNSEntry is one DNS record as reported by the detected DNS adapter.
type DNSEntry struct {
	NodeID      string `json:"node_id"      yaml:"node_id"`
	NodeName    string `json:"node_name"    yaml:"node_name"`
	AdapterType string `json:"adapter_type" yaml:"adapter_type"`
	RecordType  string `json:"type"         yaml:"type"`
	Name        string `json:"name"         yaml:"name"`
	Value       string `json:"value"        yaml:"value"`
	TTL         int    `json:"ttl"          yaml:"ttl"`
}

// ProxyEntry is one reverse-proxy route.
type ProxyEntry struct {
	NodeID      string `json:"node_id"      yaml:"node_id"`
	NodeName    string `json:"node_name"    yaml:"node_name"`
	AdapterType string `json:"adapter_type" yaml:"adapter_type"`
	SourceHost  string `json:"source_host"  yaml:"source_host"`
	SourcePath  string `json:"source_path"  yaml:"source_path"`
	TargetURL   string `json:"target_url"   yaml:"target_url"`
	SSLEnabled  bool   `json:"ssl_enabled"  yaml:"ssl_enabled"`
}

// CertEntry is TLS certificate METADATA. The private key is never included.
type CertEntry struct {
	NodeID   string     `json:"node_id"    yaml:"node_id"`
	NodeName string     `json:"node_name"  yaml:"node_name"`
	Domain   string     `json:"domain"     yaml:"domain"`
	SANs     []string   `json:"sans"       yaml:"sans"`
	Issuer   string     `json:"issuer"     yaml:"issuer"`
	Path     string     `json:"path"       yaml:"path"`
	NotAfter *time.Time `json:"not_after"  yaml:"not_after"`
}

// SecretRef is a REFERENCE to one secret: group name + key name only.
// The encrypted blob and plaintext value are structurally absent.
type SecretRef struct {
	GroupID   string `json:"group_id"   yaml:"group_id"`
	GroupName string `json:"group_name" yaml:"group_name"`
	KeyName   string `json:"key_name"   yaml:"key_name"`
}
