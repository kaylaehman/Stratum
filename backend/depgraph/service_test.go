package depgraph

import "testing"

func TestMatchInternal(t *testing.T) {
	idx := map[string]string{
		"abcdef0123456789aaaa": "c1", // full docker id -> internal id
		"112233445566":         "c2", // already-short (12-char) docker id
	}
	cases := []struct {
		docker string
		want   string
	}{
		{"abcdef0123456789aaaa", "c1"},     // exact full
		{"abcdef012345", "c1"},             // short prefix of a full id
		{"112233445566", "c2"},             // exact short
		{"112233445566ffff", "c2"},         // full extends a stored short id
		{"deadbeef0000", ""},               // no match
		{"short", ""},                      // <12 chars, no exact -> no fuzzy match
	}
	for _, c := range cases {
		if got := matchInternal(idx, c.docker); got != c.want {
			t.Errorf("matchInternal(%q) = %q, want %q", c.docker, got, c.want)
		}
	}
}
