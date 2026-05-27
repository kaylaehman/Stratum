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

// getJSON performs an authenticated GET to the given path (relative to endpoint)
// and JSON-decodes a 200 response body into dst. It returns an error for any
// non-200 status. Callers that need to handle 401 specially (like Version) do
// so by checking the error or using the raw request pattern instead.
//
// The path must start with "/".
func (c *Client) getJSON(ctx context.Context, path string, dst interface{}) error {
	url := c.endpoint + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("proxmox: build request: %w", err)
	}

	req.Header.Set("Authorization", "PVEAPIToken="+c.tokenID+"="+c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("proxmox: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("proxmox: unexpected status %d from %s", resp.StatusCode, url)
	}

	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("proxmox: decode response from %s: %w", path, err)
	}
	return nil
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

// ClusterNode is a member of the Proxmox cluster.
type ClusterNode struct {
	Name   string // node name, e.g. "pve"
	Online bool   // true when the "online" field == 1 OR "status" == "online"
}

// clusterNodeRaw is the raw JSON shape of a single item in the /nodes response.
type clusterNodeRaw struct {
	Node   string `json:"node"`
	Status string `json:"status"`
	Online int    `json:"online"`
}

// nodesResponse wraps the top-level data array from GET /api2/json/nodes.
type nodesResponse struct {
	Data []clusterNodeRaw `json:"data"`
}

// Nodes lists cluster members via GET /api2/json/nodes.
// A member is considered online when the "online" field == 1 OR when
// "status" == "online" (some PVE versions use one or the other).
func (c *Client) Nodes(ctx context.Context) ([]ClusterNode, error) {
	var resp nodesResponse
	if err := c.getJSON(ctx, "/api2/json/nodes", &resp); err != nil {
		return nil, err
	}

	nodes := make([]ClusterNode, 0, len(resp.Data))
	for _, raw := range resp.Data {
		nodes = append(nodes, ClusterNode{
			Name:   raw.Node,
			Online: raw.Online == 1 || raw.Status == "online",
		})
	}
	return nodes, nil
}

// Guest is a Proxmox QEMU VM or LXC container as listed by the API.
type Guest struct {
	VMID   int    // Proxmox vmid
	Name   string // guest name (empty string when absent)
	Status string // "running" | "stopped" (verbatim from API)
	Kind   string // "qemu" or "lxc" — set by the calling method
}

// guestRaw holds the raw JSON for a single guest item. vmid may arrive as a
// JSON number or a quoted string depending on PVE version and endpoint, so we
// decode it into a json.RawMessage and parse both forms.
type guestRaw struct {
	VMID   json.RawMessage `json:"vmid"`
	Name   string          `json:"name"`
	Status string          `json:"status"`
}

// guestsResponse wraps the top-level data array from the qemu/lxc endpoints.
type guestsResponse struct {
	Data []guestRaw `json:"data"`
}

// parseVMID converts a json.RawMessage that is either a JSON number (100) or a
// JSON string ("100") into an int.
func parseVMID(raw json.RawMessage) (int, error) {
	// Try number first — the common case.
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	// Fall back to quoted string.
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, fmt.Errorf("proxmox: cannot parse vmid from %s", string(raw))
	}
	var n2 int
	if _, err := fmt.Sscanf(s, "%d", &n2); err != nil {
		return 0, fmt.Errorf("proxmox: vmid string %q is not an integer", s)
	}
	return n2, nil
}

// guestsFromResponse converts raw API items to Guest values, setting Kind on each.
func guestsFromResponse(items []guestRaw, kind string) ([]Guest, error) {
	guests := make([]Guest, 0, len(items))
	for _, raw := range items {
		vmid, err := parseVMID(raw.VMID)
		if err != nil {
			return nil, err
		}
		guests = append(guests, Guest{
			VMID:   vmid,
			Name:   raw.Name,
			Status: raw.Status,
			Kind:   kind,
		})
	}
	return guests, nil
}

// QemuList lists QEMU VMs on a cluster node via GET /api2/json/nodes/{node}/qemu.
// Sets Kind="qemu" on each returned Guest.
func (c *Client) QemuList(ctx context.Context, node string) ([]Guest, error) {
	var resp guestsResponse
	if err := c.getJSON(ctx, "/api2/json/nodes/"+node+"/qemu", &resp); err != nil {
		return nil, err
	}
	return guestsFromResponse(resp.Data, "qemu")
}

// LxcList lists LXC containers on a cluster node via GET /api2/json/nodes/{node}/lxc.
// Sets Kind="lxc" on each returned Guest. vmid is decoded robustly from either
// a JSON number or a quoted string.
func (c *Client) LxcList(ctx context.Context, node string) ([]Guest, error) {
	var resp guestsResponse
	if err := c.getJSON(ctx, "/api2/json/nodes/"+node+"/lxc", &resp); err != nil {
		return nil, err
	}
	return guestsFromResponse(resp.Data, "lxc")
}
