package incident_test

import (
	"context"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/incident"
)

// stubStore satisfies incident.Store with in-memory data.
type stubStore struct {
	nodes      []db.Node
	containers map[string][]db.Container
	activity   []db.ActivityEntry
	samples    map[string][]db.ResourceSample
	fileEvents map[string][]db.FileEvent
}

func (s *stubStore) ListNodes(_ context.Context) ([]db.Node, error) {
	return s.nodes, nil
}

func (s *stubStore) ListContainersByNode(_ context.Context, nodeID string) ([]db.Container, error) {
	return s.containers[nodeID], nil
}

func (s *stubStore) QueryActivityLog(_ context.Context, q db.ActivityQuery) ([]db.ActivityEntry, error) {
	var out []db.ActivityEntry
	for _, e := range s.activity {
		if q.From != nil && e.CreatedAt.Before(*q.From) {
			continue
		}
		if q.To != nil && e.CreatedAt.After(*q.To) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (s *stubStore) ListResourceSamples(_ context.Context, containerID string, from, to time.Time) ([]db.ResourceSample, error) {
	var out []db.ResourceSample
	for _, r := range s.samples[containerID] {
		if !r.SampledAt.Before(from) && !r.SampledAt.After(to) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *stubStore) ListFileEvents(_ context.Context, nodeID string, _ int) ([]db.FileEvent, error) {
	return s.fileEvents[nodeID], nil
}

// helpers

func ts(offset time.Duration) time.Time {
	return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(offset)
}

// TestBuildTimeline_Sort ensures entries are returned newest-first.
func TestBuildTimeline_Sort(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{"docker":true,"agent":false}`},
		},
		containers: map[string][]db.Container{
			"n1": {
				{ID: "c1", NodeID: "n1", Name: "web", Status: "dead", LastSeen: base.Add(-1 * time.Hour)},
				{ID: "c2", NodeID: "n1", Name: "db", Status: "exited", LastSeen: base.Add(-2 * time.Hour)},
			},
		},
		activity: []db.ActivityEntry{
			{ID: "a1", Action: "node.create", Result: "success", CreatedAt: base.Add(-30 * time.Minute)},
			{ID: "a2", Action: "fs.write", Result: "success", CreatedAt: base.Add(-3 * time.Hour)},
		},
	}
	q := incident.Query{
		From: base.Add(-4 * time.Hour),
		To:   base,
	}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected entries, got none")
	}
	// Verify descending order.
	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp.After(entries[i-1].Timestamp) {
			t.Errorf("entries[%d] (%s) is after entries[%d] (%s) — not sorted", i, entries[i].Timestamp, i-1, entries[i-1].Timestamp)
		}
	}
}

// TestBuildTimeline_CapabilityGate verifies that file events are only included
// when the node has agent=true.
func TestBuildTimeline_CapabilityGate(t *testing.T) {
	base := ts(0)

	tests := []struct {
		name        string
		caps        string
		fileEvents  []db.FileEvent
		wantFECount int
	}{
		{
			name: "agent_present",
			caps: `{"docker":false,"agent":true}`,
			fileEvents: []db.FileEvent{
				{ID: "fe1", NodeID: "n1", Path: "/etc/passwd", EventType: "modified", DetectedAt: base.Add(-1 * time.Hour)},
			},
			wantFECount: 1,
		},
		{
			name:        "agent_absent",
			caps:        `{"docker":false,"agent":false}`,
			fileEvents:  []db.FileEvent{{ID: "fe2", NodeID: "n1", Path: "/etc/passwd", EventType: "modified", DetectedAt: base.Add(-1 * time.Hour)}},
			wantFECount: 0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := &stubStore{
				nodes: []db.Node{
					{ID: "n1", CapabilitiesJSON: tc.caps},
				},
				fileEvents: map[string][]db.FileEvent{
					"n1": tc.fileEvents,
				},
			}
			q := incident.Query{From: base.Add(-2 * time.Hour), To: base}
			entries, err := incident.BuildTimeline(context.Background(), store, q)
			if err != nil {
				t.Fatalf("BuildTimeline: %v", err)
			}
			var feCount int
			for _, e := range entries {
				if e.Source == incident.SourceFileEvent {
					feCount++
				}
			}
			if feCount != tc.wantFECount {
				t.Errorf("got %d file-event entries, want %d", feCount, tc.wantFECount)
			}
		})
	}
}

// TestBuildTimeline_MetricSpikes verifies spike entries are emitted for
// samples over the CPU threshold.
func TestBuildTimeline_MetricSpikes(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{"docker":true,"agent":false}`},
		},
		containers: map[string][]db.Container{
			"n1": {{ID: "c1", NodeID: "n1", Name: "app", Status: "running", LastSeen: base.Add(-10 * time.Minute)}},
		},
		samples: map[string][]db.ResourceSample{
			"c1": {
				{ContainerID: "c1", NodeID: "n1", CPUPct: 95, SampledAt: base.Add(-45 * time.Minute)},
				{ContainerID: "c1", NodeID: "n1", CPUPct: 92, SampledAt: base.Add(-44 * time.Minute)},
			},
		},
	}
	q := incident.Query{From: base.Add(-2 * time.Hour), To: base}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	var spikeCount int
	for _, e := range entries {
		if e.Source == incident.SourceMetric {
			spikeCount++
		}
	}
	if spikeCount == 0 {
		t.Error("expected at least one metric spike entry")
	}
}

