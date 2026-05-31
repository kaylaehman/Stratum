package docker

import "testing"

func TestNormalizeImageRef(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Bare name gets :latest appended.
		{"nginx", "nginx:latest"},
		// Official library image — bare.
		{"ubuntu", "ubuntu:latest"},
		// Already has a tag — unchanged.
		{"nginx:1.25", "nginx:1.25"},
		{"nginx:latest", "nginx:latest"},
		// Namespaced image without tag.
		{"jellyfin/jellyfin", "jellyfin/jellyfin:latest"},
		// Namespaced image with tag — unchanged.
		{"jellyfin/jellyfin:10.9.0", "jellyfin/jellyfin:10.9.0"},
		// Registry host with port — bare image gets :latest.
		{"registry.local:5000/myimage", "registry.local:5000/myimage:latest"},
		// Registry host with port and tag — unchanged.
		{"registry.local:5000/myimage:v2", "registry.local:5000/myimage:v2"},
		// Digest-pinned ref — unchanged.
		{"nginx@sha256:abc123", "nginx@sha256:abc123"},
		// Empty string — unchanged.
		{"", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got := NormalizeImageRef(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeImageRef(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
