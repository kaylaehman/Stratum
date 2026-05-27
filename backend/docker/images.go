package docker

import (
	"context"
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
