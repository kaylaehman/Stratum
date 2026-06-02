package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// cfIDPattern matches Cloudflare account/tunnel identifiers (hex UUIDs or
// short slugs). IDs are validated against it before being interpolated into an
// API URL path so a crafted value can't smuggle path segments.
var cfIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// validateCFID rejects empty or malformed account/tunnel IDs.
func validateCFID(kind, id string) error {
	if !cfIDPattern.MatchString(id) {
		return fmt.Errorf("cloudflare-api: invalid %s id", kind)
	}
	return nil
}

func init() { Register(&CloudflareAPI{}) }

// cloudflareAPIBase is the default Cloudflare API root. Overridable via
// Conn.Endpoint (host-only base URL) so tests can point at a fixture server.
const cloudflareAPIBase = "https://api.cloudflare.com/client/v4"

// cfReadLimit caps how many bytes we read from a Cloudflare API response.
const cfReadLimit = 4 << 20 // 4 MiB

// CloudflareAPI lists a cloudflared tunnel's public hostnames over the
// Cloudflare API. Unlike the file-based Cloudflared adapter, it works for
// DASHBOARD-MANAGED (remotely-managed) tunnels, which have no local ingress
// block to parse. It is read-only and opt-in: a node selects it by storing
// kind="cloudflare-api" + account_id/tunnel_id in proxy_config.config_json
// (with the API token in token_encrypted). It is never image-auto-detected, so
// it returns no ImagePatterns.
//
// Required token scope: Account → Cloudflare Tunnel → Read.
type CloudflareAPI struct {
	HTTP *http.Client // injected by Service.New; nil falls back to http.DefaultClient
}

func (c *CloudflareAPI) Name() string            { return "cloudflare-api" }
func (c *CloudflareAPI) ImagePatterns() []string { return nil } // opt-in only; not auto-detected
func (c *CloudflareAPI) Capabilities() Capabilities {
	return Capabilities{List: true} // read-only; tunnel ingress is owned by Cloudflare
}

func (c *CloudflareAPI) CreateRule(context.Context, Conn, Rule) (Rule, error) {
	return Rule{}, ErrUnsupported
}
func (c *CloudflareAPI) UpdateRule(context.Context, Conn, string, Rule) error { return ErrUnsupported }
func (c *CloudflareAPI) DeleteRule(context.Context, Conn, string) error       { return ErrUnsupported }

// ListRules fetches the tunnel's live ingress config from the Cloudflare API
// and converts each public hostname to a Rule. Errors are mapped to actionable
// messages (bad token, tunnel not found, locally-managed tunnel).
func (c *CloudflareAPI) ListRules(ctx context.Context, conn Conn) ([]Rule, error) {
	cl, err := c.client(conn)
	if err != nil {
		return nil, err
	}
	accountID, err := c.resolveAccountID(ctx, cl, conn)
	if err != nil {
		return nil, err
	}
	tunnelID := strings.TrimSpace(conn.Config["tunnel_id"])
	if tunnelID == "" {
		return nil, fmt.Errorf("cloudflare-api: no tunnel selected — choose a tunnel (set tunnel_id)")
	}

	res, err := cl.getTunnelConfig(ctx, accountID, tunnelID)
	if err != nil {
		return nil, err
	}
	// A locally-managed tunnel keeps its ingress in the on-host config.yml; the
	// API returns source="local" and no usable config. Point the user at the
	// file-based provider instead of returning a misleading empty list.
	if strings.EqualFold(res.Source, "local") {
		return nil, fmt.Errorf("cloudflare-api: tunnel %s is locally-managed — its ingress is in the on-host cloudflared config.yml, not the dashboard (use the file-based cloudflared provider for this node)", tunnelID)
	}
	if res.Config == nil {
		return []Rule{}, nil // dashboard-managed but no ingress configured yet
	}
	return parseCloudflareIngress(res.Config.Ingress), nil
}

