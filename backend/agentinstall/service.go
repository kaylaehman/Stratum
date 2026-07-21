// Package agentinstall generates the hardened agent install flow: it mints a
// single-use enrollment token, renders the install script, serves the agent
// binary, and signs the CSR the script submits (via certauth). The agent's
// private key is generated on the target host and never reaches the backend.
package agentinstall

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"text/template"
	"time"

	"github.com/google/uuid"

	"github.com/KAE-Labs/stratum/backend/certauth"
	"github.com/KAE-Labs/stratum/backend/db"
)

//go:embed templates/install.sh.tmpl
var templatesFS embed.FS

var installTmpl = template.Must(template.ParseFS(templatesFS, "templates/install.sh.tmpl"))

var (
	ErrNotConfigured = errors.New("agentinstall: agent CA or binaries not configured")
	ErrTokenInvalid  = errors.New("agentinstall: enrollment token invalid, expired, used, or wrong node")
	ErrBadCSR        = errors.New("agentinstall: could not parse CSR")
	ErrBadArch       = errors.New("agentinstall: unsupported architecture")
	ErrBadNodeID     = errors.New("agentinstall: invalid node id")
	ErrBaseURLScheme = errors.New("agentinstall: BASE_URL must be https to bootstrap agent mTLS")
)

const (
	tokenTTL = 15 * time.Minute
	certTTL  = 90 * 24 * time.Hour
)

// supportedArches maps the ?arch= value to the on-disk binary filename suffix.
var supportedArches = map[string]bool{"amd64": true, "arm64": true}

// nodeIDRe constrains a node id to characters safe to interpolate into a shell
// script and a URL path (node ids are UUIDs; be strict).
var nodeIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// TokenStore is the subset of persistence agentinstall needs.
type TokenStore interface {
	CreateEnrollToken(ctx context.Context, t db.EnrollToken) error
	ConsumeEnrollToken(ctx context.Context, nodeID, tokenHash string) (bool, error)
	ValidateEnrollToken(ctx context.Context, nodeID, tokenHash string) (bool, error)
	PurgeExpiredEnrollTokens(ctx context.Context) (int64, error)
}

// Service renders install scripts, issues/validates enrollment tokens, serves
// agent binaries, and signs enrollment CSRs.
type Service struct {
	ca      *certauth.CA
	tokens  TokenStore
	binDir  string
	baseURL string

	mu     sync.Mutex
	cached map[string]cachedBinary // arch -> loaded binary + hash
}

type cachedBinary struct {
	data []byte
	sha  string
}

// New builds the service. ca may be nil (CA not configured) and binDir may be
// empty/missing (dev builds without baked binaries); Enabled reports readiness.
func New(ca *certauth.CA, tokens TokenStore, binDir, baseURL string) *Service {
	return &Service{ca: ca, tokens: tokens, binDir: binDir, baseURL: baseURL, cached: map[string]cachedBinary{}}
}

// Enabled reports whether the full flow can run: a CA to sign with and at least
// one baked agent binary to serve.
func (s *Service) Enabled() bool {
	if s.ca == nil || s.binDir == "" {
		return false
	}
	for arch := range supportedArches {
		if _, err := os.Stat(s.binaryPath(arch)); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) binaryPath(arch string) string {
	return filepath.Join(s.binDir, "stratum-agent-linux-"+arch)
}

// StableSAN is the node's stable TLS identity — independent of its current IP,
// so a re-IP'd node's cert stays valid. The backend pins ServerName to this when
// dialing the agent.
func StableSAN(nodeID string) string { return nodeID + ".stratum.internal" }

func hashToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
}

