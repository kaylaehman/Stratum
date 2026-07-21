// Package certs implements Certificate Management (Feature F4): it discovers TLS
// certificates on each node's filesystem over SSH, parses expiry/issuer/SANs via
// crypto/x509, and surfaces them for expiry monitoring. It is MONITOR-ONLY —
// Stratum never issues or renews certs. Results are cached per node with a TTL
// and seeded on query (filesystem scans are comparatively expensive).
package certs

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"

	"github.com/KAE-Labs/stratum/backend/db"
)

// ExecFunc runs a command on a node over SSH (fs.Service.Exec).
type ExecFunc func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// scanPaths are the well-known cert locations scanned on each node. They are
// fixed constants (never user input), so embedding them in the scan script is
// injection-safe.
var scanPaths = []string{"/etc/letsencrypt/live", "/etc/ssl", "/opt/certs"}

// maxFiles bounds how many candidate files a single scan reads.
const maxFiles = 200

// warnDays is the expiry threshold that triggers a notification.
const warnDays = 30

// Service scans + caches certificates.
type Service struct {
	store db.Store
	exec  ExecFunc
	ttl   time.Duration

	mu     sync.Mutex
	seeded map[string]time.Time
	sf     singleflight.Group
	notify func(ctx context.Context, trigger, title, text string)
}

// New builds the service. ttl bounds cache staleness before a query re-scans.
func New(store db.Store, exec ExecFunc, ttl time.Duration) *Service {
	return &Service{store: store, exec: exec, ttl: ttl, seeded: map[string]time.Time{}}
}

// SetNotify wires a notification callback fired when certs are nearing expiry.
func (s *Service) SetNotify(fn func(ctx context.Context, trigger, title, text string)) {
	s.notify = fn
}

func (s *Service) fresh(nodeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.seeded[nodeID]
	return ok && time.Since(t) < s.ttl
}

// Invalidate forces the next EnsureFresh for a node to re-scan.
func (s *Service) Invalidate(nodeID string) {
	s.mu.Lock()
	delete(s.seeded, nodeID)
	s.mu.Unlock()
}

// List ensures every node is reasonably fresh, then returns all cached certs.
func (s *Service) List(ctx context.Context) ([]db.CertInfo, error) {
	s.EnsureAll(ctx)
	return s.store.ListCerts(ctx)
}

// EnsureAll scans every node whose cache is stale (best-effort).
func (s *Service) EnsureAll(ctx context.Context) {
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return
	}
	for _, n := range nodes {
		_ = s.EnsureFresh(ctx, n.ID)
	}
}

// RescanAll invalidates and re-scans every node.
func (s *Service) RescanAll(ctx context.Context) {
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return
	}
	for _, n := range nodes {
		s.Invalidate(n.ID)
		_ = s.EnsureFresh(ctx, n.ID)
	}
}

