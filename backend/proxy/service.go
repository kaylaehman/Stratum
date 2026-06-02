package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/fs"
)

// httpTimeout bounds a single admin-API call to a proxy tool.
const httpTimeout = 15 * time.Second

// Service detects a node's proxy tool from its container inventory and, when an
// admin endpoint is configured, lists its rules through the matching adapter.
type Service struct {
	store  db.Store
	cipher *crypto.Cipher
	http   *http.Client
	files  *fs.Service // optional; enables file-based adapters (e.g. cloudflared)
}

// New wires the store + secret cipher. The shared http client is injected into
// the API-based adapters so they're testable and bounded.
func New(store db.Store, cipher *crypto.Cipher) *Service {
	hc := &http.Client{Timeout: httpTimeout}
	// Point the registered API adapters at the bounded client.
	for _, a := range Adapters() {
		switch v := a.(type) {
		case *Traefik:
			v.HTTP = hc
		case *NPM:
			v.HTTP = hc
		case *CloudflareAPI:
			v.HTTP = hc
		}
	}
	return &Service{store: store, cipher: cipher, http: hc}
}

// WithFiles wires a filesystem service so file-based adapters (e.g. cloudflared)
// can read host config files via SFTP. Call once after New.
func (s *Service) WithFiles(f *fs.Service) { s.files = f }

// Status is the API view for a node: the detected tool (if any), its
// capabilities, whether an admin endpoint is configured, the live rules (when
// listable), and the catalog of supported tools for the empty state.
type Status struct {
	Detected         string       `json:"detected"` // adapter name or ""
	Capabilities     Capabilities `json:"capabilities"`
	Configured       bool         `json:"configured"` // admin endpoint set
	Endpoint         string       `json:"endpoint,omitempty"`
	HasToken         bool         `json:"has_token"`
	Rules            []Rule       `json:"rules"`
	RuleError        string       `json:"rule_error,omitempty"` // why rules couldn't be listed
	DashboardManaged bool         `json:"dashboard_managed"`    // cloudflared: ingress in Cloudflare dashboard
	Supported        []ToolInfo   `json:"supported"`
	// CFAccountID / CFTunnelID echo the stored cloudflare-api selection (non-secret)
	// so the setup form can pre-fill them when editing. Empty for other providers.
	CFAccountID string `json:"cf_account_id,omitempty"`
	CFTunnelID  string `json:"cf_tunnel_id,omitempty"`
}

// detect returns the adapter for a node based on its container images.
// For file-based adapters (cloudflared), also probes for a host-service
// installation when no matching container is found.
func (s *Service) detect(ctx context.Context, nodeID string) (Adapter, error) {
	// An explicit per-node kind override (stored in proxy_config.config_json)
	// takes precedence over image detection. This lets a node opt into the
	// cloudflare-api adapter even when its cloudflared container would otherwise
	// be matched by the file-based provider (e.g. dashboard-managed tunnels).
	if cfg, err := s.store.GetProxyConfig(ctx, nodeID); err == nil {
		switch kind := configKind(cfg.ConfigJSON); kind {
		case "":
			// no override; fall through to image detection
		case "cloudflare-api":
			if cf := getCloudflareAPIAdapter(); cf != nil {
				return cf, nil
			}
			slog.Warn("proxy: cloudflare-api override set but adapter not registered", "node_id", nodeID)
		default:
			// An override naming an unknown adapter shouldn't silently match a
			// container image and present the wrong provider — log and fall
			// through so detection stays observable.
			slog.Warn("proxy: unknown provider kind override, falling back to image detection", "node_id", nodeID, "kind", kind)
		}
	}

	containers, err := s.store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	images := make([]string, 0, len(containers))
	for _, c := range containers {
		images = append(images, c.Image)
	}
	if a := DetectByImages(images); a != nil {
		return a, nil
	}

	// No container matched. Check if cloudflared is installed as a host service
	// by probing config file paths via SSH/agent. This covers the very common
	// deployment where cloudflared is installed with `cloudflared service install`
	// and runs as a systemd unit — no Docker container is involved.
	if s.files != nil {
		cf := getCloudflaredAdapter()
		if cf != nil {
			conn := s.hostConn(nodeID)
			if cf.ProbeHostService(ctx, conn) {
				slog.Info("cloudflared: detected host-service installation (no container)", "node_id", nodeID)
				return cf, nil
			}
		}
	}
	return nil, nil
}

