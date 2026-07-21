// Package incident implements the "What changed?" incident timeline.
// It merges four read-only event sources — activity-log mutations,
// container status observations, metric spikes, and file-change events —
// into a single time-ordered stream. No new data is collected. Sources that
// are unavailable (capability gate or empty table) are silently skipped so a
// missing capability never fails the whole timeline.
//
// # Event sources and capability gates
//
//   Source           Capability gate  Severity rules
//   ──────────────────────────────────────────────────────────────────────
//   activity         none             result="error" → warning; else info
//   container        docker=true      status="dead" → critical; "exited"|"restarting" → warning
//   metric spike     docker=true      CPU >80% or RAM >90% for 15 min → warning
//   file event       agent=true       any inotify event in watched paths → warning
//
// # Default time window
//
// When Query.From is zero, it defaults to 24 hours before Query.To (which
// itself defaults to time.Now()). Callers may narrow the window with explicit
// From/To values.
//
// # Node filtering
//
// When Query.NodeID is set, only events from that node are returned for
// container/metric/file sources. Activity-log entries are always included
// (they are not tagged by node at the store layer).
//
// See merge_test.go for table-driven tests covering all sources and edge cases.
package incident

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/metrics"
)

// SourceType identifies which data source produced a timeline entry.
type SourceType string

const (
	SourceActivity  SourceType = "activity"
	SourceContainer SourceType = "container"
	SourceMetric    SourceType = "metric"
	SourceFileEvent SourceType = "file_event"
)

// Severity mirrors common alert levels.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Entry is one normalized timeline event.
type Entry struct {
	Timestamp  time.Time  `json:"timestamp"`
	Source     SourceType `json:"source"`
	Severity   Severity   `json:"severity"`
	NodeID     string     `json:"node_id,omitempty"`
	TargetID   string     `json:"target_id,omitempty"`
	TargetType string     `json:"target_type,omitempty"`
	Summary    string     `json:"summary"`
	DeepLink   string     `json:"deep_link,omitempty"`
}

// Query holds filter parameters for BuildTimeline.
type Query struct {
	From   time.Time
	To     time.Time
	NodeID string // empty = all nodes
}

// Store is the minimal read interface the incident package needs from db.Store.
type Store interface {
	ListNodes(ctx context.Context) ([]db.Node, error)
	ListContainersByNode(ctx context.Context, nodeID string) ([]db.Container, error)
	QueryActivityLog(ctx context.Context, q db.ActivityQuery) ([]db.ActivityEntry, error)
	ListResourceSamples(ctx context.Context, containerID string, from, to time.Time) ([]db.ResourceSample, error)
	ListFileEvents(ctx context.Context, nodeID string, limit int) ([]db.FileEvent, error)
}

// nodeCapabilities is the subset of capabilities_json we need.
type nodeCapabilities struct {
	Docker bool `json:"docker"`
	Agent  bool `json:"agent"`
}

func parseCapabilities(raw string) nodeCapabilities {
	var c nodeCapabilities
	_ = json.Unmarshal([]byte(raw), &c)
	return c
}

// BuildTimeline merges all available event sources into one sorted slice.
// Individual source errors are silently dropped (graceful degradation).
func BuildTimeline(ctx context.Context, store Store, q Query) ([]Entry, error) {
	if q.To.IsZero() {
		q.To = time.Now()
	}
	if q.From.IsZero() {
		q.From = q.To.Add(-24 * time.Hour)
	}

	nodes, _ := resolveNodes(ctx, store, q.NodeID)

	var entries []Entry
	entries = append(entries, fromActivity(ctx, store, q)...)

	for _, n := range nodes {
		caps := parseCapabilities(n.CapabilitiesJSON)
		if caps.Docker {
			entries = append(entries, fromContainers(ctx, store, n.ID, q)...)
			entries = append(entries, fromMetrics(ctx, store, n.ID, q)...)
		}
		if caps.Agent {
			entries = append(entries, fromFileEvents(ctx, store, n.ID, q)...)
		}
	}

	sortByTime(entries)
	return entries, nil
}

