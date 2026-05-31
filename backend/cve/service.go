package cve

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

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
	logger   *slog.Logger
}

// New builds the service.
func New(store db.Store, provider ClientProvider, scanner *Scanner) *Service {
	return &Service{store: store, provider: provider, scanner: scanner, logger: slog.Default()}
}

// SetLogger replaces the logger (e.g., for structured test output).
func (s *Service) SetLogger(l *slog.Logger) { s.logger = l }

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

// resolveDigest returns the image's repo digest, or the best available stable
// key for the image when the repo digest is unavailable.
//
// Priority: (1) repo digest from Docker inspect (most stable, content-
// addressable across hosts), (2) local image ID (content-addressable on this
// host), (3) image reference/tag as last resort (not content-addressable but
// better than an empty string).
//
// An empty string is never returned — an empty digest causes the frontend to
// skip the detail query entirely (useCVEDetail guards on `digest.length > 0`),
// so any container whose image_id column is NULL in the DB would show a
// non-zero summary count with a permanently empty CVE detail panel.
func (s *Service) resolveDigest(ctx context.Context, c db.Container) string {
	if client, err := s.provider(ctx, c.NodeID); err == nil {
		if d, err := client.LocalRepoDigest(ctx, c.ImageID); err == nil && d != "" {
			return d
		}
	}
	if c.ImageID != "" {
		return c.ImageID
	}
	return c.Image
}

// ListScans returns the cached scan summaries.
func (s *Service) ListScans(ctx context.Context) ([]db.ImageScanRow, error) {
	return s.store.ListImageScans(ctx)
}

// Vulns returns the detailed vulnerabilities for a scanned digest.
func (s *Service) Vulns(ctx context.Context, imageDigest string) ([]db.CVEResultRow, error) {
	return s.store.ListCVEResults(ctx, imageDigest)
}

// BulkScanResult is the per-container outcome of a bulk scan.
type BulkScanResult struct {
	ContainerID string `json:"container_id"`
	Image       string `json:"image"`
	Error       string `json:"error,omitempty"`
}

// ScanBulk scans each of the given containers sequentially (reusing the same
// scan path as ScanContainer). It always returns the full result slice; per-
// container errors are recorded but do not abort remaining scans.
func (s *Service) ScanBulk(ctx context.Context, containers []db.Container) []BulkScanResult {
	results := make([]BulkScanResult, len(containers))
	for i, c := range containers {
		r := BulkScanResult{ContainerID: c.ID, Image: c.Image}
		if err := s.ScanContainer(ctx, c); err != nil {
			r.Error = err.Error()
		}
		results[i] = r
	}
	return results
}

// CreateSchedule persists a new CVE schedule.
func (s *Service) CreateSchedule(ctx context.Context, sched db.CveSchedule) error {
	return s.store.CreateCveSchedule(ctx, sched)
}

// ListSchedules returns all configured CVE schedules.
func (s *Service) ListSchedules(ctx context.Context) ([]db.CveSchedule, error) {
	return s.store.ListCveSchedules(ctx)
}

// UpdateScheduleEnabled toggles a schedule's enabled flag.
func (s *Service) UpdateScheduleEnabled(ctx context.Context, id string, enabled bool) error {
	return s.store.UpdateCveScheduleEnabled(ctx, id, enabled)
}

// DeleteSchedule removes a CVE schedule by id.
func (s *Service) DeleteSchedule(ctx context.Context, id string) error {
	return s.store.DeleteCveSchedule(ctx, id)
}

// RunSchedules is the background loop that fires scheduled CVE scans. It
// checks every minute whether a schedule is due and, if so, scans the
// targeted containers. Blocks until ctx is cancelled.
func (s *Service) RunSchedules(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tickSchedules(ctx)
		}
	}
}

// tickSchedules is the body of one scheduler tick: loads all enabled
// schedules, fires any that are due, and records their last-run time.
func (s *Service) tickSchedules(ctx context.Context) {
	schedules, err := s.store.ListCveSchedules(ctx)
	if err != nil {
		s.logger.Warn("cve: list schedules", "error", err)
		return
	}
	for _, sched := range schedules {
		if !sched.Enabled {
			continue
		}
		interval := time.Duration(sched.IntervalSeconds) * time.Second
		if sched.LastRunAt != nil && time.Since(*sched.LastRunAt) < interval {
			continue
		}
		go s.runSchedule(ctx, sched)
	}
}

// runSchedule fires one scheduled scan, updating last_run_at regardless of
// per-container errors (so a persistent failure doesn't create a hot loop).
func (s *Service) runSchedule(ctx context.Context, sched db.CveSchedule) {
	now := time.Now()
	if err := s.store.UpdateCveScheduleLastRun(ctx, sched.ID, now); err != nil {
		s.logger.Warn("cve: update schedule last_run_at", "id", sched.ID, "error", err)
	}

	containers, err := s.containersForSchedule(ctx, sched)
	if err != nil {
		s.logger.Warn("cve: resolve schedule targets", "id", sched.ID, "error", err)
		return
	}
	for _, c := range containers {
		if err := s.ScanContainer(ctx, c); err != nil {
			s.logger.Warn("cve: scheduled scan", "container", c.ID, "image", c.Image, "error", err)
		}
	}
}

// containersForSchedule resolves the containers that a schedule targets. For
// target_type "node" it returns all containers on the node; for "container" it
// returns the single container record.
func (s *Service) containersForSchedule(ctx context.Context, sched db.CveSchedule) ([]db.Container, error) {
	switch sched.TargetType {
	case "node":
		return s.store.ListContainersByNode(ctx, sched.TargetID)
	case "container":
		c, err := s.store.GetContainer(ctx, sched.TargetID)
		if err != nil {
			return nil, fmt.Errorf("cve: get container %s: %w", sched.TargetID, err)
		}
		return []db.Container{c}, nil
	default:
		return nil, fmt.Errorf("cve: unknown schedule target_type %q", sched.TargetType)
	}
}
