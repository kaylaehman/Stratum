package certauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTestCA generates a self-signed CA and writes cert.pem/key.pem (PKCS#8),
// returning their paths.
func writeTestCA(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath = filepath.Join(dir, "ca.pem")
	keyPath = filepath.Join(dir, "ca-key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	writePEM(t, keyPath, "PRIVATE KEY", pkcs8)
	return certPath, keyPath
}

func writePEM(t *testing.T, path, typ string, der []byte) {
	t.Helper()
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestIssueClientCert_ChainsAndHasClientAuth(t *testing.T) {
	certPath, keyPath := writeTestCA(t)
	ca, err := LoadCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadCA: %v", err)
	}

	leaf, err := ca.IssueClientCert("stratum-backend", time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert: %v", err)
	}
	if leaf.Leaf == nil {
		t.Fatal("issued cert has nil Leaf")
	}
	if leaf.Leaf.Subject.CommonName != "stratum-backend" {
		t.Errorf("CN = %q, want stratum-backend", leaf.Leaf.Subject.CommonName)
	}

	// It must verify against the CA for client auth — exactly what the agent does.
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	if _, err := leaf.Leaf.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Errorf("issued leaf failed client-auth verification against CA: %v", err)
	}
}

func TestLoadCA_RejectsNonCA(t *testing.T) {
	// A leaf (non-CA) cert must be rejected as a CA.
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "not-a-ca"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "leaf.pem")
	keyPath := filepath.Join(dir, "leaf-key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(key)
	writePEM(t, keyPath, "PRIVATE KEY", pkcs8)

	if _, err := LoadCA(certPath, keyPath); err == nil {
		t.Error("LoadCA accepted a non-CA certificate, want error")
	}
}

func makeCSR(t *testing.T, key *ecdsa.PrivateKey, subjectCN string, sans []string) *x509.CertificateRequest {
	t.Helper()
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: subjectCN},
		DNSNames: sans,
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := x509.ParseCertificateRequest(der)
	if err != nil {
		t.Fatal(err)
	}
	return csr
}

func TestSignCSR_UsesBackendIdentityNotCSR(t *testing.T) {
	certPath, keyPath := writeTestCA(t)
	ca, err := LoadCA(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadCA: %v", err)
	}

	// A hostile requester asks for someone else's identity + a CA cert.
	attackerKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csr := makeCSR(t, attackerKey, "some-other-node", []string{"victim-node.stratum.internal"})

	// The backend signs with ITS chosen identity for the real node.
	const wantSAN = "node-abc.stratum.internal"
	certPEM, err := ca.SignCSR(csr, "stratum-agent", []string{wantSAN}, time.Hour)
	if err != nil {
		t.Fatalf("SignCSR: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	if leaf.Subject.CommonName != "stratum-agent" {
		t.Errorf("CN = %q; backend value must win, not the CSR's", leaf.Subject.CommonName)
	}
	if len(leaf.DNSNames) != 1 || leaf.DNSNames[0] != wantSAN {
		t.Errorf("SAN = %v; must be the backend's %q, not the CSR's victim SAN", leaf.DNSNames, wantSAN)
	}
	if leaf.IsCA {
		t.Error("issued leaf is a CA — CSR must not be able to request IsCA")
	}
	// The public key must be the requester's (proof-of-possession preserved)...
	if !leaf.PublicKey.(*ecdsa.PublicKey).Equal(&attackerKey.PublicKey) {
		t.Error("issued cert's public key is not the CSR's public key")
	}
	// ...and it must verify for ServerAuth against the CA at the backend-set SAN.
	pool := x509.NewCertPool()
	pool.AddCert(ca.cert)
	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     pool,
		DNSName:   wantSAN,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Errorf("issued leaf failed ServerAuth verification at %q: %v", wantSAN, err)
	}
}

func TestSignCSR_RejectsTamperedSignature(t *testing.T) {
	certPath, keyPath := writeTestCA(t)
	ca, _ := LoadCA(certPath, keyPath)
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	csr := makeCSR(t, key, "n", nil)
	// Corrupt the CSR's self-signature — proof-of-possession must then fail.
	csr.Signature[len(csr.Signature)-1] ^= 0xff
	if _, err := ca.SignCSR(csr, "stratum-agent", []string{"n.stratum.internal"}, time.Hour); err == nil {
		t.Error("SignCSR accepted a CSR with a broken self-signature")
	}
}

func TestSignCSR_RejectsNonP256(t *testing.T) {
	certPath, keyPath := writeTestCA(t)
	ca, _ := LoadCA(certPath, keyPath)
	key, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader) // wrong curve
	csr := makeCSR(t, key, "n", nil)
	if _, err := ca.SignCSR(csr, "stratum-agent", []string{"n.stratum.internal"}, time.Hour); err == nil {
		t.Error("SignCSR accepted a non-P256 (P-384) key")
	}
}
