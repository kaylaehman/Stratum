package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
)

// RecreateSpec is the full, faithful create-time configuration of a container,
// captured from inspect so the container can be reproduced (Feature 15/17). It
// is serialised verbatim into a rollback snapshot; the embedded SDK types
// round-trip through JSON cleanly.
type RecreateSpec struct {
	Name       string                               `json:"name"` // without leading slash
	Config     *container.Config                    `json:"config"`
	HostConfig *container.HostConfig                `json:"host_config"`
	Endpoints  map[string]*network.EndpointSettings `json:"endpoints,omitempty"`
}

// CaptureSpec inspects a container and returns everything needed to recreate it.
//
// Fidelity caveat: this is an inspect-and-replay recreate, not a compose redeploy.
// Named volumes and bind mounts (the data that matters) are preserved because
// ApplySpec never removes volumes. However, ANONYMOUS volumes (declared via
// VOLUME in the image with no host binding) are tied to the old container id and
// are discarded when the backup is removed — the recreated container gets a
// fresh empty anonymous volume. Likewise, a few HostConfig fields captured from a
// running container (resolved device majors, deprecated MacAddress) replay
// verbatim and could, on an upgraded host, cause ContainerCreate to reject the
// spec (which ApplySpec surfaces as an error with the original left intact).
func (c *Client) CaptureSpec(ctx context.Context, id string) (*RecreateSpec, error) {
	r, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.Config == nil || r.HostConfig == nil {
		return nil, errors.New("docker: inspect missing config/host_config")
	}
	spec := &RecreateSpec{
		Name:       trimSlash(r.Name),
		Config:     r.Config,
		HostConfig: r.HostConfig,
	}
	if r.NetworkSettings != nil && len(r.NetworkSettings.Networks) > 0 {
		spec.Endpoints = r.NetworkSettings.Networks
	}
	return spec, nil
}

func trimSlash(s string) string {
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}

// PullImage pulls an image reference and drains the progress stream (so the
// pull is complete before we return). Anonymous pull; private registries
// requiring auth will error.
func (c *Client) PullImage(ctx context.Context, ref string) error {
	rc, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer rc.Close()
	// Draining to EOF is what makes the pull synchronous.
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return err
	}
	return nil
}

// CurrentImageDigest returns the repo digest of the image a container is
// currently running, used to pin a rollback snapshot to an exact image.
func (c *Client) CurrentImageDigest(ctx context.Context, containerID string) (string, error) {
	insp, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}
	return c.LocalRepoDigest(ctx, insp.Image)
}

// ApplySpec recreates the container named spec.Name from the given spec, in a
// crash-safe order so a failure leaves the original container intact:
//
//  1. If a container with spec.Name exists, stop it and rename it to a unique
//     backup name (never deleted until the new one is confirmed running).
//  2. Create + start the new container under the original name.
//  3. On success, remove the backup. On ANY failure, remove the half-built new
//     container and restore the backup (rename back + restart if it was running).
//
// It returns the new container's id. The caller is responsible for snapshotting
// BEFORE calling this (so a later rollback is possible even after the backup is
// gone).
func (c *Client) ApplySpec(ctx context.Context, spec *RecreateSpec) (newID string, err error) {
	if spec == nil || spec.Config == nil || spec.Name == "" {
		return "", errors.New("docker: invalid recreate spec")
	}

	// UnixNano (not Unix seconds) so two recreates of the same container racing
	// within the same second can't mint the same backup name. The recreate
	// service also serialises per-container, but this is cheap defence-in-depth.
	backupName := fmt.Sprintf("%s__stratum_bak_%d", spec.Name, time.Now().UnixNano())
	var (
		backupID      string
		backupRunning bool
	)
	if existing, ierr := c.cli.ContainerInspect(ctx, spec.Name); ierr == nil {
		backupID = existing.ID
		backupRunning = existing.State != nil && existing.State.Running
		// Stop (best-effort; ignore already-stopped) then rename out of the way.
		_ = c.cli.ContainerStop(ctx, backupID, container.StopOptions{})
		if rerr := c.cli.ContainerRename(ctx, backupID, backupName); rerr != nil {
			return "", fmt.Errorf("docker: rename existing container: %w", rerr)
		}
	}

	// Roll back to the backup on any failure past this point. If the rename-back
	// ALSO fails, the original is stranded under the backup name and needs manual
	// recovery — surface that in the error so the audit log/operator sees it.
	restore := func(cause error) (string, error) {
		if newID != "" {
			_ = c.cli.ContainerRemove(ctx, newID, container.RemoveOptions{Force: true})
		}
		if backupID != "" {
			if rerr := c.cli.ContainerRename(ctx, backupID, spec.Name); rerr != nil {
				return "", fmt.Errorf("%w; AND restore failed — original container is stranded as %q: %v", cause, backupName, rerr)
			} else if backupRunning {
				_ = c.cli.ContainerStart(ctx, backupID, container.StartOptions{})
			}
		}
		return "", cause
	}

	netCfg := &network.NetworkingConfig{}
	if len(spec.Endpoints) > 0 {
		netCfg.EndpointsConfig = spec.Endpoints
	}

	// nil platform: the daemon infers it from the image (correct for a
	// same-host recreate).
	created, cerr := c.cli.ContainerCreate(ctx, spec.Config, spec.HostConfig, netCfg, nil, spec.Name)
	if cerr != nil {
		return restore(fmt.Errorf("docker: create container: %w", cerr))
	}
	newID = created.ID

	if serr := c.cli.ContainerStart(ctx, newID, container.StartOptions{}); serr != nil {
		return restore(fmt.Errorf("docker: start recreated container: %w", serr))
	}

	// New container is up — discard the backup.
	if backupID != "" {
		_ = c.cli.ContainerRemove(ctx, backupID, container.RemoveOptions{Force: true})
	}
	return newID, nil
}