func resolveNodes(ctx context.Context, store Store, nodeID string) ([]db.Node, error) {
	all, err := store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	if nodeID == "" {
		return all, nil
	}
	for _, n := range all {
		if n.ID == nodeID {
			return []db.Node{n}, nil
		}
	}
	return nil, nil
}

func fromActivity(ctx context.Context, store Store, q Query) []Entry {
	rows, err := store.QueryActivityLog(ctx, db.ActivityQuery{
		From:  &q.From,
		To:    &q.To,
		Limit: 500,
	})
	if err != nil {
		return nil
	}
	out := make([]Entry, 0, len(rows))
	for _, r := range rows {
		e := Entry{
			Timestamp:  r.CreatedAt,
			Source:     SourceActivity,
			Severity:   activitySeverity(r.Result),
			Summary:    r.Action,
			TargetType: ptrStr(r.TargetType),
			TargetID:   ptrStr(r.TargetID),
			DeepLink:   activityDeepLink(ptrStr(r.TargetType)),
		}
		out = append(out, e)
	}
	return out
}

func activitySeverity(result string) Severity {
	if result == "error" {
		return SeverityWarning
	}
	return SeverityInfo
}

func activityDeepLink(targetType string) string {
	switch targetType {
	case "container":
		return "/resources"
	case "node":
		return "/nodes"
	case "file":
		return "/resources"
	default:
		return "/activity"
	}
}

func fromContainers(ctx context.Context, store Store, nodeID string, q Query) []Entry {
	containers, err := store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return nil
	}
	var out []Entry
	for _, c := range containers {
		if c.LastSeen.Before(q.From) || c.LastSeen.After(q.To) {
			continue
		}
		sev, ok := containerSeverity(c.Status)
		if !ok {
			continue
		}
		out = append(out, Entry{
			Timestamp:  c.LastSeen,
			Source:     SourceContainer,
			Severity:   sev,
			NodeID:     nodeID,
			TargetID:   c.ID,
			TargetType: "container",
			Summary:    "container " + c.Name + " status: " + c.Status,
			DeepLink:   "/resources",
		})
	}
	return out
}

// containerSeverity maps non-running container statuses to severity.
// Returns ok=false for statuses that are not incident-worthy.
func containerSeverity(status string) (Severity, bool) {
	switch status {
	case "dead":
		return SeverityCritical, true
	case "exited", "restarting":
		return SeverityWarning, true
	default:
		return "", false
	}
}

func fromMetrics(ctx context.Context, store Store, nodeID string, q Query) []Entry {
	containers, err := store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return nil
	}
	var out []Entry
	for _, c := range containers {
		samples, err := store.ListResourceSamples(ctx, c.ID, q.From, q.To)
		if err != nil || len(samples) == 0 {
			continue
		}
		for _, spike := range metrics.DetectSpikes(samples) {
			if spike.From.Before(q.From) || spike.From.After(q.To) {
				continue
			}
			out = append(out, Entry{
				Timestamp:  spike.From,
				Source:     SourceMetric,
				Severity:   SeverityWarning,
				NodeID:     nodeID,
				TargetID:   c.ID,
				TargetType: "container",
				Summary:    spikeSummary(c.Name, spike),
				DeepLink:   "/metrics",
			})
		}
	}
	return out
}

func fromFileEvents(ctx context.Context, store Store, nodeID string, q Query) []Entry {
	events, err := store.ListFileEvents(ctx, nodeID, 500)
	if err != nil {
		return nil
	}
	var out []Entry
	for _, e := range events {
		if e.DetectedAt.Before(q.From) || e.DetectedAt.After(q.To) {
			continue
		}
		out = append(out, Entry{
			Timestamp:  e.DetectedAt,
			Source:     SourceFileEvent,
			Severity:   SeverityWarning,
			NodeID:     nodeID,
			TargetType: "file",
			TargetID:   e.Path,
			Summary:    "file " + e.EventType + ": " + e.Path,
			DeepLink:   "/resources",
		})
	}
	return out
}

func sortByTime(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
}

func spikeSummary(containerName string, s metrics.Spike) string {
	if s.Metric == "cpu" {
		return fmt.Sprintf("cpu spike on %s (%.0f%%)", containerName, s.Peak)
	}
	return "memory spike on " + containerName
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
