package alertpolicy

import (
	"context"
	"errors"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

// --- fake store ---

type fakeStore struct {
	policies  []appdb.AlertPolicy
	delivered []appdb.AlertDelivery
	latest    appdb.AlertDelivery
	latestErr error
}

func (f *fakeStore) ListAlertPolicies(_ context.Context) ([]appdb.AlertPolicy, error) {
	return f.policies, nil
}

func (f *fakeStore) GetAlertPolicy(_ context.Context, id string) (appdb.AlertPolicy, error) {
	for _, p := range f.policies {
		if p.ID == id {
			return p, nil
		}
	}
	return appdb.AlertPolicy{}, appdb.ErrNotFound
}

func (f *fakeStore) CreateAlertPolicy(_ context.Context, p appdb.AlertPolicy) error {
	f.policies = append(f.policies, p)
	return nil
}

func (f *fakeStore) UpdateAlertPolicy(_ context.Context, p appdb.AlertPolicy) error {
	for i, ep := range f.policies {
		if ep.ID == p.ID {
			f.policies[i] = p
			return nil
		}
	}
	return appdb.ErrNotFound
}

func (f *fakeStore) DeleteAlertPolicy(_ context.Context, id string) error {
	for i, p := range f.policies {
		if p.ID == id {
			f.policies = append(f.policies[:i], f.policies[i+1:]...)
			return nil
		}
	}
	return appdb.ErrNotFound
}

func (f *fakeStore) InsertAlertDelivery(_ context.Context, d appdb.AlertDelivery) error {
	f.delivered = append(f.delivered, d)
	return nil
}

func (f *fakeStore) ListAlertDeliveries(_ context.Context, _ int) ([]appdb.AlertDelivery, error) {
	return f.delivered, nil
}

func (f *fakeStore) LatestDeliveryForKey(_ context.Context, _ string) (appdb.AlertDelivery, error) {
	return f.latest, f.latestErr
}

// --- helpers ---

func fixedTime(h, m int) time.Time {
	return time.Date(2025, 1, 1, h, m, 0, 0, time.UTC)
}

func infoAlert() Alert {
	return Alert{Key: "test/node1", Severity: "info", Title: "Test", Source: "cve"}
}

func newSvc(store *fakeStore, nowFn func() time.Time) *Service {
	svc := New(store)
	svc.now = nowFn
	return svc
}

// --- matchesSeverity ---

func TestMatchesSeverity(t *testing.T) {
	cases := []struct {
		min, alert string
		want       bool
	}{
		{"info", "info", true},
		{"info", "warning", true},
		{"info", "critical", true},
		{"warning", "info", false},
		{"warning", "warning", true},
		{"warning", "critical", true},
		{"critical", "info", false},
		{"critical", "warning", false},
		{"critical", "critical", true},
		{"info", "unknown", true},  // unknown severity passes
		{"unknown", "info", true},  // unknown min passes
	}
	for _, c := range cases {
		got := matchesSeverity(c.min, c.alert)
		if got != c.want {
			t.Errorf("matchesSeverity(%q, %q) = %v, want %v", c.min, c.alert, got, c.want)
		}
	}
}

// --- matchesFilter ---

func TestMatchesFilter(t *testing.T) {
	cases := []struct {
		name   string
		match  appdb.AlertPolicyMatch
		alert  Alert
		want   bool
	}{
		{
			name:  "empty filter matches all",
			match: appdb.AlertPolicyMatch{},
			alert: Alert{Source: "cve", Key: "cve/img1"},
			want:  true,
		},
		{
			name:  "source filter match",
			match: appdb.AlertPolicyMatch{Sources: []string{"cve"}},
			alert: Alert{Source: "cve", Key: "cve/img1"},
			want:  true,
		},
		{
			name:  "source filter no match",
			match: appdb.AlertPolicyMatch{Sources: []string{"port"}},
			alert: Alert{Source: "cve", Key: "cve/img1"},
			want:  false,
		},
		{
			name:  "key glob match",
			match: appdb.AlertPolicyMatch{KeyGlob: "cve/*"},
			alert: Alert{Source: "cve", Key: "cve/img1"},
			want:  true,
		},
		{
			name:  "key glob no match",
			match: appdb.AlertPolicyMatch{KeyGlob: "port/*"},
			alert: Alert{Source: "cve", Key: "cve/img1"},
			want:  false,
		},
		{
			name:  "source and key both match",
			match: appdb.AlertPolicyMatch{Sources: []string{"cve"}, KeyGlob: "cve/*"},
			alert: Alert{Source: "cve", Key: "cve/img1"},
			want:  true,
		},
		{
			name:  "source match but key mismatch",
			match: appdb.AlertPolicyMatch{Sources: []string{"cve"}, KeyGlob: "port/*"},
			alert: Alert{Source: "cve", Key: "cve/img1"},
			want:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchesFilter(c.match, c.alert)
			if got != c.want {
				t.Errorf("matchesFilter = %v, want %v", got, c.want)
			}
		})
	}
}

