package api

import "testing"

func TestMatchesQuery(t *testing.T) {
	cases := []struct {
		q      string
		fields []string
		want   bool
	}{
		{"plex", []string{"Plex Media Server"}, true},   // case-insensitive substring
		{"MEDIA", []string{"plex media server"}, true},  // query case-insensitive too
		{"db", []string{"postgres", "db-prod"}, true},    // matches second field
		{"xyz", []string{"plex", "nginx"}, false},        // no match
		{"", []string{"anything"}, false},                // empty query never matches
		{"a", []string{}, false},                         // no fields
		{"prod", []string{"", "web-prod-01"}, true},      // skips empty field
	}
	for _, c := range cases {
		if got := matchesQuery(c.q, c.fields...); got != c.want {
			t.Errorf("matchesQuery(%q, %v) = %v, want %v", c.q, c.fields, got, c.want)
		}
	}
}
