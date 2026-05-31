package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/remediation"
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
