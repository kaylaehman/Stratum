// Package bulk holds the pure planning logic for bulk container operations
// (Feature 13): given an action and a set of containers, decide which will be
// acted on and which are skipped (e.g. remove only applies to stopped
// containers). Execution (the daemon calls) lives in the API layer, which has
// the docker clients; this package stays dependency-light and unit-testable.
package bulk

import "github.com/kaylaehman/stratum/backend/db"

// Actions.
const (
	ActionStart   = "start"
	ActionStop    = "stop"
	ActionRestart = "restart"
	ActionRemove  = "remove"
)

// Valid reports whether a is a supported bulk action.
func Valid(a string) bool {
	switch a {
	case ActionStart, ActionStop, ActionRestart, ActionRemove:
		return true
	default:
		return false
	}
}

// Item is one container's plan/result line.
type Item struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	NodeID      string `json:"node_id"`
	DockerID    string `json:"-"` // internal: the daemon id to act on
	Status      string `json:"status"`
	Skip        bool   `json:"skip"`
	SkipReason  string `json:"skip_reason,omitempty"`
}

// Plan annotates each container with whether the action will act on it or skip
// it. Only `remove` skips (running containers must be stopped first); the other
// actions never skip here — an already-in-state container surfaces as a conflict
// at execution time.
func Plan(action string, containers []db.Container) []Item {
	items := make([]Item, 0, len(containers))
	for _, c := range containers {
		it := Item{
			ContainerID: c.ID,
			Name:        c.Name,
			NodeID:      c.NodeID,
			DockerID:    c.DockerID,
			Status:      c.Status,
		}
		if action == ActionRemove && c.Status == "running" {
			it.Skip = true
			it.SkipReason = "container is running (stop it first)"
		}
		items = append(items, it)
	}
	return items
}