// client builds the Cloudflare API client for a connection, validating the token.
func (c *CloudflareAPI) client(conn Conn) (*cfClient, error) {
	token := strings.TrimSpace(conn.Token)
	if token == "" {
		return nil, fmt.Errorf("cloudflare-api: API token not configured (needs scope Account → Cloudflare Tunnel → Read)")
	}
	base := strings.TrimRight(strings.TrimSpace(conn.Endpoint), "/")
	if base == "" {
		base = cloudflareAPIBase
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	return &cfClient{http: hc, base: base, token: token}, nil
}

// resolveAccountID returns the configured account_id, or discovers it from the
// token's accessible accounts (using the only account, or erroring on the
// multi-account case so the user picks one).
func (c *CloudflareAPI) resolveAccountID(ctx context.Context, cl *cfClient, conn Conn) (string, error) {
	if id := strings.TrimSpace(conn.Config["account_id"]); id != "" {
		return id, nil
	}
	accounts, err := cl.listAccounts(ctx)
	if err != nil {
		return "", err
	}
	switch len(accounts) {
	case 0:
		return "", fmt.Errorf("cloudflare-api: token has no accessible accounts")
	case 1:
		return accounts[0].ID, nil
	default:
		names := make([]string, 0, len(accounts))
		for _, a := range accounts {
			names = append(names, a.Name)
		}
		return "", fmt.Errorf("cloudflare-api: token has multiple accounts (%s) — set account_id to choose one", strings.Join(names, ", "))
	}
}

// CFAccount / CFTunnel are the picker entries surfaced to the API/UI.
type CFAccount struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type CFTunnel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListAccounts returns the accounts the token can see (for the UI picker).
func (c *CloudflareAPI) ListAccounts(ctx context.Context, conn Conn) ([]CFAccount, error) {
	cl, err := c.client(conn)
	if err != nil {
		return nil, err
	}
	return cl.listAccounts(ctx)
}

// ListTunnels returns the (non-deleted) tunnels in an account (for the UI
// picker). When accountID is empty it is resolved like ListRules.
func (c *CloudflareAPI) ListTunnels(ctx context.Context, conn Conn) ([]CFTunnel, error) {
	cl, err := c.client(conn)
	if err != nil {
		return nil, err
	}
	accountID, err := c.resolveAccountID(ctx, cl, conn)
	if err != nil {
		return nil, err
	}
	return cl.listTunnels(ctx, accountID)
}

// ---- Cloudflare API client ----------------------------------------------

type cfClient struct {
	http  *http.Client
	base  string
	token string
}

// cfEnvelope is the standard Cloudflare API response envelope.
type cfEnvelope struct {
	Success bool            `json:"success"`
	Errors  []cfError       `json:"errors"`
	Result  json.RawMessage `json:"result"`
}
type cfError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// get performs an authenticated GET and returns the decoded envelope, mapping
// HTTP and API errors to actionable messages.
func (cl *cfClient) get(ctx context.Context, path string) (cfEnvelope, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cl.base+path, nil)
	if err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cl.token)
	req.Header.Set("Accept", "application/json")
	resp, err := cl.http.Do(req)
	if err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, cfReadLimit))
	if err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: read response: %w", err)
	}
	return decodeCFResponse(resp.StatusCode, body)
}

// decodeCFResponse maps a Cloudflare API HTTP status + body to an envelope or a
// clear error. Split out for unit testing without a live server.
func decodeCFResponse(status int, body []byte) (cfEnvelope, error) {
	switch status {
	case http.StatusUnauthorized:
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: invalid API token (401 Unauthorized)")
	case http.StatusForbidden:
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: API token lacks permission (403) — it needs scope Account → Cloudflare Tunnel → Read")
	case http.StatusNotFound:
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: not found (404) — check the account_id / tunnel_id")
	}
	var env cfEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: unexpected response (HTTP %d): %w", status, err)
	}
	if !env.Success {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: %s", cfErrorString(env.Errors, status))
	}
	return env, nil
}

func cfErrorString(errs []cfError, status int) string {
	if len(errs) == 0 {
		return fmt.Sprintf("request failed (HTTP %d)", status)
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Message)
	}
	return strings.Join(msgs, "; ")
}

