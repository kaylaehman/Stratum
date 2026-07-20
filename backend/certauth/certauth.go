// Package certauth loads an agent CA (certificate + private key) and issues
// short-lived leaf certificates from it. It is the missing primitive behind the
// backend↔agent mTLS: the agent's gRPC server is configured with
// tls.RequireAndVerifyClientCert, so the backend must present a client cert
// signed by the same CA. certauth mints that cert at startup.
package certauth

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"
)

// CA is a loaded certificate authority able to issue leaf certificates.
type CA struct {
	cert *x509.Certificate
	key  crypto.Signer
}

// LoadCA reads a PEM CA certificate and its PEM private key from disk. The key
// may be PKCS#8, SEC1 (EC), or PKCS#1 (RSA) — whichever the operator generated.
func LoadCA(certPath, keyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("certauth: read CA cert %q: %w", certPath, err)
	}
	cert, err := parseCert(certPEM)
	if err != nil {
		return nil, err
	}
	if !cert.IsCA {
		return nil, errors.New("certauth: provided certificate is not a CA (basicConstraints CA=false)")
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("certauth: read CA key %q: %w", keyPath, err)
	}
	key, err := parseKey(keyPEM)
	if err != nil {
		return nil, err
	}
	return &CA{cert: cert, key: key}, nil
}

// IssueClientCert mints an ECDSA P-256 leaf certificate for TLS client auth with
// the given common name, valid for ttl, and returns it as a ready-to-present
// tls.Certificate. A fresh key is generated per call; nothing is written to disk.
func (c *CA) IssueClientCert(commonName string, ttl time.Duration) (tls.Certificate, error) {
	return c.issue(commonName, ttl, x509.ExtKeyUsageClientAuth)
}

func (c *CA) issue(commonName string, ttl time.Duration, eku x509.ExtKeyUsage) (tls.Certificate, error) {
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certauth: generate leaf key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certauth: serial: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    now.Add(-5 * time.Minute), // tolerate modest clock skew
		NotAfter:     now.Add(ttl),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{eku},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.cert, &leafKey.PublicKey, c.key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certauth: sign leaf: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("certauth: parse issued leaf: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  leafKey,
		Leaf:        leaf,
	}, nil
}

func parseCert(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("certauth: no PEM block in CA certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certauth: parse CA certificate: %w", err)
	}
	return cert, nil
}

// parseKey accepts PKCS#8, SEC1 (EC), or PKCS#1 (RSA) PEM-encoded private keys.
func parseKey(pemBytes []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("certauth: no PEM block in CA key")
	}
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if signer, ok := k.(crypto.Signer); ok {
			return signer, nil
		}
		return nil, errors.New("certauth: PKCS#8 key is not a crypto.Signer")
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("certauth: CA key is not a supported PKCS#8/SEC1/PKCS#1 private key")
}
