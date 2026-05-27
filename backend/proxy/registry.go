package proxy

import (
	"errors"
	"strings"
)

// ErrUnsupported is returned by adapters for an operation they don't implement
// (e.g. a file-only proxy that can't be edited via API).
var ErrUnsupported = errors.New("proxy: operation not supported by this adapter")

// registry holds the built-in adapters. Detection iterates it; adding a tool is
// a single Register call (see init in the adapter files).
var registry []Adapter

// Register adds an adapter to the detection registry. Called from adapter init.
func Register(a Adapter) { registry = append(registry, a) }

// Adapters returns all registered adapters (detection order).
func Adapters() []Adapter { return registry }

// DetectByImages returns the adapter whose image pattern best matches any of the
// given container image refs, or nil if no supported proxy is present. The
// MOST SPECIFIC (longest) matching pattern wins, so "jc21/nginx-proxy-manager"
// (NPM) beats the generic "nginx" adapter regardless of registration order.
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

// SupportedTools lists every registered adapter's name + capabilities, for the
// UI's "no proxy detected, here's what's supported" state.
type ToolInfo struct {
	Name         string       `json:"name"`
	Capabilities Capabilities `json:"capabilities"`
}

func SupportedTools() []ToolInfo {
	out := make([]ToolInfo, 0, len(registry))
	for _, a := range registry {
		out = append(out, ToolInfo{Name: a.Name(), Capabilities: a.Capabilities()})
	}
	return out
}