func (cl *cfClient) listAccounts(ctx context.Context) ([]CFAccount, error) {
	// per_page=50 is the Cloudflare maximum for the accounts endpoint; a token
	// scoped to >50 accounts (rare) would be truncated. Acceptable for the
	// homelab target — the user can set account_id explicitly if needed.
	env, err := cl.get(ctx, "/accounts?per_page=50")
	if err != nil {
		return nil, err
	}
	var accounts []CFAccount
	if err := json.Unmarshal(env.Result, &accounts); err != nil {
		return nil, fmt.Errorf("cloudflare-api: parse accounts: %w", err)
	}
	return accounts, nil
}

func (cl *cfClient) listTunnels(ctx context.Context, accountID string) ([]CFTunnel, error) {
	if err := validateCFID("account", accountID); err != nil {
		return nil, err
	}
	// per_page=100 comfortably covers any homelab; tunnels beyond the first page
	// are intentionally not fetched (no pagination loop).
	env, err := cl.get(ctx, "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel?is_deleted=false&per_page=100")
	if err != nil {
		return nil, err
	}
	var tunnels []CFTunnel
	if err := json.Unmarshal(env.Result, &tunnels); err != nil {
		return nil, fmt.Errorf("cloudflare-api: parse tunnels: %w", err)
	}
	return tunnels, nil
}

// cfTunnelConfigResult is the result of GET .../cfd_tunnel/{id}/configurations.
type cfTunnelConfigResult struct {
	TunnelID string          `json:"tunnel_id"`
	Version  int             `json:"version"`
	Config   *cfTunnelConfig `json:"config"`
	Source   string          `json:"source"` // "cloudflare" (dashboard-managed) | "local"
}
type cfTunnelConfig struct {
	Ingress []cfIngress `json:"ingress"`
}
type cfIngress struct {
	Hostname string `json:"hostname"`
	Service  string `json:"service"`
	Path     string `json:"path,omitempty"`
}

func (cl *cfClient) getTunnelConfig(ctx context.Context, accountID, tunnelID string) (cfTunnelConfigResult, error) {
	if err := validateCFID("account", accountID); err != nil {
		return cfTunnelConfigResult{}, err
	}
	if err := validateCFID("tunnel", tunnelID); err != nil {
		return cfTunnelConfigResult{}, err
	}
	env, err := cl.get(ctx, "/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel/"+url.PathEscape(tunnelID)+"/configurations")
	if err != nil {
		return cfTunnelConfigResult{}, err
	}
	var res cfTunnelConfigResult
	if err := json.Unmarshal(env.Result, &res); err != nil {
		return cfTunnelConfigResult{}, fmt.Errorf("cloudflare-api: parse tunnel config: %w", err)
	}
	return res, nil
}

// parseCloudflareIngress converts the Cloudflare ingress array to Rules. The
// catch-all entry (no hostname, e.g. service "http_status:404") is skipped, and
// SSLEnabled is true because cloudflared always terminates TLS at the edge.
// Pure function — unit-tested against a recorded fixture.
func parseCloudflareIngress(ingress []cfIngress) []Rule {
	rules := make([]Rule, 0, len(ingress))
	for i, in := range ingress {
		if strings.TrimSpace(in.Hostname) == "" {
			continue // catch-all
		}
		rules = append(rules, Rule{
			ID:          fmt.Sprintf("cf-api-%d", i),
			AdapterType: "cloudflare-api",
			SourceHost:  in.Hostname,
			SourcePath:  in.Path,
			TargetURL:   in.Service,
			SSLEnabled:  true,
		})
	}
	return rules
}

// getCloudflareAPIAdapter returns the registered CloudflareAPI adapter, or nil.
func getCloudflareAPIAdapter() *CloudflareAPI {
	for _, a := range Adapters() {
		if cf, ok := a.(*CloudflareAPI); ok {
			return cf
		}
	}
	return nil
}
