package agentinstall

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/certauth"
	"github.com/KAE-Labs/stratum/backend/db"
)

// --- test doubles ----------------------------------------------------------

type memTokens struct {
	mu sync.Mutex
	m  map[string]db.EnrollToken
}

func newMemTokens() *memTokens { return &memTokens{m: map[string]db.EnrollToken{}} }

func (t *memTokens) CreateEnrollToken(_ context.Context, tok db.EnrollToken) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.m[tok.TokenHash] = tok
	return nil
}

func (t *memTokens) valid(nodeID, hash string) (db.EnrollToken, bool) {
	tok, ok := t.m[hash]
	if !ok || tok.NodeID != nodeID || tok.UsedAt != nil || time.Now().After(tok.ExpiresAt) {
		return db.EnrollToken{}, false
	}
	return tok, true
}

func (t *memTokens) ConsumeEnrollToken(_ context.Context, nodeID, hash string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tok, ok := t.valid(nodeID, hash)
	if !ok {
		return false, nil
	}
	now := time.Now()
	tok.UsedAt = &now
	t.m[hash] = tok
	return true, nil
}

func (t *memTokens) ValidateEnrollToken(_ context.Context, nodeID, hash string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.valid(nodeID, hash)
	return ok, nil
}

func (t *memTokens) PurgeExpiredEnrollTokens(context.Context) (int64, error) { return 0, nil }

// --- fixtures --------------------------------------------------------------

func testCA(t *testing.T) *certauth.CA {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.pem")
	keyPath := filepath.Join(dir, "ca-key.pem")
	os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(key)
	os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), 0o600)
	ca, err := certauth.LoadCA(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	return ca
}

func testService(t *testing.T, baseURL string) (*Service, *memTokens) {
	t.Helper()
	binDir := t.TempDir()
	for _, arch := range []string{"amd64", "arm64"} {
		os.WriteFile(filepath.Join(binDir, "stratum-agent-linux-"+arch), []byte("fake-"+arch), 0o755)
	}
	toks := newMemTokens()
	return New(testCA(t), toks, binDir, baseURL), toks
}

// --- tests -----------------------------------------------------------------

func TestRenderInstallScript_HardeningAndNoSecrets(t *testing.T) {
	svc, _ := testService(t, "https://stratum.example.com")
	script, err := svc.RenderInstallScript(context.Background(), ScriptParams{NodeID: "node-abc", Host: "10.0.0.5"})
	if err != nil {
		t.Fatalf("RenderInstallScript: %v", err)
	}

	mustContain := []string{
		"--proto '=https'",             // https-only fetch
		"sha256sum -c",                 // checksum verified
		"useradd --system",             // non-root user
		"User=stratum-agent",           // service runs non-root
		"NoNewPrivileges=yes",
		"ProtectSystem=strict",
		"MemoryDenyWriteExecute=yes",
		"RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6",
		"SystemCallFilter=@system-service",
		"CapabilityBoundingSet=",
		"openssl req -new",                    // CSR generated on host
		`NODE_ID="node-abc"`,                  // node id set as a shell var
		"/api/nodes/$NODE_ID/agent/enroll",    // enroll URL uses the shell var
		"listen_addr: \"10.0.0.5:7750\"", // bound to the node IP, not 0.0.0.0
		"BEGIN CERTIFICATE",            // CA cert embedded (public)
	}
	for _, s := range mustContain {
		if !strings.Contains(script, s) {
			t.Errorf("rendered script missing %q", s)
		}
	}
	mustNotContain := []string{
		"--insecure", " -k ", "-L ", // no TLS-verification bypass / redirect-follow
		"BEGIN EC PRIVATE KEY", "BEGIN PRIVATE KEY", // no private key ever embedded
	}
	for _, s := range mustNotContain {
		if strings.Contains(script, s) {
			t.Errorf("rendered script must not contain %q", s)
		}
	}
}

func TestRenderInstallScript_RejectsNonHTTPSBaseURL(t *testing.T) {
	svc, _ := testService(t, "http://stratum.example.com")
	if _, err := svc.RenderInstallScript(context.Background(), ScriptParams{NodeID: "node-abc", Host: "h"}); err != ErrBaseURLScheme {
		t.Errorf("err = %v; want ErrBaseURLScheme for http BASE_URL", err)
	}
}

func TestSignEnrollment_SingleUseAndNodeScoped(t *testing.T) {
	svc, _ := testService(t, "https://s.example.com")
	ctx := context.Background()

	// Mint a token by rendering an install script for node-A.
	script, err := svc.RenderInstallScript(ctx, ScriptParams{NodeID: "node-A", Host: "h"})
	if err != nil {
		t.Fatal(err)
	}
	token := extractToken(t, script)

	// A CSR from an agent key.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csrDER, _ := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "stratum-agent"},
	}, key)
	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER})

	// Wrong node → rejected even with a valid token.
	if _, err := svc.SignEnrollment(ctx, "node-B", token, csrPEM); err != ErrTokenInvalid {
		t.Errorf("cross-node enroll: err = %v; want ErrTokenInvalid", err)
	}

	// Correct node → signs, and the issued cert carries node-A's stable SAN.
	certPEM, err := svc.SignEnrollment(ctx, "node-A", token, csrPEM)
	if err != nil {
		t.Fatalf("SignEnrollment: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	leaf, _ := x509.ParseCertificate(block.Bytes)
	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != "node-A.stratum.internal" {
		t.Errorf("cert SAN = %v; want [node-A.stratum.internal]", leaf.DNSNames)
	}

	// Single-use: the same token can't enroll again.
	if _, err := svc.SignEnrollment(ctx, "node-A", token, csrPEM); err != ErrTokenInvalid {
		t.Errorf("token reuse: err = %v; want ErrTokenInvalid", err)
	}
}

// extractToken pulls the TOKEN="..." value out of a rendered script.
func extractToken(t *testing.T, script string) string {
	t.Helper()
	const marker = `TOKEN="`
	i := strings.Index(script, marker)
	if i < 0 {
		t.Fatal("no TOKEN in script")
	}
	rest := script[i+len(marker):]
	return rest[:strings.IndexByte(rest, '"')]
}
