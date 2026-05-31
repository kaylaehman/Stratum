package agentsrv

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// TLSConfig holds file paths needed to build a mutual-TLS config.
// CertFile and KeyFile are the agent's own certificate and private key.
// CAFile is the backend's CA certificate used to authenticate the backend
// client.
type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

// BuildServerTLS returns a *tls.Config suitable for the gRPC server side of an
// mTLS connection. It requires a valid cert, key, and CA — failing fast if any
// file is absent or malformed, so the agent never silently serves plaintext.
func BuildServerTLS(cfg TLSConfig) (*tls.Config, error) {
	if cfg.CertFile == "" || cfg.KeyFile == "" || cfg.CAFile == "" {
		return nil, errors.New("tls: cert, key, and CA paths are all required for mTLS")
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("tls: load agent cert/key: %w", err)
	}

	caPEM, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("tls: read CA cert: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("tls: CA cert PEM did not contain any valid certificates")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}, nil
}
