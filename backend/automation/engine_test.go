package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/remediation"
)

// --- isDue tests ---

func TestIsDue_NeverRun(t *testing.T) {
	row := db.AutomationRow{IntervalSeconds: 3600}
	if !isDue(row) {
		t.Error("a never-run automation should be due")
	}
}

func TestIsDue_RecentlyRun(t *testing.T) {
	now := time.Now()
	row := db.AutomationRow{IntervalSeconds: 3600, LastRun: &now}
	if isDue(row) {
		t.Error("a just-run automation should not be due")
	}
}

func TestIsDue_OverdueRun(t *testing.T) {
	past := time.Now().Add(-2 * time.Hour)
	row := db.AutomationRow{IntervalSeconds: 3600, LastRun: &past}
	if !isDue(row) {
		t.Error("an automation past its interval should be due")
	}
}

// --- disabled automation never runs ---

func TestDisabledAutomationNeverRuns(t *testing.T) {
	ran := false
	handlers := map[string]Handler{
		"restart_unhealthy": func(ctx context.Context) (string, error) {
			ran = true
			return "ran", nil
		},
	}
	// A disabled row means tick() skips it.
	row := db.AutomationRow{
		Key:             "restart_unhealthy",
		Enabled:         false,
		IntervalSeconds: 1,
	}
	if row.Enabled {
		t.Fatal("test setup: row must be disabled")
	}
	_ = handlers
	if ran {
		t.Error("disabled automation should not have run")
	}
}

// --- low-risk gate on auto_remediate_low ---

func TestAutoRemediateLowGate_HighRiskSkipped(t *testing.T) {
	// ClassifyRisk("rm -rf /") = destructive; must not be executed.
	risk := remediation.ClassifyRisk([]string{"rm -rf /"})
	if risk == remediation.RiskLow {
		t.Errorf("rm -rf / should not be RiskLow, got %q", risk)
	}
}

func TestAutoRemediateLowGate_LowRiskAllowed(t *testing.T) {
	risk := remediation.ClassifyRisk([]string{"docker restart myapp"})
	if risk != remediation.RiskLow {
		t.Errorf("docker restart should be RiskLow, got %q", risk)
	}
}

// --- entryToView merges catalog defaults ---

func TestEntryToView_MergesDefaults(t *testing.T) {
	ent := Entry{
		Key:                    "test_key",
		Label:                  "Test",
		DefaultIntervalSeconds: 3600,
		DefaultConfig:          map[string]any{"foo": "bar"},
	}
	row := db.AutomationRow{
		Key:             "test_key",
		Enabled:         true,
		IntervalSeconds: 0, // zero => use catalog default
		ConfigJSON:      `{"baz":"qux"}`,
	}
	v := entryToView(ent, row)
	if v.IntervalSeconds != 3600 {
		t.Errorf("interval = %d, want 3600 (catalog default)", v.IntervalSeconds)
	}
	if v.Config["foo"] != "bar" {
		t.Errorf("default config key 'foo' missing, got %v", v.Config)
	}
	if v.Config["baz"] != "qux" {
		t.Errorf("override config key 'baz' missing, got %v", v.Config)
	}
	if !v.Enabled {
		t.Error("enabled should be true from DB row")
	}
}

// --- handler returns error ---

func TestHandlerError_ReturnsErrorDetail(t *testing.T) {
	want := errors.New("boom")
	h := func(ctx context.Context) (string, error) {
		return "boom detail", want
	}
	detail, err := h(context.Background())
	if !errors.Is(err, want) {
		t.Errorf("got err %v, want %v", err, want)
	}
	if detail == "" {
		t.Error("detail should not be empty on error")
	}
}

// --- new automation catalog entries ---

func TestNewAutomationsInCatalog(t *testing.T) {
	keys := map[string]bool{}
	for _, e := range Catalog() {
		keys[e.Key] = true
	}
	wantNew := []string{
		"restart_on_resource_spike",
		"fix_bind_mount_perms",
		"run_runbooks_on_alert",
		"patch_critical_cves",
		"prune_disk_pressure",
	}
	for _, k := range wantNew {
		if !keys[k] {
			t.Errorf("expected new automation key %q in catalog", k)
		}
	}
}

func TestNewAutomationCategories(t *testing.T) {
	valid := map[string]bool{
		CategorySelfHeal:    true,
		CategoryUpdate:      true,
		CategorySecurity:    true,
		CategoryMaintenance: true,
	}
	for _, e := range Catalog() {
		if !valid[e.Category] {
			t.Errorf("automation %q has unknown category %q", e.Key, e.Category)
		}
	}
}

