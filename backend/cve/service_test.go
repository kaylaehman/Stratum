package cve

import (
	"context"
	"errors"
	"testing"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/docker"
)

// errNoDocker is the sentinel returned by the stub provider when docker is unavailable.
var errNoDocker = errors.New("no docker in test")

// noDockerProvider always returns an error (no docker connection available).
func noDockerProvider(_ context.Context, _ string) (*docker.Client, error) {
	return nil, errNoDocker
}

// TestResolveDigestNeverEmpty is the regression test for the bug where
// resolveDigest returned "" when c.ImageID was empty (NULL in DB) and the
// Docker client was unavailable. An empty digest caused the frontend's
// useCVEDetail hook to skip the query (guarded by `digest.length > 0`), so
// scan results would show a non-zero vulnerability count with a permanently
// empty detail panel.
func TestResolveDigestNeverEmpty(t *testing.T) {
	svc := New(nil, noDockerProvider, NewScanner())

	cases := []struct {
		name      string
		container db.Container
		wantEmpty bool
	}{
		{
			name: "both image_id and image populated",
			container: db.Container{
				NodeID:  "node1",
				ImageID: "sha256:abc123",
				Image:   "nginx:latest",
			},
			wantEmpty: false,
		},
		{
			name: "image_id empty (NULL from DB), image populated — regression case",
			container: db.Container{
				NodeID:  "node1",
				ImageID: "",
				Image:   "nginx:latest",
			},
			wantEmpty: false,
		},
		{
			name: "image_id populated, image empty",
			container: db.Container{
				NodeID:  "node1",
				ImageID: "sha256:deadbeef",
				Image:   "",
			},
			wantEmpty: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := svc.resolveDigest(context.Background(), tc.container)
			isEmpty := d == ""
			if isEmpty != tc.wantEmpty {
				t.Errorf("resolveDigest() = %q, wantEmpty=%v", d, tc.wantEmpty)
			}
		})
	}
}

// TestResolveDigestFallbackOrder verifies priority: image_id before image ref.
func TestResolveDigestFallbackOrder(t *testing.T) {
	svc := New(nil, noDockerProvider, NewScanner())

	c := db.Container{
		NodeID:  "node1",
		ImageID: "sha256:localid",
		Image:   "myapp:latest",
	}
	d := svc.resolveDigest(context.Background(), c)
	if d != "sha256:localid" {
		t.Errorf("resolveDigest() = %q, want %q", d, "sha256:localid")
	}

	// With empty image_id, must fall back to image reference (not "").
	c.ImageID = ""
	d = svc.resolveDigest(context.Background(), c)
	if d != "myapp:latest" {
		t.Errorf("resolveDigest() empty image_id = %q, want %q", d, "myapp:latest")
	}
}