// getCloudflaredAdapter returns the registered Cloudflared adapter, or nil.
func getCloudflaredAdapter() *Cloudflared {
	for _, a := range Adapters() {
		if cf, ok := a.(*Cloudflared); ok {
			return cf
		}
	}
	return nil
}

// hostConn builds a minimal Conn with only ReadFile populated (no endpoint or
// token needed for host-service probing).
func (s *Service) hostConn(nodeID string) Conn {
	if s.files == nil {
		return Conn{}
	}
	filesSvc := s.files
	return Conn{
		ReadFile: func(ctx context.Context, path string) (io.ReadCloser, error) {
			return filesSvc.Download(ctx, nodeID, path)
		},
	}
}

// conn builds the admin connection (endpoint + decrypted token) for a node.
// When a filesystem service is wired, ReadFile is populated so file-based
// adapters can read host config files.
func (s *Service) conn(ctx context.Context, nodeID string) (Conn, bool) {
	cfg, err := s.store.GetProxyConfig(ctx, nodeID)
	if err != nil {
		return Conn{}, false
	}
	c := Conn{Endpoint: cfg.Endpoint, Config: parseConfigJSON(cfg.ConfigJSON)}
	if len(cfg.TokenEncrypted) > 0 {
		if pt, derr := s.cipher.Open(cfg.TokenEncrypted); derr == nil {
			c.Token = string(pt)
		}
	}
	if s.files != nil {
		filesSvc := s.files
		c.ReadFile = func(ctx context.Context, path string) (io.ReadCloser, error) {
			return filesSvc.Download(ctx, nodeID, path)
		}
	}
	return c, cfg.Endpoint != ""
}

// Status detects the node's proxy and lists rules when possible.
func (s *Service) Status(ctx context.Context, nodeID string) (Status, error) {
	st := Status{Supported: SupportedTools(), Rules: []Rule{}}
	adapter, err := s.detect(ctx, nodeID)
	if err != nil {
		return st, err
	}
	if adapter == nil {
		return st, nil // no supported proxy detected
	}
	st.Detected = adapter.Name()
	st.Capabilities = adapter.Capabilities()

	conn, endpointConfigured := s.conn(ctx, nodeID)
	// Enrich conn with bind-mount candidates for file-based adapters.
	s.enrichMountCandidates(ctx, nodeID, adapter.Name(), &conn)

	// A file-based adapter (e.g. cloudflared) is "configured" when host file
	// access is available, even without an admin endpoint. The cloudflare-api
	// adapter is "configured" once an API token is stored (the endpoint is
	// optional and defaults to api.cloudflare.com).
	fileAccessAvailable := conn.ReadFile != nil
	configured := endpointConfigured || (isFileBased(adapter) && fileAccessAvailable)
	if isCloudflareAPI(adapter) {
		configured = conn.Token != ""
	}

	st.Configured = configured
	st.Endpoint = conn.Endpoint
	st.HasToken = conn.Token != ""
	if isCloudflareAPI(adapter) {
		st.CFAccountID = conn.Config["account_id"]
		st.CFTunnelID = conn.Config["tunnel_id"]
	}

	if adapter.Capabilities().List && configured {
		rules, lerr := adapter.ListRules(ctx, conn)
		if lerr != nil {
			st.RuleError = lerr.Error()
			slog.Warn("proxy: could not list rules", "adapter", adapter.Name(), "node_id", nodeID, "error", lerr)
		} else {
			st.Rules = rules
			// Resolve each rule's TargetURL to a known container so the UI can
			// deep-link from a route to the resource it serves. Best-effort and
			// store-only; leaves Resolved nil when no confident match exists.
			s.resolveRuleTargets(ctx, nodeID, st.Rules)
			// For cloudflared: check if this is a dashboard-managed tunnel so the
			// UI can display an informative state rather than an empty table.
			if cf, ok := adapter.(*Cloudflared); ok && len(rules) == 0 {
				st.DashboardManaged = cf.IsDashboardManaged(ctx, conn)
			}
		}
	} else if adapter.Capabilities().List && !configured {
		switch {
		case isCloudflareAPI(adapter):
			st.RuleError = "Cloudflare API token not configured (needs scope Account → Cloudflare Tunnel → Read)"
		case isFileBased(adapter):
			st.RuleError = "host file access not available (configure SSH credentials for this node)"
		default:
			st.RuleError = "admin endpoint not configured"
		}
	}
	return st, nil
}

