package recreate

import "testing"

func TestPinnedRef(t *testing.T) {
	const dg = "sha256:abc123"
	cases := []struct {
		ref, digest, want string
	}{
		{"nginx:latest", dg, "nginx@" + dg},
		{"nginx", dg, "nginx@" + dg},
		{"jellyfin/jellyfin:10.9", dg, "jellyfin/jellyfin@" + dg},
		{"ghcr.io:443/org/app:v1", dg, "ghcr.io:443/org/app@" + dg},
		{"ghcr.io/org/app", dg, "ghcr.io/org/app@" + dg},
		{"nginx@" + dg, dg, "nginx@" + dg}, // already pinned: unchanged
		{"", dg, ""},                       // no ref -> can't pin
		{"nginx:latest", "", ""},           // no digest -> can't pin
	}
	for _, c := range cases {
		if got := pinnedRef(c.ref, c.digest); got != c.want {
			t.Errorf("pinnedRef(%q, %q) = %q, want %q", c.ref, c.digest, got, c.want)
		}
	}
}

func TestRepoOnly(t *testing.T) {
	cases := map[string]string{
		"nginx:latest":            "nginx",
		"nginx":                   "nginx",
		"jellyfin/jellyfin:10.9":  "jellyfin/jellyfin",
		"ghcr.io:443/org/app:v1":  "ghcr.io:443/org/app",
		"ghcr.io:443/org/app":     "ghcr.io:443/org/app",
		"registry.local:5000/img": "registry.local:5000/img",
	}
	for in, want := range cases {
		if got := repoOnly(in); got != want {
			t.Errorf("repoOnly(%q) = %q, want %q", in, got, want)
		}
	}
}