// --- inQuietHours ---

func TestInQuietHours(t *testing.T) {
	cases := []struct {
		name      string
		qh        appdb.AlertQuietHours
		nowHour   int
		nowMin    int
		want      bool
	}{
		{
			name: "inside window (no wrap)",
			qh:   appdb.AlertQuietHours{StartMin: 120, EndMin: 480, Tz: "UTC"}, // 02:00-08:00
			nowHour: 3, nowMin: 0,
			want: true,
		},
		{
			name: "outside window (no wrap)",
			qh:   appdb.AlertQuietHours{StartMin: 120, EndMin: 480, Tz: "UTC"},
			nowHour: 9, nowMin: 0,
			want: false,
		},
		{
			name: "inside window (wraps midnight)",
			qh:   appdb.AlertQuietHours{StartMin: 1380, EndMin: 60, Tz: "UTC"}, // 23:00-01:00
			nowHour: 23, nowMin: 30,
			want: true,
		},
		{
			name: "inside window (wraps midnight, early)",
			qh:   appdb.AlertQuietHours{StartMin: 1380, EndMin: 60, Tz: "UTC"},
			nowHour: 0, nowMin: 30,
			want: true,
		},
		{
			name: "outside window (wraps midnight)",
			qh:   appdb.AlertQuietHours{StartMin: 1380, EndMin: 60, Tz: "UTC"},
			nowHour: 2, nowMin: 0,
			want: false,
		},
		{
			name: "at start boundary",
			qh:   appdb.AlertQuietHours{StartMin: 120, EndMin: 480, Tz: "UTC"},
			nowHour: 2, nowMin: 0,
			want: true,
		},
		{
			name: "at end boundary (exclusive)",
			qh:   appdb.AlertQuietHours{StartMin: 120, EndMin: 480, Tz: "UTC"},
			nowHour: 8, nowMin: 0,
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			now := fixedTime(c.nowHour, c.nowMin)
			got := inQuietHours(c.qh, now)
			if got != c.want {
				t.Errorf("inQuietHours = %v, want %v", got, c.want)
			}
		})
	}
}

// --- Route: dedup window ---

func TestRoute_DedupWindow_SuppressesRepeat(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "dedup-test", Enabled: true,
			MinSeverity: "info", DedupWindowSec: 300, // 5 min
		}},
		// Last delivery was 2 minutes ago (within window)
		latest: appdb.AlertDelivery{
			ID:        "d0",
			PolicyID:  "p1",
			AlertKey:  "test/node1",
			Severity:  "info",
			Channel:   "",
			Status:    appdb.AlertDeliveryStatusDelivered,
			CreatedAt: baseTime.Add(-2 * time.Minute),
		},
		latestErr: nil,
	}

	svc := newSvc(store, func() time.Time { return baseTime })
	decisions, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Status != appdb.AlertDeliveryStatusSuppressedDedup {
		t.Errorf("expected suppressed_dedup, got %q", decisions[0].Status)
	}
	// Delivery should still be recorded.
	if len(store.delivered) != 1 {
		t.Errorf("expected 1 delivery row recorded, got %d", len(store.delivered))
	}
}

