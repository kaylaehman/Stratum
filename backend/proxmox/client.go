// Package proxmox provides a minimal Proxmox VE REST client using token-based
// authentication. There is no official Go SDK for Proxmox, so this implements
// only what Stratum needs: capability probing and future API surface.
package proxmox

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AuthStatus reflects what the /version probe revealed.
type AuthStatus string

const (
	// AuthConfirmed means HTTP 200: the token is valid and Proxmox responded.
	AuthConfirmed AuthStatus = "confirmed"
	// AuthUnauthed means HTTP 401: the endpoint IS Proxmox, but the token is
	// missing or invalid. This is still a successful Proxmox detection.
	AuthUnauthed AuthStatus = "unauthed"
)

// defaultClientTimeout is applied to the http.Client when the context has no
// deadline of its own. Eight seconds is generous for a LAN probe but not so
// long that it blocks node auto-detection noticeably.
const defaultClientTimeout = 8 * time.Second

// Client is a minimal Proxmox VE REST client (token auth).
// All Proxmox API calls in Stratum must be gated behind a node.type == proxmox
// check before reaching this client.
type Client struct {
	endpoint   string       // base URL, e.g. "https://host:8006"
	tokenID    string       // e.g. "user@realm!tokenname"
	secret     string       // raw token secret UUID
	httpClient *http.Client // configured transport (TLS, timeout)
}

// New builds a Client. endpoint is like "https://host:8006" (trailing slash is
// tolerated and stripped). insecureSkipVerify allows self-signed certificates,
// which is the default in homelab Proxmox installations; callers should default
// to false and only enable it when the user has acknowledged the risk.
func New(endpoint, tokenID, secret string, insecureSkipVerify bool) *Client {
	endpoint = strings.TrimRight(endpoint, "/")

	transport := &http.Transport{
		// insecureSkipVerify is intentionally opt-in; homelab Proxmox nodes
		// almost universally use self-signed certs, so without this option the
		// version probe would always fail on unmanaged PKI environments.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkipVerify}, //nolint:gosec
	}

	return &Client{
		endpoint: endpoint,
		tokenID:  tokenID,
		secret:   secret,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultClientTimeout,
		},
	}
}

// versionResponse is the JSON shape returned by GET /api2/json/version.
type versionResponse struct {
	Data struct {
		Version string `json:"version"`
		Release string `json:"release"`
		RepoID  string `json:"repoid"`
	} `json:"data"`
}

// Version probes GET /api2/json/version and interprets the response:
//   - HTTP 200 → (versionString, AuthConfirmed, nil)
//   - HTTP 401 → ("", AuthUnauthed, nil)   — endpoint IS Proxmox (auth challenge issued)
//   - connection error / non-Proxmox / unexpected status → ("", "", error)
func (c *Client) Version(ctx context.Context) (version string, status AuthStatus, err error) {
	url := c.endpoint + "/api2/json/version"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("proxmox: build request: %w", err)
	}

	// Proxmox token auth header format: PVEAPIToken=<tokenID>=<secret>
	req.Header.Set("Authorization", "PVEAPIToken="+c.tokenID+"="+c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("proxmox: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var v versionResponse
		if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
			return "", "", fmt.Errorf("proxmox: decode version response: %w", err)
		}
		return v.Data.Version, AuthConfirmed, nil

	case http.StatusUnauthorized:
		// 401 is deliberate: Proxmox answered with an auth challenge, which
		// confirms the endpoint is a Proxmox node. Return AuthUnauthed, not an
		// error, so the caller can surface a "fix your token" message rather
		// than treating this as a non-Proxmox host.
		return "", AuthUnauthed, nil

	default:
		return "", "", fmt.Errorf("proxmox: unexpected status %d from %s", resp.StatusCode, url)
	}
}