// TestBuildTimeline_NodeFilter ensures node_id filtering works.
func TestBuildTimeline_NodeFilter(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{"docker":true,"agent":false}`},
			{ID: "n2", CapabilitiesJSON: `{"docker":true,"agent":false}`},
		},
		containers: map[string][]db.Container{
			"n1": {{ID: "c1", NodeID: "n1", Name: "web", Status: "dead", LastSeen: base.Add(-1 * time.Hour)}},
			"n2": {{ID: "c2", NodeID: "n2", Name: "api", Status: "dead", LastSeen: base.Add(-1 * time.Hour)}},
		},
	}
	q := incident.Query{From: base.Add(-2 * time.Hour), To: base, NodeID: "n1"}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	for _, e := range entries {
		if e.NodeID != "" && e.NodeID != "n1" {
			t.Errorf("entry has node_id %q, want n1 only", e.NodeID)
		}
	}
}

// TestBuildTimeline_ContainerSeverity verifies severity mapping for container statuses.
func TestBuildTimeline_ContainerSeverity(t *testing.T) {
	cases := []struct {
		status  string
		wantSev incident.Severity
		wantIn  bool
	}{
		{"dead", incident.SeverityCritical, true},
		{"exited", incident.SeverityWarning, true},
		{"restarting", incident.SeverityWarning, true},
		{"running", "", false},
		{"paused", "", false},
		{"created", "", false},
	}
	base := ts(0)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.status, func(t *testing.T) {
			store := &stubStore{
				nodes: []db.Node{
					{ID: "n1", CapabilitiesJSON: `{"docker":true}`},
				},
				containers: map[string][]db.Container{
					"n1": {{ID: "c1", NodeID: "n1", Name: "x", Status: tc.status, LastSeen: base.Add(-30 * time.Minute)}},
				},
			}
			q := incident.Query{From: base.Add(-1 * time.Hour), To: base}
			entries, err := incident.BuildTimeline(context.Background(), store, q)
			if err != nil {
				t.Fatalf("BuildTimeline: %v", err)
			}
			var found *incident.Entry
			for i := range entries {
				if entries[i].Source == incident.SourceContainer {
					found = &entries[i]
					break
				}
			}
			if tc.wantIn && found == nil {
				t.Errorf("status %q: expected container entry, got none", tc.status)
			}
			if !tc.wantIn && found != nil {
				t.Errorf("status %q: expected no container entry, got one", tc.status)
			}
			if found != nil && found.Severity != tc.wantSev {
				t.Errorf("status %q: severity=%q, want %q", tc.status, found.Severity, tc.wantSev)
			}
		})
	}
}

// TestBuildTimeline_EmptyStore returns no entries (no panic).
func TestBuildTimeline_EmptyStore(t *testing.T) {
	store := &stubStore{}
	entries, err := incident.BuildTimeline(context.Background(), store, incident.Query{})
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	if entries == nil {
		entries = []incident.Entry{}
	}
	if len(entries) != 0 {
		t.Errorf("empty store: got %d entries, want 0", len(entries))
	}
}

// TestBuildTimeline_DefaultTimeWindow verifies that when From/To are zero the
// function does not panic and still returns a valid (possibly empty) result.
// This exercises the default-window path: To=now, From=now-24h.
func TestBuildTimeline_DefaultTimeWindow(t *testing.T) {
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{"docker":false,"agent":false}`},
		},
	}
	// Zero Query — defaults applied inside BuildTimeline.
	entries, err := incident.BuildTimeline(context.Background(), store, incident.Query{})
	if err != nil {
		t.Fatalf("BuildTimeline with zero Query: %v", err)
	}
	// No data in store; just verify no panic and sane empty result.
	if entries == nil {
		entries = []incident.Entry{}
	}
	_ = entries
}

