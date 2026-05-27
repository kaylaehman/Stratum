package api

import (
	"testing"
	"time"
)

func TestParseOptTime(t *testing.T) {
	if v, ok := parseOptTime("", false); !ok || v != nil {
		t.Errorf("empty: got %v %v, want nil true", v, ok)
	}
	if _, ok := parseOptTime("not-a-date", false); ok {
		t.Error("garbage date should not parse")
	}

	// Date-only `from` is the start instant (midnight).
	from, ok := parseOptTime("2026-05-27", false)
	if !ok || from == nil || !from.Equal(time.Date(2026, 5, 27, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("date-only from = %v, want 2026-05-27T00:00:00Z", from)
	}

	// Date-only `to` must cover the whole day (end-of-day), else a same-day
	// filter would exclude everything after midnight.
	to, ok := parseOptTime("2026-05-27", true)
	if !ok || to == nil {
		t.Fatalf("date-only to failed to parse")
	}
	wantEnd := time.Date(2026, 5, 27, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if !to.Equal(wantEnd) {
		t.Errorf("date-only to = %v, want end-of-day %v", to, wantEnd)
	}

	// A full RFC3339 timestamp is used verbatim regardless of endOfDay.
	rfc, ok := parseOptTime("2026-05-27T08:30:00Z", true)
	if !ok || !rfc.Equal(time.Date(2026, 5, 27, 8, 30, 0, 0, time.UTC)) {
		t.Errorf("rfc3339 to = %v", rfc)
	}
}