// binary lazily loads + hashes the binary for arch, caching the result.
func (s *Service) binary(arch string) (cachedBinary, error) {
	if !supportedArches[arch] {
		return cachedBinary{}, ErrBadArch
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.cached[arch]; ok {
		return b, nil
	}
	data, err := os.ReadFile(s.binaryPath(arch))
	if err != nil {
		return cachedBinary{}, fmt.Errorf("agentinstall: read %s binary: %w", arch, err)
	}
	sum := sha256.Sum256(data)
	b := cachedBinary{data: data, sha: hex.EncodeToString(sum[:])}
	s.cached[arch] = b
	return b, nil
}

// Binary returns the agent binary bytes for arch (for the download handler).
func (s *Service) Binary(arch string) ([]byte, error) {
	b, err := s.binary(arch)
	if err != nil {
		return nil, err
	}
	return b.data, nil
}

// ValidateToken reports whether token is currently valid for nodeID without
// consuming it (gates the non-secret binary download).
func (s *Service) ValidateToken(ctx context.Context, nodeID, token string) (bool, error) {
	return s.tokens.ValidateEnrollToken(ctx, nodeID, hashToken(token))
}

// SignEnrollment consumes the token (single-use, node-scoped) and signs the CSR
// with the node's stable SAN. Returns the signed agent cert PEM.
func (s *Service) SignEnrollment(ctx context.Context, nodeID, token string, csrPEM []byte) ([]byte, error) {
	if s.ca == nil {
		return nil, ErrNotConfigured
	}
	if !nodeIDRe.MatchString(nodeID) {
		return nil, ErrBadNodeID
	}
	ok, err := s.tokens.ConsumeEnrollToken(ctx, nodeID, hashToken(token))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTokenInvalid
	}
	block, _ := pem.Decode(csrPEM)
	if block == nil {
		return nil, ErrBadCSR
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, ErrBadCSR
	}
	return s.ca.SignCSR(csr, "stratum-agent", []string{StableSAN(nodeID)}, certTTL)
}

// ScriptParams is the trusted node context for rendering an install script.
type ScriptParams struct {
	NodeID    string
	Host      string // node address; used to pick a listen bind
	CreatedBy string // user id, for the token audit trail
}

// scriptData is the validated template payload.
type scriptData struct {
	NodeID      string
	BaseURL     string
	Token       string
	CACert      string
	SHA256Amd64 string
	SHA256Arm64 string
	ListenAddr  string
}

// RenderInstallScript validates inputs, mints a single-use enrollment token, and
// renders the install script. It refuses to run over a non-https BASE_URL (the
// agent key/cert bootstrap must not transit cleartext).
func (s *Service) RenderInstallScript(ctx context.Context, p ScriptParams) (string, error) {
	if !s.Enabled() {
		return "", ErrNotConfigured
	}
	if !nodeIDRe.MatchString(p.NodeID) {
		return "", ErrBadNodeID
	}
	u, err := url.Parse(s.baseURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return "", ErrBaseURLScheme
	}
	amd64, err := s.binary("amd64")
	if err != nil {
		return "", err
	}
	arm64, err := s.binary("arm64")
	if err != nil {
		return "", err
	}

	tok, err := s.issueToken(ctx, p.NodeID, p.CreatedBy)
	if err != nil {
		return "", err
	}

	data := scriptData{
		NodeID:      p.NodeID,
		BaseURL:     u.Scheme + "://" + u.Host, // no path/query smuggling
		Token:       tok,
		CACert:      string(bytes.TrimSpace(s.ca.CACertPEM())),
		SHA256Amd64: amd64.sha,
		SHA256Arm64: arm64.sha,
		ListenAddr:  listenAddr(p.Host),
	}
	var buf bytes.Buffer
	if err := installTmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agentinstall: render script: %w", err)
	}
	return buf.String(), nil
}

// listenAddr binds the agent to the node's own address when it's an IP (so it's
// not exposed on every interface); otherwise the agent default (:7750).
func listenAddr(host string) string {
	if ip := net.ParseIP(host); ip != nil {
		return net.JoinHostPort(host, "7750")
	}
	return ":7750"
}

func (s *Service) issueToken(ctx context.Context, nodeID, createdBy string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("agentinstall: token entropy: %w", err)
	}
	tok := base64.RawURLEncoding.EncodeToString(raw)
	now := time.Now()
	err := s.tokens.CreateEnrollToken(ctx, db.EnrollToken{
		ID:        uuid.NewString(),
		NodeID:    nodeID,
		TokenHash: hashToken(tok),
		Action:    "agent-install",
		CreatedBy: createdBy,
		CreatedAt: now,
		ExpiresAt: now.Add(tokenTTL),
	})
	if err != nil {
		return "", err
	}
	return tok, nil
}

// Purge deletes used/expired tokens; call periodically.
func (s *Service) Purge(ctx context.Context) (int64, error) {
	return s.tokens.PurgeExpiredEnrollTokens(ctx)
}

// Run purges used/expired enrollment tokens hourly until ctx is done, so the
// agent_enroll_tokens table can't grow unbounded from unused install-script
// generations.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_, _ = s.Purge(ctx)
		}
	}
}
