package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	// List + Create. Create writes a new ingress hostname into the tunnel config
	// and (by default) the proxied CNAME the route needs. Update/Delete stay
	// unsupported by design — the ask is to *add* a route, and keeping mutation
	// to a single additive operation bounds the blast radius on live infra.
	return Capabilities{List: true, Create: true}
}

func (c *CloudflareAPI) UpdateRule(context.Context, Conn, string, Rule) error { return ErrUnsupported }
func (c *CloudflareAPI) DeleteRule(context.Context, Conn, string) error       { return ErrUnsupported }

// cfTunnelCNAMESuffix is the target every named-tunnel CNAME points at:
// "<tunnel-id>.cfargotunnel.com". Cloudflare proxies the hostname to the tunnel.
const cfTunnelCNAMESuffix = ".cfargotunnel.com"

// CreateRule adds a public hostname to the tunnel's ingress and, unless opted
// out, ensures the proxied CNAME the hostname needs to route. It is additive:
// the new entry is inserted before the catch-all, every existing entry is kept
// verbatim, and a duplicate hostname+path is rejected rather than overwritten.
//
// Steps (in order, so a later failure leaves an observable state):
//  1. validate the hostname + service URL
//  2. read the current config; reject locally-managed tunnels
//  3. insert the entry before the catch-all (reject duplicates)
//  4. PUT the updated ingress
//  5. ensure the DNS record (best-effort; a failure here is returned as a
//     warning *after* the ingress succeeded — the route exists but won't
//     resolve until DNS is fixed)
//
// create_dns is read from conn.Config: any value other than "false" ensures DNS.
func (c *CloudflareAPI) CreateRule(ctx context.Context, conn Conn, r Rule) (Rule, error) {
	host := strings.TrimSpace(strings.ToLower(r.SourceHost))
	if err := validateProxyHostname(host); err != nil {
		return Rule{}, err
	}
	service := strings.TrimSpace(r.TargetURL)
	if err := validateProxyService(service); err != nil {
		return Rule{}, err
	}
	path := strings.TrimSpace(r.SourcePath)

	cl, err := c.client(conn)
	if err != nil {
		return Rule{}, err
	}
	accountID, err := c.resolveAccountID(ctx, cl, conn)
	if err != nil {
		return Rule{}, err
	}
	tunnelID := strings.TrimSpace(conn.Config["tunnel_id"])
	if tunnelID == "" {
		return Rule{}, fmt.Errorf("cloudflare-api: no tunnel selected — choose a tunnel (set tunnel_id)")
	}

	cur, err := cl.getTunnelConfig(ctx, accountID, tunnelID)
	if err != nil {
		return Rule{}, err
	}
	if strings.EqualFold(cur.Source, "local") {
		return Rule{}, fmt.Errorf("cloudflare-api: tunnel %s is locally-managed — its ingress lives in the on-host cloudflared config.yml; edit it there instead", tunnelID)
	}

	existing := []cfIngress(nil)
	if cur.Config != nil {
		existing = cur.Config.Ingress
	}
	merged, err := insertIngress(existing, cfIngress{Hostname: host, Service: service, Path: path})
	if err != nil {
		return Rule{}, err
	}
	if err := cl.putTunnelConfig(ctx, accountID, tunnelID, cfTunnelConfig{Ingress: merged}); err != nil {
		return Rule{}, err
	}

	out := Rule{
		ID:          "cf-api-" + host,
		AdapterType: "cloudflare-api",
		SourceHost:  host,
		SourcePath:  path,
		TargetURL:   service,
		SSLEnabled:  true,
	}

	if conn.Config["create_dns"] != "false" {
		if err := cl.ensureTunnelDNS(ctx, host, tunnelID); err != nil {
			// The ingress write already succeeded — report DNS as a non-fatal
			// warning so the caller can surface "route added, but DNS needs
			// attention" rather than rolling back a good config change.
			return out, fmt.Errorf("%w: %v", ErrDNSWarning, err)
		}
	}
	return out, nil
}

// ErrDNSWarning wraps a post-ingress DNS failure: the route was added but its
// CNAME could not be ensured. Callers should surface it as a warning, not a
// hard failure (errors.Is(err, ErrDNSWarning)).
var ErrDNSWarning = errors.New("cloudflare-api: route added but DNS record could not be ensured")

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