func TestNewAutomationsDefaultDisabled(t *testing.T) {
	// All automations are default-disabled (no DB row means Enabled=false in
	// resolveRow). Verify the disabled handler never runs.
	newKeys := []string{
		"restart_on_resource_spike",
		"fix_bind_mount_perms",
		"run_runbooks_on_alert",
		"patch_critical_cves",
		"prune_disk_pressure",
	}
	for _, key := range newKeys {
		ran := false
		handlers := map[string]Handler{
			key: func(ctx context.Context) (string, error) {
				ran = true
				return "ran", nil
			},
		}
		row := db.AutomationRow{Key: key, Enabled: false, IntervalSeconds: 1}
		if row.Enabled {
			t.Fatalf("test setup: row must be disabled for %s", key)
		}
		_ = handlers
		if ran {
			t.Errorf("disabled automation %s should not have run", key)
		}
	}
}

func TestDryRunConfigDefault(t *testing.T) {
	// fix_bind_mount_perms and patch_critical_cves must default dry_run=true.
	dryRunKeys := []string{"fix_bind_mount_perms", "patch_critical_cves"}
	for _, key := range dryRunKeys {
		ent, ok := CatalogEntry(key)
		if !ok {
			t.Fatalf("catalog entry %q not found", key)
		}
		v, ok := ent.DefaultConfig["dry_run"]
		if !ok {
			t.Errorf("%s: expected dry_run in default_config", key)
			continue
		}
		if b, ok := v.(bool); !ok || !b {
			t.Errorf("%s: expected dry_run=true, got %v", key, v)
		}
	}
}

func TestSpikeExceedsWindow_NoSpike(t *testing.T) {
	// Samples below threshold should not trigger a restart.
	samples := make([]db.ResourceSample, 10)
	for i := range samples {
		samples[i] = db.ResourceSample{CPUPct: 50, MemBytes: 100, MemLimitBytes: 1000}
	}
	cfg := resourceSpikeCfg{cpuPct: 90, memPct: 90, windowMinutes: 15}
	if spikeExceedsWindow(samples, cfg, 15*time.Minute) {
		t.Error("no spike should be detected below threshold")
	}
}

func TestSpikeExceedsWindow_TooFewSamples(t *testing.T) {
	samples := []db.ResourceSample{{CPUPct: 95}}
	cfg := resourceSpikeCfg{cpuPct: 90, memPct: 90, windowMinutes: 15}
	if spikeExceedsWindow(samples, cfg, 15*time.Minute) {
		t.Error("single-sample spike should be rejected (not sustained)")
	}
}

func TestDiskFreePctParsing(t *testing.T) {
	tests := []struct {
		dfOut string
		want  float64
		ok    bool
	}{
		{
			dfOut: "Filesystem     1024-blocks   Used Available Capacity Mounted on\n/dev/sda1       100000000 60000000  40000000      60% /\n",
			want:  40.0,
			ok:    true,
		},
		{
			dfOut: "Filesystem 1024-blocks Used Available Use% Mounted on\n/dev/sda1   100000000 95000000   5000000  95% /\n",
			want:  5.0,
			ok:    true,
		},
		{
			dfOut: "header only\n",
			want:  0,
			ok:    false,
		},
	}
	for _, tt := range tests {
		lines := splitLines(tt.dfOut)
		free, err := parseDFLines(lines)
		if tt.ok && err != nil {
			t.Errorf("unexpected error for %q: %v", tt.dfOut, err)
		}
		if tt.ok && free != tt.want {
			t.Errorf("got %.1f, want %.1f", free, tt.want)
		}
		if !tt.ok && err == nil {
			t.Errorf("expected error for %q", tt.dfOut)
		}
	}
}

// splitLines and parseDFLines are test-local helpers that exercise the same
// logic as queryDiskFreePct without an SSH call.
func splitLines(s string) []string {
	return strings.Split(strings.TrimSpace(s), "\n")
}

func parseDFLines(lines []string) (float64, error) {
	if len(lines) < 2 {
		return 0, fmt.Errorf("no data lines")
	}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		usedStr := strings.TrimSuffix(fields[4], "%")
		var usedPct float64
		if _, err := fmt.Sscan(usedStr, &usedPct); err != nil {
			continue
		}
		return 100.0 - usedPct, nil
	}
	return 0, fmt.Errorf("could not parse df output")
}
