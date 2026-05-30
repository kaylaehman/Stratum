package docker

import (
	"context"
	"io"
	"strings"
)

// LocalRepoDigest returns the content-addressable repo digest of a locally
// present image (e.g. "sha256:abc..."), via ImageInspect. Returns "" when the
// image has no repo digest (e.g. built locally, never pushed/pulled).
func (c *Client) LocalRepoDigest(ctx context.Context, imageID string) (string, error) {
	insp, err := c.cli.ImageInspect(ctx, imageID)
	if err != nil {
		return "", err
	}
	return digestFromRepoDigests(insp.RepoDigests), nil
}

// HasRepoDigest reports whether the given image (by id or ref) carries a registry
// repo digest. A locally-built image that was never pushed/pulled has none, which
// means `trivy image <ref>` would fall back to a registry pull and fail — such
// images must be scanned via an exported tarball instead (see ImageSave).
func (c *Client) HasRepoDigest(ctx context.Context, imageRef string) (bool, error) {
	insp, err := c.cli.ImageInspect(ctx, imageRef)
	if err != nil {
		return false, err
	}
	return len(insp.RepoDigests) > 0, nil
}

// ImageSave exports the named image (id, name, or ref) as a Docker-format tar
// stream via the Engine API `GET /images/get`. The caller MUST Close the
// returned reader. This is used to scan locally-built/unpublished images that a
// registry pull cannot reach.
//
// NOTE: behind a restricted socket-proxy (tecnativa/docker-socket-proxy style),
// image export requires the proxy to allow GET /images/{name}/get — i.e. the
// IMAGES permission must be enabled. A read-only proxy that blocks it will make
// local-image scanning unavailable while registry scans still work.
func (c *Client) ImageSave(ctx context.Context, imageRef string) (io.ReadCloser, error) {
	rc, err := c.cli.ImageSave(ctx, []string{imageRef})
	if err != nil {
		return nil, err
	}
	return rc, nil
}

// digestFromRepoDigests extracts the "sha256:..." portion from the first
// "repo@sha256:..." entry.
func digestFromRepoDigests(repoDigests []string) string {
	for _, rd := range repoDigests {
		if i := strings.LastIndex(rd, "@"); i >= 0 {
			return rd[i+1:]
		}
	}
	return ""
}

// RemoteDigest queries the registry for an image reference's current manifest
// digest WITHOUT pulling it (anonymous; private registries that require auth
// return an error, surfaced by the caller as "unknown"). ref is the image
// reference as configured (e.g. "jellyfin/jellyfin:latest").
func (c *Client) RemoteDigest(ctx context.Context, ref string) (string, error) {
	di, err := c.cli.DistributionInspect(ctx, ref, "")
	if err != nil {
		return "", err
	}
	return di.Descriptor.Digest.String(), nil
}