// EnsureFresh re-scans a node if its cache is stale (singleflight-deduped).
func (s *Service) EnsureFresh(ctx context.Context, nodeID string) error {
	if s.fresh(nodeID) {
		return nil
	}
	_, err, _ := s.sf.Do(nodeID, func() (any, error) {
		if s.fresh(nodeID) {
			return nil, nil
		}
		certs, err := s.scanNode(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		if err := s.store.ReplaceCertsByNode(ctx, nodeID, certs); err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.seeded[nodeID] = time.Now()
		s.mu.Unlock()
		s.maybeAlert(ctx, certs)
		return nil, nil
	})
	return err
}

// scanNode runs the discovery script over SSH and parses every cert found.
func (s *Service) scanNode(ctx context.Context, nodeID string) ([]db.CertInfo, error) {
	out, err := s.exec(ctx, nodeID, "sh", "-c", scanScript())
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var certs []db.CertInfo
	for _, blk := range parseBlocks(out) {
		raw, derr := base64.StdEncoding.DecodeString(strings.TrimSpace(blk.b64))
		if derr != nil {
			continue
		}
		leaf := leafCert(raw)
		if leaf == nil {
			continue // not a certificate file (e.g. a private key)
		}
		nb, na := leaf.NotBefore, leaf.NotAfter
		certs = append(certs, db.CertInfo{
			ID:          uuid.NewString(),
			NodeID:      nodeID,
			Source:      "filesystem",
			Domain:      certDomain(leaf),
			SANs:        leaf.DNSNames,
			Issuer:      issuerName(leaf),
			Path:        blk.path,
			NotBefore:   &nb,
			NotAfter:    &na,
			LastChecked: now,
		})
	}
	return certs, nil
}

// maybeAlert notifies when any cert is within warnDays of expiry.
func (s *Service) maybeAlert(ctx context.Context, certs []db.CertInfo) {
	if s.notify == nil {
		return
	}
	now := time.Now()
	soonest := -1
	count := 0
	var soonDomain string
	for _, c := range certs {
		if c.NotAfter == nil {
			continue
		}
		days := int(c.NotAfter.Sub(now).Hours() / 24)
		if days <= warnDays {
			count++
			if soonest < 0 || days < soonest {
				soonest, soonDomain = days, c.Domain
			}
		}
	}
	if count > 0 {
		s.notify(ctx, "cert.expiry", "Certificates expiring soon",
			fmt.Sprintf("%d certificate(s) within %d days; earliest: %s in %d day(s)", count, warnDays, soonDomain, soonest))
	}
}

// --- parsing helpers ---

type certBlock struct {
	path string
	b64  string
}

// scanScript finds candidate cert files under the scan paths and emits each as a
// "===FILE:<path>===" marker followed by its base64 contents. The system trust
// store (/etc/ssl/certs and the ca-certificates / ca-bundle files) is pruned so
// it neither wastes the maxFiles budget nor surfaces CA roots — those are not
// serving certs (leafCert also filters CA certs as a second line of defense).
func scanScript() string {
	return `for f in $(find ` + strings.Join(scanPaths, " ") +
		` -type f \( -name '*.pem' -o -name '*.crt' -o -name '*.cer' \)` +
		` -not -path '*/ssl/certs/*' -not -name 'ca-certificates.crt' -not -name 'ca-bundle.crt' 2>/dev/null | head -n ` +
		fmt.Sprint(maxFiles) + `); do echo "===FILE:$f==="; base64 "$f" 2>/dev/null; done`
}

// parseBlocks splits scan output into per-file base64 blocks.
func parseBlocks(out string) []certBlock {
	var blocks []certBlock
	var cur *certBlock
	for _, line := range strings.Split(out, "\n") {
		if rest, ok := strings.CutPrefix(line, "===FILE:"); ok {
			if cur != nil {
				blocks = append(blocks, *cur)
			}
			path := strings.TrimSuffix(strings.TrimSpace(rest), "===")
			cur = &certBlock{path: path}
			continue
		}
		if cur != nil {
			cur.b64 += line
		}
	}
	if cur != nil {
		blocks = append(blocks, *cur)
	}
	return blocks
}

// leafCert returns the first parseable SERVING (non-CA) certificate in a PEM
// bundle — the leaf of a fullchain. CA certificates are skipped: a serving cert
// is never a CA, so this prevents a system trust-store bundle (all roots/
// intermediates, e.g. /etc/ssl/certs/ca-certificates.crt) from being reported as
// a serving cert — the cause of the bogus identical "ACCVRAIZ1" expiry on every
// node. Returns nil when the bundle holds no serving cert (e.g. a CA bundle or a
// private key file).
func leafCert(pemBytes []byte) *x509.Certificate {
	rest := pemBytes
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil
		}
		if block.Type == "CERTIFICATE" {
			if c, err := x509.ParseCertificate(block.Bytes); err == nil && !c.IsCA {
				return c
			}
		}
		if len(rest) == 0 {
			return nil
		}
	}
}

func certDomain(c *x509.Certificate) string {
	if c.Subject.CommonName != "" {
		return c.Subject.CommonName
	}
	if len(c.DNSNames) > 0 {
		return c.DNSNames[0]
	}
	return ""
}

func issuerName(c *x509.Certificate) string {
	if c.Issuer.CommonName != "" {
		return c.Issuer.CommonName
	}
	if len(c.Issuer.Organization) > 0 {
		return c.Issuer.Organization[0]
	}
	return ""
}
