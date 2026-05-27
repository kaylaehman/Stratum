package logtail

import (
	"strings"
	"testing"
)

func TestParseLine_WithRFC3339NanoTimestamp(t *testing.T) {
	raw := "2026-05-27T12:34:56.123456789Z hello world"
	got := parseLine("ctr1", "stdout", raw)

	if got.Timestamp != "2026-05-27T12:34:56.123456789Z" {
		t.Errorf("Timestamp = %q, want %q", got.Timestamp, "2026-05-27T12:34:56.123456789Z")
	}
	if got.Text != "hello world" {
		t.Errorf("Text = %q, want %q", got.Text, "hello world")
	}
	if got.ContainerID != "ctr1" {
		t.Errorf("ContainerID = %q, want ctr1", got.ContainerID)
	}
	if got.Stream != "stdout" {
		t.Errorf("Stream = %q, want stdout", got.Stream)
	}
	if got.Truncated {
		t.Error("Truncated should be false")
	}
}

func TestParseLine_NoTimestamp(t *testing.T) {
	raw := "just a plain message"
	got := parseLine("ctr2", "stderr", raw)

	if got.Timestamp != "" {
		t.Errorf("Timestamp = %q, want empty", got.Timestamp)
	}
	if got.Text != raw {
		t.Errorf("Text = %q, want %q", got.Text, raw)
	}
	if got.Truncated {
		t.Error("Truncated should be false for normal line")
	}
}

func TestParseLine_OversizeLine_Truncated(t *testing.T) {
	// Build a line that exceeds MaxLineBytes.
	oversize := strings.Repeat("x", MaxLineBytes+100)
	got := parseLine("ctr3", "stdout", oversize)

	if !got.Truncated {
		t.Fatal("expected Truncated = true for oversize line")
	}
	if len(got.Text) > MaxLineBytes {
		t.Errorf("Text length %d exceeds MaxLineBytes %d", len(got.Text), MaxLineBytes)
	}
	if !strings.HasSuffix(got.Text, truncationMarker) {
		t.Errorf("Text does not end with truncation marker, got suffix: %q", got.Text[len(got.Text)-20:])
	}
}

func TestParseLine_ExactlyMaxBytes_NotTruncated(t *testing.T) {
	exact := strings.Repeat("y", MaxLineBytes)
	got := parseLine("ctr4", "stdout", exact)

	if got.Truncated {
		t.Error("line of exactly MaxLineBytes should not be truncated")
	}
	if got.Text != exact {
		t.Errorf("Text was altered unexpectedly")
	}
}

func TestParseLine_TimestampWithOffset(t *testing.T) {
	// Docker can also emit +00:00 form.
	raw := "2026-05-27T12:34:56.000000001+00:00 msg with offset ts"
	got := parseLine("ctr5", "stdout", raw)

	if got.Timestamp == "" {
		t.Error("expected timestamp to be parsed")
	}
	if got.Text != "msg with offset ts" {
		t.Errorf("Text = %q", got.Text)
	}
}
