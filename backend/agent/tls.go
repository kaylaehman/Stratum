// Package agent provides a gRPC client for the Stratum per-host agent.
// All connections use mutual TLS: the backend presents its own cert/key
// (signed by the CA) and verifies the agent's cert against the same CA.
package agent

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
)

// ClientTLSConfig builds a *tls.Config for the backend's outbound gRPC
// connections to agents. certFile/keyFile are the backend's own client cert;
// caFile is the CA certificate that signed both the backend cert and the
// agent cert.
//
// TODO(install-script): The backend cert/key for mTLS client auth is separate
// from the CA. If AGENT_CA_CERT_PATH/AGENT_CA_KEY_PATH are present but the
// backend has no client cert yet, this function returns an error. A follow-up
// will auto-issue the backend's client cert from the CA at startup.
func ClientTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	if caFile == "" {
		return nil, errors.New("agent: CA cert path is required for mTLS (set AGENT_CA_CERT_PATH)")
	}

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("agent: read CA cert %q: %w", caFile, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("agent: CA cert PEM contained no valid certificates")
	}

	tlsCfg := &tls.Config{
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
	}

	// If a client cert/key are provided, add them for mutual auth.
	if certFile != "" && keyFile != "" {
		cert, cerr := tls.LoadX509KeyPair(certFile, keyFile)
		if cerr != nil {
			return nil, fmt.Errorf("agent: load backend client cert/key: %w", cerr)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
