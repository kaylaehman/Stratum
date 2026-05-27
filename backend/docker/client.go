package docker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	dockerclient "github.com/docker/docker/client"
)

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
		httpClient := &http.Client{
			Transport: &http.Transport{TLSClientConfig: tc},
		}
		opts = append(opts, dockerclient.WithHTTPClient(httpClient))
	}

	cli, err := dockerclient.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker: new client: %w", err)
	}
	return &Client{cli: cli}, nil
}

// buildTLSConfig constructs a *tls.Config from PEM strings.
func buildTLSConfig(t *TLS) (*tls.Config, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(t.CA)) {
		return nil, fmt.Errorf("docker: failed to parse CA PEM")
	}
	cert, err := tls.X509KeyPair([]byte(t.Cert), []byte(t.Key))
	if err != nil {
		return nil, fmt.Errorf("docker: parse client cert/key: %w", err)
	}
	return &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
	}, nil
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