// resolveRuleTargets fills rule.Resolved for each rule whose TargetURL maps to
// a known container. It uses the full cross-node container inventory, the
// cross-node published-port index, and the node list — all already cached in
// the store — so resolution is store-only (no live Docker calls) and handles
// tunnel targets that point at containers on different nodes (e.g. a
// cloudflared rule "http://192.168.20.9:5006" whose container runs on another
// host). Best-effort: store errors leave all rules unresolved (Resolved nil).
func (s *Service) resolveRuleTargets(ctx context.Context, tunnelNodeID string, rules []Rule) {
	if len(rules) == 0 {
		return
	}
	// Fetch all nodes so resolveTargetWithNodes can map a target IP to a node
	// whose Host field stores that IP (cross-node fallback).
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		nodes = nil // non-fatal; direct HostIP matches still work
	}

	// Collect containers from every node. A single ListContainersByNode call per
	// node is the only cross-node option the store exposes; the fan-out is cheap
	// (inventory is small) and avoids adding a new Store method.
	var containers []db.Container
	for _, n := range nodes {
		nc, nerr := s.store.ListContainersByNode(ctx, n.ID)
		if nerr == nil {
			containers = append(containers, nc...)
		}
	}
	if len(containers) == 0 {
		// Fallback: at least load the tunnel node's containers so name matches
		// work even when ListNodes failed.
		containers, _ = s.store.ListContainersByNode(ctx, tunnelNodeID)
	}

	// ListAllPortExposures already returns every node's ports.
	ports, err := s.store.ListAllPortExposures(ctx)
	if err != nil {
		ports = nil // name matches still work without port data
	}

	for i := range rules {
		rules[i].Resolved = resolveTargetWithNodes(rules[i].TargetURL, tunnelNodeID, containers, ports, nodes)
	}
}

// isFileBased reports whether an adapter reads its rules from the host
// filesystem rather than an HTTP admin API.
func isFileBased(a Adapter) bool {
	_, ok := a.(*Cloudflared)
	return ok
}

// isCloudflareAPI reports whether an adapter is the Cloudflare-API provider.
func isCloudflareAPI(a Adapter) bool {
	_, ok := a.(*CloudflareAPI)
	return ok
}

// parseConfigJSON decodes proxy_config.config_json (a flat string map, e.g.
// {"kind":"cloudflare-api","account_id":"..","tunnel_id":".."}). Returns nil on
// empty/invalid input so adapters that don't use config see Conn.Config == nil.
func parseConfigJSON(s string) map[string]string {
	s = strings.TrimSpace(s)
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil || len(m) == 0 {
		return nil
	}
	return m
}

// configKind extracts the "kind" override from a config_json blob (""=none).
func configKind(s string) string {
	return parseConfigJSON(s)["kind"]
}

// enrichMountCandidates queries bind mounts for file-based adapters and adds
// host-path config candidates to the conn. No-op for API-based adapters.
func (s *Service) enrichMountCandidates(ctx context.Context, nodeID, adapterName string, conn *Conn) {
	if adapterName != "cloudflared" {
		return
	}
	mounts, err := s.store.ListMountsByNode(ctx, nodeID)
	if err != nil {
		return
	}
	conn.MountCandidates = mountBasedCandidates(mounts)
}

