package webhooks

import "sync"

// TriggerDef describes a single alert trigger in the registry.
//
// # Registration API
//
// Other packages register triggers by calling Register, typically from init()
// or a constructor called at startup. This allows uptime monitors, agentic
// remediation, and any future feature to add triggers without modifying this
// package:
//
//	webhooks.Register(webhooks.TriggerDef{
//	    Key:         "uptime.down",
//	    Label:       "Uptime monitor down",
//	    Description: "Fires when a monitored endpoint stops responding.",
//	    ConfigSchema: []webhooks.TriggerConfigField{
//	        {Key: "threshold_ms", Label: "Timeout (ms)", Type: "number", Default: "5000"},
//	    },
//	    RequiresCapability: "", // available on all nodes
//	})
//
// Register panics on duplicate key (fail-fast at startup). The ListWebhooks
// handler reads Registered() so every registered trigger surfaces automatically
// in the settings UI with no additional wiring.
type TriggerDef struct {
	// Key is the stable wire identifier stored in WebhookConfig.Triggers and
	// passed to Notify. Never change a key after it has shipped.
	Key string `json:"key"`

	// Label is the short human-readable name shown in the trigger list.
	Label string `json:"label"`

	// Description is one sentence explaining when this trigger fires.
	Description string `json:"description"`

	// ConfigSchema lists optional per-trigger configuration fields rendered by
	// the frontend as threshold / parameter inputs alongside the trigger toggle.
	// Empty slice means no extra configuration for this trigger.
	ConfigSchema []TriggerConfigField `json:"config_schema,omitempty"`

	// RequiresCapability, if non-empty, names a node capability
	// (matching a key in capabilities_json, e.g. "agent", "docker") that must
	// be present for this trigger to evaluate on that node. The dispatcher skips
	// nodes lacking the capability rather than erroring.
	RequiresCapability string `json:"requires_capability,omitempty"`
}

// TriggerConfigField describes one configuration input for a trigger. The
// frontend renders these as labelled form controls alongside the trigger toggle.
type TriggerConfigField struct {
	// Key identifies the field within the per-webhook trigger config map.
	Key string `json:"key"`
	// Label is the human-readable field label.
	Label string `json:"label"`
	// Type is a frontend hint: "number", "string", or "select".
	Type string `json:"type"`
	// Default is the pre-filled value when no stored config exists.
	Default string `json:"default"`
	// Options is only populated when Type == "select".
	Options []string `json:"options,omitempty"`
}

var (
	regMu    sync.RWMutex
	regSlice []TriggerDef
	regMap   = map[string]TriggerDef{}
)

// Register adds def to the global trigger registry.
// Panics if def.Key is empty or is already registered.
func Register(def TriggerDef) {
	if def.Key == "" {
		panic("webhooks.Register: key must not be empty")
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := regMap[def.Key]; dup {
		panic("webhooks.Register: duplicate trigger key: " + def.Key)
	}
	regSlice = append(regSlice, def)
	regMap[def.Key] = def
}

// Registered returns a snapshot of all registered TriggerDefs in insertion order.
func Registered() []TriggerDef {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]TriggerDef, len(regSlice))
	copy(out, regSlice)
	return out
}

// LookupTrigger returns the TriggerDef for key and whether it was found.
func LookupTrigger(key string) (TriggerDef, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	def, ok := regMap[key]
	return def, ok
}

// AllTriggerKeys returns all registered trigger keys in insertion order.
// This is what the ListWebhooks handler returns as available_triggers (for
// backwards-compat with callers that only need the key list).
func AllTriggerKeys() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, len(regSlice))
	for i, d := range regSlice {
		out[i] = d.Key
	}
	return out
}
