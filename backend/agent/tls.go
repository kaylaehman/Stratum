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

	"github.com/KAE-Labs/stratum/backend/agentinstall"
)

// pinnedTLS returns a copy of base with ServerName set to the node's stable SAN
// (<node-id>.stratum.internal). This is load-bearing: without it the backend
// verifies only that the agent's cert chains to the CA, so ANY node's cert would
// authenticate as ANY node. Pinning ServerName per dial makes each connection
// verify it is talking to that specific node's issued cert. The SAN is stable
// (not the node's current IP), so a re-IP'd node's cert stays valid.
func pinnedTLS(base *tls.Config, nodeID string) *tls.Config {
	if base == nil {
		return nil
	}
	c := base.Clone()
	c.ServerName = agentinstall.StableSAN(nodeID)
	return c
}

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

// ClientTLSConfigWithCert builds the backend's outbound mTLS config using an
// already-issued client certificate (e.g. minted from the CA at startup by
// certauth) rather than loading one from disk. The agent's gRPC server enforces
// tls.RequireAndVerifyClientCert, so this client cert is what makes the
// connection mutual — without it the handshake is refused by the agent.
func ClientTLSConfigWithCert(caFile string, clientCert tls.Certificate) (*tls.Config, error) {
	cfg, err := ClientTLSConfig("", "", caFile)
	if err != nil {
		return nil, err
	}
	cfg.Certificates = []tls.Certificate{clientCert}
	return cfg, nil
}
