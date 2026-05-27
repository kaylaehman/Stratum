package logtail

import (
	"strings"
	"time"
)

// LogLine is one demuxed, parsed log line.
type LogLine struct {
	ContainerID string `json:"container_id"`
	Timestamp   string `json:"ts"`                  // RFC3339Nano (UTC) from docker, or "" if absent
	Stream      string `json:"stream"`              // "stdout" | "stderr"
	Text        string `json:"text"`
	Truncated   bool   `json:"truncated,omitempty"`
}

// MaxLineBytes is the per-line hard cap (16 KB).
const MaxLineBytes = 16 << 10

// truncationMarker is appended to lines that exceed MaxLineBytes.
const truncationMarker = "...[truncated]"

// parseLine splits a docker --timestamps line ("2026-01-02T15:04:05.000000000Z the message")
// into Timestamp + Text and truncates Text to MaxLineBytes (setting Truncated).
// A line with no leading RFC3339 timestamp keeps Timestamp "" and all of Text.
func parseLine(containerID, stream, raw string) LogLine {
	ts, text := splitTimestamp(raw)
	truncated := false
	if len(text) > MaxLineBytes {
		text = text[:MaxLineBytes-len(truncationMarker)] + truncationMarker
		truncated = true
	}
	return LogLine{
		ContainerID: containerID,
		Timestamp:   ts,
		Stream:      stream,
		Text:        text,
		Truncated:   truncated,
	}
}

// splitTimestamp attempts to parse a leading RFC3339Nano timestamp from s.
// Docker emits lines formatted as "<ts> <rest>" where <ts> is RFC3339Nano.
// If the prefix is a valid timestamp it is returned separately; otherwise
// ts is "" and text is the entire raw string.
func splitTimestamp(s string) (ts, text string) {
	// RFC3339Nano timestamps end at the first space character.
	idx := strings.IndexByte(s, ' ')
	if idx <= 0 {
		return "", s
	}
	candidate := s[:idx]
	_, err := time.Parse(time.RFC3339Nano, candidate)
	if err != nil {
		return "", s
	}
	rest := s[idx+1:]
	return candidate, rest
}