// get performs an authenticated GET and returns the decoded envelope.
func (cl *cfClient) get(ctx context.Context, path string) (cfEnvelope, error) {
	return cl.do(ctx, http.MethodGet, path, nil)
}

// do performs an authenticated request, JSON-encoding body when non-nil, and
// returns the decoded envelope, mapping HTTP and API errors to actionable
// messages. A nil body sends no payload (GET/DELETE).
func (cl *cfClient) do(ctx context.Context, method, path string, body any) (cfEnvelope, error) {
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return cfEnvelope{}, fmt.Errorf("cloudflare-api: encode request: %w", err)
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, cl.base+path, rdr)
	if err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cl.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := cl.http.Do(req)
	if err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, cfReadLimit))
	if err != nil {
		return cfEnvelope{}, fmt.Errorf("cloudflare-api: read response: %w", err)
	}
	return decodeCFResponse(resp.StatusCode, respBody)
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

// putTunnelConfig writes the tunnel's ingress configuration via PUT. The body
// shape mirrors the GET result's "config" object. Needs token scope
// Account → Cloudflare Tunnel → Edit.
func (cl *cfClient) putTunnelConfig(ctx context.Context, accountID, tunnelID string, cfg cfTunnelConfig) error {
	if err := validateCFID("account", accountID); err != nil {
		return err
	}
	if err := validateCFID("tunnel", tunnelID); err != nil {
		return err
	}
	body := map[string]any{"config": cfg}
	_, err := cl.do(ctx, http.MethodPut,
		"/accounts/"+url.PathEscape(accountID)+"/cfd_tunnel/"+url.PathEscape(tunnelID)+"/configurations",
		body)
	return err
}

// cfZone is one zone returned by the zones list endpoint.
type cfZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// cfDNSRecord is the subset of a DNS record we read/write.
type cfDNSRecord struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl,omitempty"`
}

// findZone walks the hostname's parent domains and returns the first zone the
// token can see that matches. For "jellyfin.kaylas.systems" it tries
// "jellyfin.kaylas.systems", then "kaylas.systems", etc. Needs token scope
// Zone → Zone → Read. Returns a not-found error when no parent is a visible zone.
func (cl *cfClient) findZone(ctx context.Context, hostname string) (cfZone, error) {
	for _, cand := range zoneCandidates(hostname) {
		env, err := cl.get(ctx, "/zones?name="+url.QueryEscape(cand))
		if err != nil {
			return cfZone{}, err
		}
		var zones []cfZone
		if err := json.Unmarshal(env.Result, &zones); err != nil {
			return cfZone{}, fmt.Errorf("cloudflare-api: parse zones: %w", err)
		}
		if len(zones) > 0 {
			return zones[0], nil
		}
	}
	return cfZone{}, fmt.Errorf("cloudflare-api: no Cloudflare zone found for %q (token needs Zone → Zone → Read, and the domain must be in this account)", hostname)
}

// ensureTunnelDNS makes sure a proxied CNAME exists pointing the hostname at the
// tunnel. It is idempotent: an existing record with the correct content is left
// alone; a missing one is created. Needs token scope Zone → DNS → Edit (+ Read).
func (cl *cfClient) ensureTunnelDNS(ctx context.Context, hostname, tunnelID string) error {
	zone, err := cl.findZone(ctx, hostname)
	if err != nil {
		return err
	}
	target := tunnelID + cfTunnelCNAMESuffix

	// Look for an existing record for this exact name.
	env, err := cl.get(ctx, "/zones/"+url.PathEscape(zone.ID)+"/dns_records?name="+url.QueryEscape(hostname))
	if err != nil {
		return err
	}
	var existing []cfDNSRecord
	if err := json.Unmarshal(env.Result, &existing); err != nil {
		return fmt.Errorf("cloudflare-api: parse dns records: %w", err)
	}
	for _, rec := range existing {
		if !strings.EqualFold(rec.Type, "CNAME") {
			continue
		}
		content := strings.ToLower(strings.TrimSuffix(rec.Content, "."))
		switch {
		case content == strings.ToLower(target):
			return nil // already points at THIS tunnel — nothing to do
		case strings.HasSuffix(content, cfTunnelCNAMESuffix):
			// A CNAME to a DIFFERENT tunnel: the route we just added points here,
			// so leaving this record would send traffic to the wrong tunnel. Don't
			// clobber it — surface the conflict for the user to repoint manually.
			return fmt.Errorf("cloudflare-api: %q already CNAMEs to a different tunnel (%s) — repoint it to %s manually", hostname, rec.Content, target)
		}
	}
	if len(existing) > 0 {
		// A record exists but isn't a tunnel CNAME — don't silently overwrite a
		// record that may serve something else.
		return fmt.Errorf("cloudflare-api: a DNS record for %q already exists and is not a tunnel CNAME — remove or repoint it manually", hostname)
	}

	rec := cfDNSRecord{Type: "CNAME", Name: hostname, Content: target, Proxied: true, TTL: 1}
	_, err = cl.do(ctx, http.MethodPost, "/zones/"+url.PathEscape(zone.ID)+"/dns_records", rec)
	return err
}

