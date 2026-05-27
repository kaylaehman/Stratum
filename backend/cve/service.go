package cve

import (
	"context"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/docker"
)

// ClientProvider yields a docker client for a node (to resolve image digests).
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

// ScanContainer scans a container's image (by ref) and caches the results under
// the image's content digest (falling back to the local image id for
// locally-built images). Returns ErrUnavailable if Trivy isn't installed.
func (s *Service) ScanContainer(ctx context.Context, c db.Container) error {
	if !s.scanner.Available() {
		return ErrUnavailable
	}
	digest := s.resolveDigest(ctx, c)
	vulns, err := s.scanner.Scan(ctx, c.Image)
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
