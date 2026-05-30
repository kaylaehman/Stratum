package cve

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

// ClientProvider yields a docker client for a node (to resolve image digests and
// export local images for tarball scanning).
type ClientProvider func(ctx context.Context, nodeID string) (*docker.Client, error)

// Service scans containers' images and caches results by image digest.
type Service struct {
	store    db.Store
	provider ClientProvider
	scanner  *Scanner
}

// New builds the service.
func New(store db.Store, provider ClientProvider, scanner *Scanner) *Service {
	return &Service{store: store, provider: provider, scanner: scanner}
}

// Available reports whether the underlying scanner is installed.
func (s *Service) Available() bool { return s.scanner.Available() }

// Status returns scanner presence + DB freshness for the UI.
func (s *Service) Status(ctx context.Context) Status { return s.scanner.StatusInfo(ctx) }

// WarmDB best-effort warms the Trivy vulnerability DB (non-fatal, time-bounded).
func (s *Service) WarmDB(ctx context.Context) error { return s.scanner.WarmDB(ctx) }

// ScanContainer scans a container's image and caches the results under the
// image's content digest (falling back to the local image id for locally-built
// images). Returns ErrUnavailable if Trivy isn't installed.
//
// Path selection: images that carry a registry repo digest are scanned by
// reference (the fast registry path). Locally-built/unpublished images (no repo
// digest) are EXPORTED from the node's Docker endpoint to a temp tarball and
// scanned with `trivy image --input <tar>`, because a bare `trivy image <ref>`
// would fall back to a registry pull and fail for an image that was never pushed.
func (s *Service) ScanContainer(ctx context.Context, c db.Container) error {
	if !s.scanner.Available() {
		return ErrUnavailable
	}
	digest := s.resolveDigest(ctx, c)

	vulns, err := s.scanImage(ctx, c)
	if err != nil {
		return err
	}

	rows := make([]db.CVEResultRow, len(vulns))
	for i, v := range vulns {
		rows[i] = db.CVEResultRow{
			ImageDigest: digest, CVEID: v.CVEID, Severity: v.Severity, Package: v.Package,
			InstalledVersion: v.InstalledVersion, FixedVersion: v.FixedVersion, Title: v.Title,
		}
	}
	if err := s.store.ReplaceCVEResults(ctx, digest, rows); err != nil {
		return err
	}
	counts := SeverityCounts(vulns)
	return s.store.UpsertImageScan(ctx, db.ImageScanRow{
		ImageDigest: digest, Image: c.Image,
		Critical: counts[SevCritical], High: counts[SevHigh], Medium: counts[SevMedium],
		Low: counts[SevLow], Unknown: counts[SevUnknown],
	})
}

// scanImage runs the appropriate scan path for the container's image: tarball
// export for local images, registry reference otherwise. If we cannot determine
// locality (no docker client), we fall back to the registry path.
func (s *Service) scanImage(ctx context.Context, c db.Container) ([]Vuln, error) {
	client, err := s.provider(ctx, c.NodeID)
	if err != nil {
		// No docker access for this node: best-effort registry path.
		return s.scanner.Scan(ctx, c.Image)
	}

	// Probe locality by the presence of a registry repo digest. Prefer probing by
	// image id (stable) and fall back to the ref.
	probe := c.ImageID
	if probe == "" {
		probe = c.Image
	}
	hasDigest, derr := client.HasRepoDigest(ctx, probe)
	if derr == nil && hasDigest {
		// Published image — fast registry path.
		return s.scanner.Scan(ctx, c.Image)
	}

	// Local/unpublished (or couldn't tell): export to a temp tarball and scan it.
	vulns, terr := s.scanViaTarball(ctx, client, c)
	if terr == nil {
		return vulns, nil
	}
	// Tarball path failed (e.g. a restricted socket-proxy blocks image export);
	// fall back to the registry path so published-but-misprobed images still work.
	return s.scanner.Scan(ctx, c.Image)
}

// scanViaTarball exports the container's image from the node's Docker endpoint to
// a temp tar and scans it with `trivy image --input`. The temp file is always
// cleaned up.
func (s *Service) scanViaTarball(ctx context.Context, client *docker.Client, c db.Container) ([]Vuln, error) {
	ref := c.ImageID
	if ref == "" {
		ref = c.Image
	}
	rc, err := client.ImageSave(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("cve: export image %q: %w", ref, err)
	}
	defer rc.Close()

	tarPath := tempTarPath()
	f, err := os.Create(tarPath)
	if err != nil {
		return nil, fmt.Errorf("cve: create temp tar: %w", err)
	}
	defer os.Remove(tarPath)
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		return nil, fmt.Errorf("cve: write image tar: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("cve: close image tar: %w", err)
	}
	return s.scanner.ScanTarball(ctx, tarPath)
}

// resolveDigest returns the image's repo digest, or its local image id when the
// repo digest is unavailable (e.g. a locally-built image).
func (s *Service) resolveDigest(ctx context.Context, c db.Container) string {
	if client, err := s.provider(ctx, c.NodeID); err == nil {
		if d, err := client.LocalRepoDigest(ctx, c.ImageID); err == nil && d != "" {
			return d
		}
	}
	return c.ImageID
}

// ListScans returns the cached scan summaries.
func (s *Service) ListScans(ctx context.Context) ([]db.ImageScanRow, error) {
	return s.store.ListImageScans(ctx)
}

// Vulns returns the detailed vulnerabilities for a scanned digest.
func (s *Service) Vulns(ctx context.Context, imageDigest string) ([]db.CVEResultRow, error) {
	return s.store.ListCVEResults(ctx, imageDigest)
}