// SetConfig stores a node's proxy admin endpoint + optional token (sealed) and
// optional non-secret config (kind override + Cloudflare account_id/tunnel_id).
// token: nil keeps the existing sealed token, "" clears it, a value seals it.
// config: nil keeps the existing config_json, non-nil replaces it (an empty map
// clears the kind override, reverting to image auto-detection).
func (s *Service) SetConfig(ctx context.Context, nodeID, endpoint string, token *string, config map[string]string) error {
	existing, _ := s.store.GetProxyConfig(ctx, nodeID)
	cfg := db.ProxyConfig{
		NodeID:         nodeID,
		Endpoint:       endpoint,
		TokenEncrypted: existing.TokenEncrypted,
		ConfigJSON:     existing.ConfigJSON,
	}
	if token != nil {
		if *token == "" {
			cfg.TokenEncrypted = nil
		} else {
			sealed, err := s.cipher.Seal([]byte(*token))
			if err != nil {
				return err
			}
			cfg.TokenEncrypted = sealed
		}
	}
	if config != nil {
		blob, err := json.Marshal(config)
		if err != nil {
			return err
		}
		cfg.ConfigJSON = string(blob)
	}
	return s.store.UpsertProxyConfig(ctx, cfg)
}

// ErrTokenRequired indicates discovery was attempted with no token (neither an
// override nor a stored one). Mapped to 400 by the API layer.
var ErrTokenRequired = errors.New("cloudflare API token required")

// CFDiscovery is the result of probing a token for Cloudflare accounts and the
// tunnels of the effective account (for the setup picker).
type CFDiscovery struct {
	Accounts  []CFAccount `json:"accounts"`
	Tunnels   []CFTunnel  `json:"tunnels"`
	AccountID string      `json:"account_id"` // effective account used for Tunnels
	// Error carries a non-fatal discovery error (e.g. the account list was
	// fetched but listing its tunnels failed) so the picker can still render the
	// accounts while showing what went wrong. Empty on full success.
	Error string `json:"error,omitempty"`
}

// DiscoverCloudflare lists the accounts a token can see and, when the account is
// unambiguous (explicit accountID or a single account), the tunnels in it. The
// token override lets the UI populate the picker before the config is saved;
// when empty, the node's stored token is used. Cloudflare errors (bad token,
// missing scope) are returned verbatim for the UI to surface.
func (s *Service) DiscoverCloudflare(ctx context.Context, nodeID, tokenOverride, accountID string) (CFDiscovery, error) {
	cf := getCloudflareAPIAdapter()
	if cf == nil {
		return CFDiscovery{}, ErrNoAdapter
	}
	conn, _ := s.conn(ctx, nodeID) // stored endpoint + token + config
	if tokenOverride != "" {
		conn.Token = tokenOverride
	}
	if strings.TrimSpace(conn.Token) == "" {
		return CFDiscovery{}, ErrTokenRequired
	}

	accounts, err := cf.ListAccounts(ctx, conn)
	if err != nil {
		return CFDiscovery{}, err
	}
	out := CFDiscovery{Accounts: accounts}

	effective := strings.TrimSpace(accountID)
	if effective == "" && len(accounts) == 1 {
		effective = accounts[0].ID
	}
	if effective != "" {
		// Clone conn with the effective account so ListTunnels targets it.
		tunnelConn := conn
		merged := map[string]string{}
		for k, v := range conn.Config {
			merged[k] = v
		}
		merged["account_id"] = effective
		tunnelConn.Config = merged
		tunnels, terr := cf.ListTunnels(ctx, tunnelConn)
		if terr != nil {
			return out, terr // return accounts already found alongside the error
		}
		out.Tunnels = tunnels
		out.AccountID = effective
	}
	return out, nil
}

// ErrNoAdapter is returned when an action targets a node with no detected proxy.
var ErrNoAdapter = errors.New("proxy: no supported proxy detected on this node")