// TestBuildTimeline_ActivitySeverityErrorResult verifies that an activity entry
// with result="error" maps to SeverityWarning.
func TestBuildTimeline_ActivitySeverityErrorResult(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{}`},
		},
		activity: []db.ActivityEntry{
			{ID: "a1", Action: "container.stop", Result: "error", CreatedAt: base.Add(-10 * time.Minute)},
		},
	}
	q := incident.Query{From: base.Add(-1 * time.Hour), To: base}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	var found *incident.Entry
	for i := range entries {
		if entries[i].Source == incident.SourceActivity {
			found = &entries[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected activity entry, got none")
	}
	if found.Severity != incident.SeverityWarning {
		t.Errorf("result=error: severity=%q, want warning", found.Severity)
	}
}

// TestBuildTimeline_ActivitySeveritySuccessResult verifies that a successful
// activity entry maps to SeverityInfo.
func TestBuildTimeline_ActivitySeveritySuccessResult(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{}`},
		},
		activity: []db.ActivityEntry{
			{ID: "a1", Action: "container.start", Result: "success", CreatedAt: base.Add(-10 * time.Minute)},
		},
	}
	q := incident.Query{From: base.Add(-1 * time.Hour), To: base}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	for _, e := range entries {
		if e.Source == incident.SourceActivity {
			if e.Severity != incident.SeverityInfo {
				t.Errorf("result=success: severity=%q, want info", e.Severity)
			}
			return
		}
	}
	t.Fatal("expected activity entry, got none")
}

// TestBuildTimeline_OutsideWindowExcluded verifies that container events and
// file-change events outside the query window are not included.
func TestBuildTimeline_OutsideWindowExcluded(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{"docker":true,"agent":true}`},
		},
		containers: map[string][]db.Container{
			// LastSeen is 5h ago — outside the 2h window.
			"n1": {{ID: "c1", NodeID: "n1", Name: "x", Status: "dead", LastSeen: base.Add(-5 * time.Hour)}},
		},
		fileEvents: map[string][]db.FileEvent{
			// DetectedAt is 3h ago — also outside the 2h window.
			"n1": {{ID: "fe1", NodeID: "n1", Path: "/etc/passwd", EventType: "modified", DetectedAt: base.Add(-3 * time.Hour)}},
		},
	}
	q := incident.Query{From: base.Add(-2 * time.Hour), To: base}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	for _, e := range entries {
		if e.Source == incident.SourceContainer || e.Source == incident.SourceFileEvent {
			t.Errorf("outside-window entry included: source=%q ts=%s", e.Source, e.Timestamp)
		}
	}
}

// TestBuildTimeline_UnknownNodeIDReturnsEmpty verifies that when a specific
// node_id is requested but does not exist, the result is empty (not an error).
func TestBuildTimeline_UnknownNodeIDReturnsEmpty(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{"docker":true}`},
		},
		containers: map[string][]db.Container{
			"n1": {{ID: "c1", NodeID: "n1", Name: "web", Status: "dead", LastSeen: base.Add(-30 * time.Minute)}},
		},
	}
	q := incident.Query{From: base.Add(-1 * time.Hour), To: base, NodeID: "nonexistent"}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline with unknown node: %v", err)
	}
	// Activity log is NOT filtered by node, so we may get activity entries.
	// Container/file entries for nonexistent node should be zero.
	for _, e := range entries {
		if (e.Source == incident.SourceContainer || e.Source == incident.SourceFileEvent) && e.NodeID != "nonexistent" {
			// Any container/file entries must have the requested node ID.
			t.Errorf("unexpected entry from node %q when filtering for nonexistent", e.NodeID)
		}
	}
}

// TestBuildTimeline_FileEventDeepLink verifies file-event entries carry the
// /resources deep-link path.
func TestBuildTimeline_FileEventDeepLink(t *testing.T) {
	base := ts(0)
	store := &stubStore{
		nodes: []db.Node{
			{ID: "n1", CapabilitiesJSON: `{"docker":false,"agent":true}`},
		},
		fileEvents: map[string][]db.FileEvent{
			"n1": {{ID: "fe1", NodeID: "n1", Path: "/etc/crontab", EventType: "modified", DetectedAt: base.Add(-30 * time.Minute)}},
		},
	}
	q := incident.Query{From: base.Add(-1 * time.Hour), To: base}
	entries, err := incident.BuildTimeline(context.Background(), store, q)
	if err != nil {
		t.Fatalf("BuildTimeline: %v", err)
	}
	for _, e := range entries {
		if e.Source == incident.SourceFileEvent {
			if e.DeepLink != "/resources" {
				t.Errorf("file event deep link = %q, want /resources", e.DeepLink)
			}
			if e.TargetType != "file" {
				t.Errorf("file event TargetType = %q, want file", e.TargetType)
			}
			return
		}
	}
	t.Fatal("expected file event entry, got none")
}
