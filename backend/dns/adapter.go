// Package dns implements DNS Record Management (Feature F3): the same
// detection-based, plugin-style adapter pattern as the reverse-proxy layer.
// Stratum detects the DNS tool from a node's container images and loads the
// matching adapter; adding a tool is a single Register call.
package dns

import (
	"context"
	"errors"
	"strings"
)

// ErrUnsupported is returned by adapters for operations they don't implement.
var ErrUnsupported = errors.New("dns: operation not supported by this adapter")

// Capabilities describes what an adapter can do.
type Capabilities struct {
	List   bool `json:"list"`
	Create bool `json:"create"`
	Update bool `json:"update"`
	Delete bool `json:"delete"`
}

// Record is the tool-agnostic representation of one DNS record.
type Record struct {
	ID          string `json:"id"`
	AdapterType string `json:"adapter_type"`
	Type        string `json:"type"` // A | AAAA | CNAME | PTR | TXT | SRV
	Name        string `json:"name"`
	Value       string `json:"value"`
	TTL         int    `json:"ttl,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

// Conn carries the admin/API base URL + optional token for one call.
type Conn struct {
	Endpoint string
	Token    string
}

// Adapter is one DNS tool's integration.
type Adapter interface {
	Name() string
	ImagePatterns() []string
	Capabilities() Capabilities
	ListRecords(ctx context.Context, conn Conn) ([]Record, error)
	CreateRecord(ctx context.Context, conn Conn, r Record) (Record, error)
	UpdateRecord(ctx context.Context, conn Conn, id string, r Record) error
	DeleteRecord(ctx context.Context, conn Conn, id string) error
}

// --- registry ---

var registry []Adapter

// Register adds an adapter (called from adapter init).
func Register(a Adapter) { registry = append(registry, a) }

// Adapters returns all registered adapters.
func Adapters() []Adapter { return registry }

// DetectByImages returns the adapter whose image pattern best (most specifically)
// matches any image ref, or nil if none.
func DetectByImages(images []string) Adapter {
	var best Adapter
	bestLen := -1
	for _, a := range registry {
		for _, pat := range a.ImagePatterns() {
			lp := strings.ToLower(pat)
			for _, img := range images {
				if strings.Contains(strings.ToLower(img), lp) && len(lp) > bestLen {
					best, bestLen = a, len(lp)
				}
			}
		}
	}
	return best
}

// ToolInfo is one supported tool for the empty-state catalog.
type ToolInfo struct {
	Name         string       `json:"name"`
	Capabilities Capabilities `json:"capabilities"`
}

// SupportedTools lists every registered adapter.
func SupportedTools() []ToolInfo {
	out := make([]ToolInfo, 0, len(registry))
	for _, a := range registry {
		out = append(out, ToolInfo{Name: a.Name(), Capabilities: a.Capabilities()})
	}
	return out
}
