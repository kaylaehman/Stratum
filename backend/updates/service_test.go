package updates

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		local, remote string
		remoteFailed  bool
		want          string
	}{
		{"sha256:aaa", "sha256:aaa", false, StatusUpToDate},
		{"sha256:aaa", "sha256:bbb", false, StatusUpdateAvailable},
		{"", "sha256:bbb", false, StatusUnknown},        // locally-built (no repo digest)
		{"sha256:aaa", "", false, StatusUnknown},         // no remote digest
		{"sha256:aaa", "sha256:bbb", true, StatusUnknown}, // remote lookup failed
		{"", "", true, StatusUnknown},
	}
	for _, c := range cases {
		if got := Classify(c.local, c.remote, c.remoteFailed); got != c.want {
			t.Errorf("Classify(%q,%q,failed=%v) = %q, want %q", c.local, c.remote, c.remoteFailed, got, c.want)
		}
	}
}
