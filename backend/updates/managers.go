package updates

import (
	"sort"
	"strings"
)

// managers.go — detection of external container-management tools (Watchtower,
// Portainer) whose presence on a host can conflict with Stratum's own update /
// recreate actions. Surfaced as a warning on the Updates page and used by the
// auto_update_containers automation to skip hosts another tool already updates.

// ManagementTool is an external tool found running on a host.
type ManagementTool struct {
	Name string `json:"name"`
	// AutoUpdates is true when the tool updates containers on its own
	// (Watchtower), making a concurrent Stratum update an active conflict rather
	// than merely a second management UI (Portainer).
	AutoUpdates bool `json:"auto_updates"`
}

// managerPatterns maps an image substring to the tool it identifies. Matching is
// case-insensitive and substring-based so it catches the common image refs
// (containrrr/watchtower, portainer/portainer-ce, portainer/agent, …).
var managerPatterns = []struct {
	pattern     string
	name        string
	autoUpdates bool
}{
	{"watchtower", "watchtower", true},
	{"portainer", "portainer", false},
}

// DetectManagers returns the conflicting management tools found among the given
// container images, deduped by name and sorted for stable output. Returns an
// empty slice when none are present.
func DetectManagers(images []string) []ManagementTool {
	found := map[string]bool{}
	var out []ManagementTool
	for _, img := range images {
		lower := strings.ToLower(img)
		for _, m := range managerPatterns {
			if found[m.name] || !strings.Contains(lower, m.pattern) {
				continue
			}
			found[m.name] = true
			out = append(out, ManagementTool{Name: m.name, AutoUpdates: m.autoUpdates})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// HasAutoUpdater reports whether any detected tool updates containers on its own
// (i.e. a real conflict with a Stratum-driven update), so callers can skip a
// host rather than fight Watchtower over the same containers.
func HasAutoUpdater(tools []ManagementTool) bool {
	for _, t := range tools {
		if t.AutoUpdates {
			return true
		}
	}
	return false
}