func TestRoute_DedupWindow_PassesAfterWindow(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "dedup-test", Enabled: true,
			MinSeverity: "info", DedupWindowSec: 300,
		}},
		// Last delivery was 10 minutes ago (outside window)
		latest: appdb.AlertDelivery{
			CreatedAt: baseTime.Add(-10 * time.Minute),
		},
	}

	svc := newSvc(store, func() time.Time { return baseTime })
	decisions, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Status != appdb.AlertDeliveryStatusDelivered {
		t.Errorf("expected delivered, got %+v", decisions)
	}
}

func TestRoute_DedupWindow_PassesWhenNoHistory(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "dedup-test", Enabled: true,
			MinSeverity: "info", DedupWindowSec: 300,
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, func() time.Time { return baseTime })
	decisions, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Status != appdb.AlertDeliveryStatusDelivered {
		t.Errorf("expected delivered (no history), got %+v", decisions)
	}
}

// --- Route: quiet hours ---

func TestRoute_QuietHours_SuppressesNonCritical(t *testing.T) {
	// 03:00 UTC — within 02:00-06:00 quiet window
	baseTime := fixedTime(3, 0)

	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "quiet", Enabled: true,
			MinSeverity: "info",
			QuietHours: &appdb.AlertQuietHours{
				StartMin: 120, EndMin: 360, Tz: "UTC", AllowCritical: false,
			},
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, func() time.Time { return baseTime })
	a := infoAlert()
	a.Severity = "warning"
	decisions, err := svc.Route(context.Background(), a)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Status != appdb.AlertDeliveryStatusSuppressedQuiet {
		t.Errorf("expected suppressed_quiet, got %+v", decisions)
	}
}

func TestRoute_QuietHours_AllowCriticalBypass(t *testing.T) {
	// 03:00 UTC — within quiet window; AllowCritical=true
	baseTime := fixedTime(3, 0)

	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "quiet-allow-crit", Enabled: true,
			MinSeverity: "info",
			QuietHours: &appdb.AlertQuietHours{
				StartMin: 120, EndMin: 360, Tz: "UTC", AllowCritical: true,
			},
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, func() time.Time { return baseTime })
	a := infoAlert()
	a.Severity = "critical"
	decisions, err := svc.Route(context.Background(), a)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Status != appdb.AlertDeliveryStatusDelivered {
		t.Errorf("expected delivered (critical bypass), got %+v", decisions)
	}
}

func TestRoute_QuietHours_OutsideWindow_Delivers(t *testing.T) {
	// 10:00 UTC — outside 02:00-06:00 window
	baseTime := fixedTime(10, 0)

	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "quiet-outside", Enabled: true,
			MinSeverity: "info",
			QuietHours: &appdb.AlertQuietHours{
				StartMin: 120, EndMin: 360, Tz: "UTC", AllowCritical: false,
			},
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, func() time.Time { return baseTime })
	decisions, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Status != appdb.AlertDeliveryStatusDelivered {
		t.Errorf("expected delivered (outside quiet window), got %+v", decisions)
	}
}

// --- Route: MinSeverity drops lower ---

func TestRoute_MinSeverity_DropsLower(t *testing.T) {
	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "critical-only", Enabled: true,
			MinSeverity: "critical",
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, time.Now)
	a := infoAlert()
	a.Severity = "warning"
	decisions, err := svc.Route(context.Background(), a)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 0 {
		t.Errorf("expected no decisions for warning below critical, got %+v", decisions)
	}
}

func TestRoute_MinSeverity_AdmitsCritical(t *testing.T) {
	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "critical-only", Enabled: true,
			MinSeverity: "critical",
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, time.Now)
	a := infoAlert()
	a.Severity = "critical"
	decisions, err := svc.Route(context.Background(), a)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 || decisions[0].Status != appdb.AlertDeliveryStatusDelivered {
		t.Errorf("expected delivered, got %+v", decisions)
	}
}

// --- Route: channel selection ---

