package docker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	dockerclient "github.com/docker/docker/client"
)

// dialTimeout is the TCP dial deadline for remote Docker endpoints.
const dialTimeout = 10 * time.Second

// responseHeaderTimeout is the maximum time to wait for the daemon to send
// the first response byte after the request is sent. Prevents a slow/stuck
// daemon from holding open a connection indefinitely on simple reads.
const responseHeaderTimeout = 30 * time.Second

// TLS holds optional PEM material for a remote tcp+tls endpoint.
type TLS struct {
	CA   string // PEM
	Cert string // PEM
	Key  string // PEM
}

// Client wraps the Docker Engine API SDK client.
type Client struct {
	cli *dockerclient.Client
}

// New builds a Docker client.
//   - endpoint == ""  -> use the local default (client.FromEnv: unix socket / npipe)
//   - endpoint != ""  -> client.WithHost(endpoint) (e.g. "tcp://host:2376")
//   - tls != nil      -> build a *tls.Config from the PEM material and apply via WithHTTPClient
//
// WithAPIVersionNegotiation is always applied.
func New(endpoint string, tlsCfg *TLS) (*Client, error) {
	opts := []dockerclient.Opt{dockerclient.WithAPIVersionNegotiation()}

	if endpoint == "" {
		opts = append(opts, dockerclient.FromEnv)
	} else {
		opts = append(opts, dockerclient.WithHost(endpoint))
	}

	if tlsCfg != nil {
		tc, err := buildTLSConfig(tlsCfg)
		if err != nil {
			return nil, fmt.Errorf("docker: build TLS config: %w", err)
		}
		opts = append(opts, dockerclient.WithHTTPClient(newHTTPClient(tc)))
	} else if endpoint != "" {
		// Non-TLS remote TCP endpoint: still apply transport timeouts so a
		// slow/hung homelab daemon doesn't hold connections open forever.
		opts = append(opts, dockerclient.WithHTTPClient(newHTTPClient(nil)))
	}

	cli, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker: new client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// newHTTPClient builds an *http.Client with explicit dial and response-header
// timeouts. tc may be nil for plain-TCP (non-TLS) remote endpoints. These
// timeouts are per-request transport guards — they do NOT replace the
// per-operation context deadline; rather they bound a hang at the TCP/TLS layer
// before the application context can fire (e.g. a half-open TCP connection that
// never triggers a RST). For long-running streaming calls (logs, events) the
// caller passes a context with no deadline and the stream is bounded by the
// client's own cancellation — ResponseHeaderTimeout does not cut those because
// the header arrives immediately and the body streams afterward.
func newHTTPClient(tc *tls.Config) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSClientConfig:       tc,
			ResponseHeaderTimeout: responseHeaderTimeout,
			// Allow enough idle connections so concurrent poll + ad-hoc handler
			// calls don't race on a single connection.
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// buildTLSConfig constructs a *tls.Config from PEM strings. Each of CA and the
// client cert/key pair is independently optional:
//   - CA only          -> verify the server against a private CA, no client auth
//                          (server-TLS-only socket proxy).
//   - cert + key only   -> present a client certificate, verify the server against
//                          the system roots (mTLS with a publicly-trusted server).
//   - CA + cert + key   -> full mTLS against a private CA (the dockerd --tlsverify
//                          default).
//
// A cert without its key (or vice versa) is an error. At least one of the three
// must be set, else the caller should pass tls == nil instead.
func buildTLSConfig(t *TLS) (*tls.Config, error) {
	cfg := &tls.Config{}
	if t.CA != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(t.CA)) {
			return nil, fmt.Errorf("docker: failed to parse CA PEM")
		}
		cfg.RootCAs = pool
	}
	switch {
	case t.Cert != "" && t.Key != "":
		cert, err := tls.X509KeyPair([]byte(t.Cert), []byte(t.Key))
		if err != nil {
			return nil, fmt.Errorf("docker: parse client cert/key: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	case t.Cert != "" || t.Key != "":
		return nil, fmt.Errorf("docker: client cert and key must be supplied together")
	}
	return cfg, nil
}

// Ping checks daemon reachability and returns the negotiated API version string.
func (c *Client) Ping(ctx context.Context) (string, error) {
	ping, err := c.cli.Ping(ctx)
	if err != nil {
		return "", err
	}
	return ping.APIVersion, nil
}

// Close releases the underlying client.
func (c *Client) Close() error {
	return c.cli.Close()
}
