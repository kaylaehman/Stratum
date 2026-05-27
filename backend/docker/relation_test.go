package docker

import "testing"

func TestRelation(t *testing.T) {
	cases := []struct {
		a, b string
		want Relationship
	}{
		{"/data", "/data", RelEqual},
		{"/data", "/data/app", RelAParentB},
		{"/data/app", "/data", RelBParentA},
		{"/data", "/data-archive", RelUnrelated}, // segment-aware
		{"/data/", "/data/app/", RelAParentB},     // trailing slashes cleaned
		{"/srv/media", "/home/kayla", RelUnrelated},
		{"/a/b/c", "/a/b", RelBParentA},
	}
	for _, c := range cases {
		if got := Relation(c.a, c.b); got != c.want {
			t.Errorf("Relation(%q,%q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}
