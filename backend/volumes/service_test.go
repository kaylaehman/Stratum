package volumes

import "testing"

func TestStatusClassification(t *testing.T) {
	cases := []struct {
		refCount int64
		attached int
		want     string
	}{
		{2, 2, StatusAttached},  // daemon + mount index agree
		{1, 0, StatusAttached},  // daemon says attached even if mount index lagged
		{0, 1, StatusAttached},  // mount index sees an attachment df missed
		{0, 0, StatusUnused},    // genuinely unused
		{-1, 0, StatusUnknown},  // daemon couldn't report a refcount
		{-1, 2, StatusAttached}, // unknown refcount but mounts present => attached
	}
	for _, c := range cases {
		if got := status(c.refCount, c.attached); got != c.want {
			t.Errorf("status(ref=%d, attached=%d) = %q, want %q", c.refCount, c.attached, got, c.want)
		}
	}
}

func TestAppendUnique(t *testing.T) {
	s := appendUnique(nil, "a")
	s = appendUnique(s, "b")
	s = appendUnique(s, "a") // dup ignored
	if len(s) != 2 || s[0] != "a" || s[1] != "b" {
		t.Errorf("appendUnique = %v, want [a b]", s)
	}
}
