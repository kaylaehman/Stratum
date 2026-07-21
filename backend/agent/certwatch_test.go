package agent

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
)

// generateSelfSigned writes a self-signed cert+key PEM pair to dir and returns
// the paths.  t.Helper is called so any failures report at the call-site.
func generateSelfSigned(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "stratum-test"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	cf, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	if err := pem.Encode(cf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("encode cert: %v", err)
	}
	cf.Close()

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	kf, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	if err := pem.Encode(kf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		t.Fatalf("encode key: %v", err)
	}
	kf.Close()

	return certPath, keyPath
}

// newNullManager builds a Manager without a real store, for testing
// InvalidateAll without DB access.
func newNullManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(nil, nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
}

func TestCertWatcherInitialLoad(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateSelfSigned(t, dir)

	mgr := newNullManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cw, err := newCertWatcher(certPath, keyPath, certPath, mgr, logger)
	if err != nil {
		t.Fatalf("newCertWatcher: %v", err)
	}

	cfg := cw.TLSConfig()
	if cfg == nil {
		t.Fatal("TLSConfig() returned nil")
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Errorf("MinVersion = %d, want TLS 1.3", cfg.MinVersion)
	}
	if len(cfg.Certificates) == 0 {
		t.Error("expected at least one client certificate in TLS config")
	}
}

func TestCertWatcherReloadsOnChange(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateSelfSigned(t, dir)

	mgr := newNullManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cw, err := newCertWatcher(certPath, keyPath, certPath, mgr, logger)
	if err != nil {
		t.Fatalf("newCertWatcher: %v", err)
	}

	originalHashes := cw.hashes

	// Overwrite the cert file with a fresh cert to simulate rotation.
	// We write the same path so the watcher picks it up on next poll.
	dir2 := t.TempDir()
	newCertPath, newKeyPath := generateSelfSigned(t, dir2)

	newCertData, _ := os.ReadFile(newCertPath)
	newKeyData, _ := os.ReadFile(newKeyPath)
	os.WriteFile(certPath, newCertData, 0o600)
	os.WriteFile(keyPath, newKeyData, 0o600)

	// Trigger check manually (bypasses ticker).
	cw.checkAndReload()

	if cw.hashes == originalHashes {
		t.Error("hashes unchanged after cert rotation — reload did not trigger")
	}

	// TLSConfig should still be valid after reload.
	if cw.TLSConfig() == nil {
		t.Error("TLSConfig() nil after rotation")
	}
}

func TestCertWatcherNoChangeSkipsReload(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateSelfSigned(t, dir)

	mgr := newNullManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cw, err := newCertWatcher(certPath, keyPath, certPath, mgr, logger)
	if err != nil {
		t.Fatalf("newCertWatcher: %v", err)
	}

	originalCfg := cw.TLSConfig()
	originalHashes := cw.hashes

	cw.checkAndReload()

	if cw.hashes != originalHashes {
		t.Error("hashes changed even though files were not modified")
	}
	if cw.TLSConfig() != originalCfg {
		t.Error("TLSConfig pointer changed even though files were not modified")
	}
}

func TestCertWatcherInvalidatesConnectionsOnRotation(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateSelfSigned(t, dir)

	mgr := newNullManager(t)
	// Inject a fake cached client to verify it gets invalidated.
	mgr.mu.Lock()
	mgr.clients["node-1"] = &Client{nodeID: "node-1"}
	mgr.mu.Unlock()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cw, err := newCertWatcher(certPath, keyPath, certPath, mgr, logger)
	if err != nil {
		t.Fatalf("newCertWatcher: %v", err)
	}

	// Rotate the cert.
	dir2 := t.TempDir()
	newCertPath, newKeyPath := generateSelfSigned(t, dir2)
	newCertData, _ := os.ReadFile(newCertPath)
	newKeyData, _ := os.ReadFile(newKeyPath)
	os.WriteFile(certPath, newCertData, 0o600)
	os.WriteFile(keyPath, newKeyData, 0o600)

	cw.checkAndReload()

	mgr.mu.Lock()
	remaining := len(mgr.clients)
	mgr.mu.Unlock()

	if remaining != 0 {
		t.Errorf("expected 0 cached clients after rotation, got %d", remaining)
	}
}

func TestCertWatcherRunStopsOnCtxCancel(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := generateSelfSigned(t, dir)

	mgr := newNullManager(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cw, err := newCertWatcher(certPath, keyPath, certPath, mgr, logger)
	if err != nil {
		t.Fatalf("newCertWatcher: %v", err)
	}
	// Set a very short poll so the test doesn't take long.
	cw.pollInt = 1 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		cw.Run(ctx)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("certWatcher.Run did not stop after context cancel")
	}
}

// TestAgentAbsentDegracefullyNoPanic ensures that a Manager with no TLS config
// (nil) for an SSH-only / agentless node returns an error rather than panicking.
// This validates the "agentless stays first-class" requirement.
func TestAgentAbsentDegracefullyNoPanic(t *testing.T) {
	store := &stubStore{}
	store.getNodeFn = func(_ context.Context, id string) (db.Node, error) {
		// Simulate a node that has no agent capability.
		return db.Node{
			ID:               "ssh-only",
			Host:             "192.168.1.5",
			CapabilitiesJSON: `{"proxmox":false,"docker":true,"agent":false}`,
		}, nil
	}

	mgr := NewManager(store, nil /* no TLS */, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	_, err := mgr.Get(context.Background(), "ssh-only")
	if err == nil {
		t.Fatal("expected error for agent-absent node, got nil")
	}
}

// TestAgentTLSNilDegracefully tests that even if capabilities say agent=true
// but no TLS config is set, we get a clean error (not a panic).
func TestAgentTLSNilDegracefully(t *testing.T) {
	store := &stubStore{}
	store.getNodeFn = func(_ context.Context, id string) (db.Node, error) {
		return db.Node{
			ID:               "node-with-agent",
			Host:             "192.168.1.10",
			CapabilitiesJSON: `{"agent":true}`,
		}, nil
	}

	mgr := NewManager(store, nil /* intentionally nil */, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	_, err := mgr.Get(context.Background(), "node-with-agent")
	if err == nil {
		t.Fatal("expected error when TLS config is nil, got nil")
	}
}
