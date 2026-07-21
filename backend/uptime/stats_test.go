package uptime

import (
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

func makeResult(status CheckStatus, ago time.Duration, respMs int) db.UptimeResult {
	return db.UptimeResult{
		MonitorID:      "m1",
		CheckedAt:      time.Now().Add(-ago),
		Status:         string(status),
		ResponseTimeMs: respMs,
	}
}

func TestUptimePct_AllUp(t *testing.T) {
	results := []db.UptimeResult{
		makeResult(StatusUp, 1*time.Hour, 100),
		makeResult(StatusUp, 2*time.Hour, 120),
		makeResult(StatusUp, 23*time.Hour, 90),
	}
	pct := UptimePct(results, time.Now().Add(-24*time.Hour))
	if pct != 100.0 {
		t.Errorf("expected 100.0, got %.2f", pct)
	}
}

func TestUptimePct_AllDown(t *testing.T) {
	results := []db.UptimeResult{
		makeResult(StatusDown, 1*time.Hour, 0),
		makeResult(StatusDown, 2*time.Hour, 0),
	}
	pct := UptimePct(results, time.Now().Add(-24*time.Hour))
	if pct != 0.0 {
		t.Errorf("expected 0.0, got %.2f", pct)
	}
}

func TestUptimePct_Mixed(t *testing.T) {
	results := []db.UptimeResult{
		makeResult(StatusUp, 1*time.Hour, 100),
		makeResult(StatusDown, 2*time.Hour, 0),
		makeResult(StatusUp, 3*time.Hour, 110),
		makeResult(StatusDown, 4*time.Hour, 0),
	}
	pct := UptimePct(results, time.Now().Add(-24*time.Hour))
	// 2 up out of 4 = 50%
	if pct != 50.0 {
		t.Errorf("expected 50.0, got %.2f", pct)
	}
}

func TestUptimePct_DegradedCountsAsUp(t *testing.T) {
	results := []db.UptimeResult{
		makeResult(StatusDegraded, 1*time.Hour, 4000),
		makeResult(StatusDown, 2*time.Hour, 0),
	}
	pct := UptimePct(results, time.Now().Add(-24*time.Hour))
	// degraded = up: 1/2 = 50%
	if pct != 50.0 {
		t.Errorf("expected 50.0, got %.2f", pct)
	}
}

func TestUptimePct_Empty(t *testing.T) {
	pct := UptimePct(nil, time.Now().Add(-24*time.Hour))
	if pct != 100.0 {
		t.Errorf("expected 100.0 for empty, got %.2f", pct)
	}
}

func TestUptimePct_WindowFilter(t *testing.T) {
	results := []db.UptimeResult{
		makeResult(StatusUp, 1*time.Hour, 100),
		makeResult(StatusDown, 25*time.Hour, 0), // outside 24h window
	}
	pct := UptimePct(results, time.Now().Add(-24*time.Hour))
	if pct != 100.0 {
		t.Errorf("expected 100.0 (only 1 in-window result which is up), got %.2f", pct)
	}
}

func TestAvgResponseMs(t *testing.T) {
	results := []db.UptimeResult{
		makeResult(StatusUp, 1*time.Hour, 100),
		makeResult(StatusUp, 2*time.Hour, 200),
		makeResult(StatusUp, 3*time.Hour, 300),
	}
	avg := AvgResponseMs(results)
	if avg != 200.0 {
		t.Errorf("expected 200.0, got %.2f", avg)
	}
}

func TestAvgResponseMs_Empty(t *testing.T) {
	if avg := AvgResponseMs(nil); avg != 0.0 {
		t.Errorf("expected 0.0 for empty, got %.2f", avg)
	}
}

func TestComputeStats_CurrentStatus(t *testing.T) {
	results := []db.UptimeResult{
		makeResult(StatusUp, 2*time.Hour, 100),
		makeResult(StatusDown, 10*time.Minute, 0), // newest
	}
	stats := ComputeStats("m1", results)
	if stats.Current != StatusDown {
		t.Errorf("expected current=down, got %s", stats.Current)
	}
}

func TestComputeStats_EmptyIsDown(t *testing.T) {
	stats := ComputeStats("m1", nil)
	if stats.Current != StatusDown {
		t.Errorf("expected down for no results, got %s", stats.Current)
	}
}
