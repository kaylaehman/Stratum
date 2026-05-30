package docker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// genCertPEM produces a throwaway self-signed cert + its private key as PEM,
// suitable for exercising buildTLSConfig's parse paths (no network use).
func genCertPEM(t *testing.T) (certPEM, keyPEM string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	key := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return string(cert), string(key)
}

// TestNew_TLSVariants verifies the independently-optional TLS material handling:
// CA-only (server TLS, no client auth), mTLS (CA + client cert/key), and the
// error when a client cert is supplied without its key.
func TestNew_TLSVariants(t *testing.T) {
	certPEM, keyPEM := genCertPEM(t)

	// CA-only: a server-TLS-only socket proxy with no client certificate.
	if _, err := New("tcp://127.0.0.1:2376", &TLS{CA: certPEM}); err != nil {
		t.Errorf("CA-only TLS should build: %v", err)
	}
	// Full mTLS: CA + client cert/key.
	if _, err := New("tcp://127.0.0.1:2376", &TLS{CA: certPEM, Cert: certPEM, Key: keyPEM}); err != nil {
		t.Errorf("mTLS should build: %v", err)
	}
	// Client cert/key without a CA: verify against system roots.
	if _, err := New("tcp://127.0.0.1:2376", &TLS{Cert: certPEM, Key: keyPEM}); err != nil {
		t.Errorf("client cert/key without CA should build: %v", err)
	}
	// Cert without its key is rejected.
	if _, err := New("tcp://127.0.0.1:2376", &TLS{Cert: certPEM}); err == nil {
		t.Error("client cert without key should error")
	}
}

// TestNew_LocalDefault verifies that constructing a client with the local
// default (empty endpoint, no TLS) succeeds without requiring a live daemon.
func TestNew_LocalDefault(t *testing.T) {
	c, err := New("", nil)
	if err != nil {
		t.Fatalf("New(\"\", nil) unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("New(\"\", nil) returned nil client")
	}
}

// TestNew_InvalidTLS verifies that malformed PEM material is rejected at
// construction time (before any network call is made).
func TestNew_InvalidTLS(t *testing.T) {
	bogus := &TLS{
		CA:   "not-valid-pem",
		Cert: "not-valid-pem",
		Key:  "not-valid-pem",
	}
	_, err := New("tcp://127.0.0.1:2376", bogus)
	if err == nil {
		t.Fatal("New with invalid PEM expected an error, got nil")
	}
}

// TestClose_Fresh verifies that Close on a freshly-constructed client returns nil.
func TestClose_Fresh(t *testing.T) {
	c, err := New("", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}
}

// TestPing_Unreachable verifies that Ping against an unreachable endpoint
// returns an error quickly (no hanging). No live daemon required.
func TestPing_Unreachable(t *testing.T) {
	// Use a TCP endpoint that is guaranteed to refuse connections fast.
	c, err := New("tcp://127.0.0.1:19999", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = c.Ping(ctx)
	if err == nil {
		t.Fatal("Ping to unreachable endpoint expected error, got nil")
	}
}