// zoneCandidates returns the hostname's progressively-shorter parent domains,
// most-specific first, down to (but not including) a bare TLD. The hostname
// itself is included first to handle a route at a zone apex.
func zoneCandidates(hostname string) []string {
	labels := strings.Split(strings.Trim(hostname, "."), ".")
	var out []string
	for i := 0; i+1 < len(labels); i++ { // stop before the last single label (TLD)
		out = append(out, strings.Join(labels[i:], "."))
	}
	return out
}

// insertIngress returns a new ingress slice with entry inserted immediately
// before the catch-all (the trailing entry with no hostname). Every existing
// entry is preserved in order. A duplicate hostname+path is rejected. When the
// input has no catch-all (e.g. an empty config), a default 404 catch-all is
// appended so the resulting config stays valid.
func insertIngress(existing []cfIngress, entry cfIngress) ([]cfIngress, error) {
	for _, in := range existing {
		if strings.EqualFold(strings.TrimSpace(in.Hostname), entry.Hostname) &&
			strings.TrimSpace(in.Path) == entry.Path {
			return nil, fmt.Errorf("cloudflare-api: a route for %q already exists on this tunnel", entry.Hostname)
		}
	}
	out := make([]cfIngress, 0, len(existing)+2)
	inserted := false
	for _, in := range existing {
		if !inserted && strings.TrimSpace(in.Hostname) == "" {
			out = append(out, entry) // place new route just before the catch-all
			inserted = true
		}
		out = append(out, in)
	}
	if !inserted {
		// No catch-all present — add the route, then a default catch-all so
		// Cloudflare accepts the config (the last rule must match everything).
		out = append(out, entry, cfIngress{Service: "http_status:404"})
	}
	return out, nil
}

// hostnamePattern validates a DNS hostname conservatively: dot-separated labels
// of letters/digits/hyphens, each 1–63 chars, at least two labels. Rejects
// schemes, ports, paths, and wildcards so a crafted value can't reach the API
// as something other than a hostname.
var hostnamePattern = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

// ErrInvalidHostname / ErrInvalidService are bad-input sentinels so the API layer
// can map a malformed route request to 400 rather than a 502 (which would imply
// an upstream Cloudflare failure). errors.Is(err, ErrInvalidHostname) etc.
var (
	ErrInvalidHostname = errors.New("cloudflare-api: invalid source hostname")
	ErrInvalidService  = errors.New("cloudflare-api: invalid target service URL")
)

// validateProxyHostname rejects anything that isn't a plain fully-qualified host.
func validateProxyHostname(host string) error {
	if host == "" {
		return fmt.Errorf("%w: a source hostname is required", ErrInvalidHostname)
	}
	if len(host) > 253 || !hostnamePattern.MatchString(host) {
		return fmt.Errorf("%w: %q", ErrInvalidHostname, host)
	}
	return nil
}

// validateProxyService requires an http/https service URL with a host. Cloudflare
// also accepts a handful of non-HTTP services (ssh://, rdp://, tcp://) but the
// container add-flow only ever targets HTTP backends, so we keep the surface tight.
func validateProxyService(service string) error {
	if service == "" {
		return fmt.Errorf("%w: a target service URL is required", ErrInvalidService)
	}
	u, err := url.Parse(service)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidService, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: must be an http(s) URL", ErrInvalidService)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: must include a host", ErrInvalidService)
	}
	return nil
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