func TestRoute_ChannelSelection(t *testing.T) {
	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "specific-channels", Enabled: true,
			MinSeverity: "info",
			Channels:    []string{"wh-slack", "wh-discord"},
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, time.Now)
	decisions, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 2 {
		t.Fatalf("expected 2 decisions (one per channel), got %d", len(decisions))
	}
	channels := map[string]bool{}
	for _, d := range decisions {
		channels[d.Channel] = true
		if d.Status != appdb.AlertDeliveryStatusDelivered {
			t.Errorf("expected delivered, got %q for channel %q", d.Status, d.Channel)
		}
	}
	if !channels["wh-slack"] || !channels["wh-discord"] {
		t.Errorf("expected both channels selected, got %v", channels)
	}
}

func TestRoute_DefaultPolicy_AllChannels(t *testing.T) {
	// Empty channels slice on the default policy → sentinel "" = all channels.
	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "default", Name: "Default", Enabled: true,
			MinSeverity: "info",
			Channels:    []string{}, // empty = all channels
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, time.Now)
	decisions, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Channel != "" {
		t.Errorf("expected empty channel (all channels sentinel), got %q", decisions[0].Channel)
	}
}

// --- Route: delivery recording ---

func TestRoute_DeliveriesRecorded(t *testing.T) {
	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "record-test", Enabled: true,
			MinSeverity: "info",
			Channels:    []string{"wh-1"},
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, time.Now)
	_, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(store.delivered) != 1 {
		t.Fatalf("expected 1 delivery recorded, got %d", len(store.delivered))
	}
	d := store.delivered[0]
	if d.PolicyID != "p1" {
		t.Errorf("PolicyID = %q, want p1", d.PolicyID)
	}
	if d.Channel != "wh-1" {
		t.Errorf("Channel = %q, want wh-1", d.Channel)
	}
	if d.AlertKey != "test/node1" {
		t.Errorf("AlertKey = %q, want test/node1", d.AlertKey)
	}
	if d.Status != appdb.AlertDeliveryStatusDelivered {
		t.Errorf("Status = %q, want delivered", d.Status)
	}
}

// --- Route: disabled policy is skipped ---

func TestRoute_DisabledPolicy_Skipped(t *testing.T) {
	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "disabled", Enabled: false,
			MinSeverity: "info",
		}},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, time.Now)
	decisions, err := svc.Route(context.Background(), infoAlert())
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	if len(decisions) != 0 {
		t.Errorf("expected no decisions for disabled policy, got %+v", decisions)
	}
}

// --- Route: DB error propagation ---

func TestRoute_StoreError_Propagates(t *testing.T) {
	// LatestDeliveryForKey returns a non-ErrNotFound error: Route must propagate it.
	store := &fakeStore{
		policies: []appdb.AlertPolicy{{
			ID: "p1", Name: "test", Enabled: true,
			MinSeverity: "info", DedupWindowSec: 60,
		}},
		latestErr: errors.New("db: connection lost"),
	}

	svc := newSvc(store, time.Now)
	_, err := svc.Route(context.Background(), infoAlert())
	if err == nil {
		t.Error("expected error propagation from store, got nil")
	}
}

// --- Multiple policies: each contributes independent decisions ---

func TestRoute_MultiplePolicies(t *testing.T) {
	store := &fakeStore{
		policies: []appdb.AlertPolicy{
			{ID: "p1", Name: "all", Enabled: true, MinSeverity: "info", Channels: []string{"wh-a"}},
			{ID: "p2", Name: "critical-only", Enabled: true, MinSeverity: "critical", Channels: []string{"wh-b"}},
		},
		latestErr: appdb.ErrNotFound,
	}

	svc := newSvc(store, time.Now)
	a := infoAlert()
	a.Severity = "warning"
	decisions, err := svc.Route(context.Background(), a)
	if err != nil {
		t.Fatalf("Route returned error: %v", err)
	}
	// Only p1 should match (p2 requires critical)
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].PolicyID != "p1" {
		t.Errorf("expected p1 decision, got %q", decisions[0].PolicyID)
	}
}
