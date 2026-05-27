package certs

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"
)

// makeCertPEM builds a self-signed cert and returns its PEM bytes.
func makeCertPEM(t *testing.T, cn string, sans []string, notAfter time.Time) []byte {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		Issuer:       pkix.Name{CommonName: "Test CA"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     notAfter,
		DNSNames:     sans,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func scanOutput(path string, pemBytes []byte) string {
	return "===FILE:" + path + "===\n" + base64.StdEncoding.EncodeToString(pemBytes) + "\n"
}

func TestScanNodeParsesCerts(t *testing.T) {
	certPEM := makeCertPEM(t, "example.com", []string{"example.com", "www.example.com"}, time.Now().Add(60*24*time.Hour))
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("not-a-real-key")})

	out := scanOutput("/etc/letsencrypt/live/example.com/fullchain.pem", certPEM) +
		scanOutput("/etc/ssl/private/server.key", keyPEM) // not a cert -> skipped

	s := &Service{exec: func(context.Context, string, string, ...string) (string, error) {
		return out, nil
	}}
	got, err := s.scanNode(context.Background(), "node1")
	if err != nil {
		t.Fatalf("scanNode: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d certs, want 1 (key file must be skipped)", len(got))
	}
	c := got[0]
	if c.Domain != "example.com" {
		t.Errorf("domain = %q", c.Domain)
	}
	if len(c.SANs) != 2 {
		t.Errorf("sans = %v", c.SANs)
	}
	// Self-signed: issuer CN == subject CN.
	if c.Issuer != "example.com" {
		t.Errorf("issuer = %q, want example.com (self-signed)", c.Issuer)
	}
	if c.Path != "/etc/letsencrypt/live/example.com/fullchain.pem" {
		t.Errorf("path = %q", c.Path)
	}
	if c.NotAfter == nil || c.NotAfter.Before(time.Now()) {
		t.Errorf("not_after = %v", c.NotAfter)
	}
}

func TestLeafCertPicksFirstInChain(t *testing.T) {
	leaf := makeCertPEM(t, "leaf.example.com", nil, time.Now().Add(24*time.Hour))
	intermediate := makeCertPEM(t, "Intermediate CA", nil, time.Now().Add(365*24*time.Hour))
	bundle := append(append([]byte{}, leaf...), intermediate...)
	c := leafCert(bundle)
	if c == nil || c.Subject.CommonName != "leaf.example.com" {
		t.Fatalf("leafCert picked %v, want leaf.example.com", c)
	}
	if leafCert([]byte("garbage")) != nil {
		t.Error("leafCert(garbage) should be nil")
	}
}
