package scheduler

import "strings"

// SystemdTimer is one parsed `systemctl list-timers` row.
type SystemdTimer struct {
	Unit     string `json:"unit"`
	Activates string `json:"activates"` // the service it triggers
	Next     string `json:"next"`
	Last     string `json:"last"`
}

// ParseTimers parses `systemctl list-timers --all --no-pager` output. The table
// columns are: NEXT LEFT LAST PASSED UNIT ACTIVATES. NEXT/LAST are multi-word
// timestamps, so we anchor on the trailing UNIT (ends ".timer") + ACTIVATES.
func ParseTimers(output string) []SystemdTimer {
	timers := []SystemdTimer{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		// Find the UNIT field (the token ending in ".timer").
		unitIdx := -1
		for i, f := range fields {
			if strings.HasSuffix(f, ".timer") {
				unitIdx = i
				break
			}
		}
		if unitIdx < 0 {
			continue // header / "N timers listed." / non-row
		}
		t := SystemdTimer{Unit: fields[unitIdx]}
		if unitIdx+1 < len(fields) {
			t.Activates = fields[unitIdx+1]
		}
		// NEXT is fields[0..]; "n/a" timers show a leading dash. Capture a coarse
		// next/last by joining tokens before the unit, split at the LAST/PASSED
		// boundary is unreliable across locales, so just surface the leading
		// tokens as a combined "schedule" hint.
		lead := fields[:unitIdx]
		t.Next, t.Last = splitNextLast(lead)
		timers = append(timers, t)
	}
	return timers
}

// splitNextLast makes a coarse best-effort split of the leading timestamp
// tokens. `systemctl list-timers` prints: NEXT(3 tokens) LEFT(2) LAST(3) PASSED(2).
// Locale/format variance makes exact parsing brittle, so we take the first ~5
// tokens as "next" context and the remainder as "last".
func splitNextLast(lead []string) (next, last string) {
	if len(lead) == 0 {
		return "", ""
	}
	cut := 5
	if len(lead) < cut {
		cut = len(lead)
	}
	next = strings.Join(lead[:cut], " ")
	if len(lead) > cut {
		last = strings.Join(lead[cut:], " ")
	}
	return next, last
}
