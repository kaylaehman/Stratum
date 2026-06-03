package proxy

// container.go — the container-scoped view of reverse proxying. It answers two
// questions for a single container:
//
//   - which public hostnames already route to it (serving routes), and
//   - can a new route be added, to which proxy, and toward what target URL.
//
// The serving-route data is the same rule→container resolution the node panel
// uses (Rule.Resolved), filtered to one container. Adding a route delegates to
// the detected proxy adapter's CreateRule (today: cloudflare-api).

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kaylaehman/stratum/backend/db"
)

// ContainerProxyStatus is the API view for one container's reverse-proxy state.
type ContainerProxyStatus struct {
	ContainerID string `json:"container_id"`
	// Routes are the proxy rules whose target resolves to this container.
	Routes []Rule `json:"routes"`
	// AddTargets are the proxies (on any node) that can add a route to this
	// container. Empty when no create-capable proxy is configured.
	AddTargets []AddTarget `json:"add_targets"`
	// SuggestedTargets are candidate service URLs (http://host:port) derived from
	// the container's published ports, to pre-fill the add form.
	SuggestedTargets []string `json:"suggested_targets"`
}

// AddTarget is a proxy a route could be added to.
type AddTarget struct {
	NodeID       string       `json:"node_id"`
	NodeName     string       `json:"node_name"`
	Adapter      string       `json:"adapter"`
	Capabilities Capabilities `json:"capabilities"`
	// CFTunnelID echoes the cloudflare-api tunnel a route would be added to (so
	// the UI can show context). Empty for other adapters.
	CFTunnelID string `json:"cf_tunnel_id,omitempty"`
}

// AddRouteRequest is the input for adding a route to a container.
type AddRouteRequest struct {
	ProxyNodeID string
	SourceHost  string
	TargetURL   string
	Path        string
	CreateDNS   bool
	DryRun      bool
}

// AddRoutePlan is the result of an add (or a dry-run preview of one).
type AddRoutePlan struct {
	ProxyNodeID string `json:"proxy_node_id"`
	Adapter     string `json:"adapter"`
	SourceHost  string `json:"source_host"`
	TargetURL   string `json:"target_url"`
	Path        string `json:"path,omitempty"`
	CreateDNS   bool   `json:"create_dns"`
	// DNSRecord is a human-readable preview of the CNAME that would be created
	// (cloudflare-api only); empty when DNS isn't being touched.
	DNSRecord string `json:"dns_record,omitempty"`
	// Applied is true when the route was actually written (false for a dry-run).
	Applied bool  `json:"applied"`
	Rule    *Rule `json:"rule,omitempty"`
	// Warning carries a non-fatal post-write issue (e.g. the route was added but
	// its DNS record could not be ensured).
	Warning string `json:"warning,omitempty"`
}

// ErrProxyNotConfigured indicates the chosen proxy node has no usable connection
// (no token / endpoint / file access). Mapped to 400 by the API layer.
var ErrProxyNotConfigured = errors.New("proxy: selected proxy is not configured on this node")

// ErrCannotAddRoute indicates the chosen node has no create-capable proxy.
var ErrCannotAddRoute = errors.New("proxy: this node's proxy cannot add routes")

// configured reports whether an adapter has everything it needs to act on a
// node. Mirrors the logic in Status so the two stay consistent.
func (s *Service) configured(adapter Adapter, conn Conn, endpointConfigured bool) bool {
	if isCloudflareAPI(adapter) {
		return conn.Token != ""
	}
	if isFileBased(adapter) {
		return conn.ReadFile != nil
	}
	return endpointConfigured
}

// ContainerProxy gathers the serving routes + add targets for one container.
func (s *Service) ContainerProxy(ctx context.Context, containerID string) (ContainerProxyStatus, error) {
	out := ContainerProxyStatus{
		ContainerID:      containerID,
		Routes:           []Rule{},
		AddTargets:       []AddTarget{},
		SuggestedTargets: []string{},
	}
	c, err := s.store.GetContainer(ctx, containerID)
	if err != nil {
		return out, err
	}
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return out, err
	}
	out.SuggestedTargets = s.suggestedTargets(ctx, c, nodes)

	for _, n := range nodes {
		adapter, derr := s.detect(ctx, n.ID)
		if derr != nil || adapter == nil {
			continue
		}
		conn, endpointConfigured := s.conn(ctx, n.ID)
		s.enrichMountCandidates(ctx, n.ID, adapter.Name(), &conn)
		if !s.configured(adapter, conn, endpointConfigured) {
			continue
		}

		if adapter.Capabilities().List {
			rules, lerr := adapter.ListRules(ctx, conn)
			if lerr == nil {
				s.resolveRuleTargets(ctx, n.ID, rules)
				for _, r := range rules {
					if r.Resolved != nil && r.Resolved.ContainerID == containerID {
						out.Routes = append(out.Routes, r)
					}
				}
			}
		}
		if adapter.Capabilities().Create {
			at := AddTarget{
				NodeID:       n.ID,
				NodeName:     n.Name,
				Adapter:      adapter.Name(),
				Capabilities: adapter.Capabilities(),
			}
			if isCloudflareAPI(adapter) {
				at.CFTunnelID = conn.Config["tunnel_id"]
			}
			out.AddTargets = append(out.AddTargets, at)
		}
	}
	return out, nil
}

// suggestedTargets builds http://host:port URLs from the container's published
// ports, using a specific bind IP when present, else the owning node's Host.
func (s *Service) suggestedTargets(ctx context.Context, c db.Container, nodes []db.Node) []string {
	ports, err := s.store.ListAllPortExposures(ctx)
	if err != nil {
		return []string{}
	}
	ownerHost := ""
	for _, n := range nodes {
		if n.ID == c.NodeID {
			ownerHost = n.Host
			break
		}
	}
	seen := map[string]bool{}
	type cand struct {
		url  string
		port int
	}
	var cands []cand
	for _, p := range ports {
		if p.ContainerID != c.ID || p.NodeID != c.NodeID || p.HostPort == 0 {
			continue
		}
		// A loopback-only bind (127.0.0.1 / ::1) is not reachable from a proxy on
		// another host, so it's not a useful suggestion — skip it. A wildcard bind
		// (0.0.0.0 / ::, or empty) is reachable via the node's host; a specific
		// non-loopback IP is used directly.
		host := ownerHost
		switch p.HostIP {
		case "127.0.0.1", "::1":
			continue
		case "", "0.0.0.0", "::":
			// host stays the owning node's address
		default:
			host = p.HostIP
		}
		if host == "" {
			continue
		}
		u := fmt.Sprintf("http://%s:%d", host, p.HostPort)
		if seen[u] {
			continue
		}
		seen[u] = true
		cands = append(cands, cand{u, p.HostPort})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].port < cands[j].port })
	urls := make([]string, 0, len(cands))
	for _, c := range cands {
		urls = append(urls, c.url)
	}
	return urls
}

// AddContainerProxy adds (or previews) a reverse-proxy route to a container via
// the chosen proxy node's adapter. A dry-run returns the computed plan — the
// ingress entry and the DNS record that would be created — without any write.
func (s *Service) AddContainerProxy(ctx context.Context, req AddRouteRequest) (AddRoutePlan, error) {
	plan := AddRoutePlan{
		ProxyNodeID: req.ProxyNodeID,
		SourceHost:  req.SourceHost,
		TargetURL:   req.TargetURL,
		Path:        req.Path,
		CreateDNS:   req.CreateDNS,
	}
	adapter, err := s.detect(ctx, req.ProxyNodeID)
	if err != nil {
		return plan, err
	}
	if adapter == nil || !adapter.Capabilities().Create {
		return plan, ErrCannotAddRoute
	}
	plan.Adapter = adapter.Name()

	conn, endpointConfigured := s.conn(ctx, req.ProxyNodeID)
	s.enrichMountCandidates(ctx, req.ProxyNodeID, adapter.Name(), &conn)
	if !s.configured(adapter, conn, endpointConfigured) {
		return plan, ErrProxyNotConfigured
	}

	// Validate up front so a dry-run rejects bad input too (CreateRule validates
	// again on the real path). The validators carry ErrInvalidHostname /
	// ErrInvalidService sentinels the API maps to 400.
	if err := validateProxyHostname(strings.ToLower(strings.TrimSpace(req.SourceHost))); err != nil {
		return plan, err
	}
	if err := validateProxyService(strings.TrimSpace(req.TargetURL)); err != nil {
		return plan, err
	}

	if isCloudflareAPI(adapter) && req.CreateDNS {
		plan.DNSRecord = fmt.Sprintf("%s  CNAME  %s%s  (proxied)",
			req.SourceHost, conn.Config["tunnel_id"], cfTunnelCNAMESuffix)
	}

	if req.DryRun {
		return plan, nil
	}

	// Thread the create_dns choice through a cloned Config so we never mutate the
	// shared connection config.
	cfg := make(map[string]string, len(conn.Config)+1)
	for k, v := range conn.Config {
		cfg[k] = v
	}
	if !req.CreateDNS {
		cfg["create_dns"] = "false"
	}
	conn.Config = cfg

	rule, err := adapter.CreateRule(ctx, conn, Rule{
		SourceHost: req.SourceHost,
		TargetURL:  req.TargetURL,
		SourcePath: req.Path,
	})
	if err != nil {
		// A DNS warning means the route WAS added — report it as a partial
		// success with a warning rather than a hard failure.
		if errors.Is(err, ErrDNSWarning) {
			plan.Applied = true
			plan.Rule = &rule
			plan.Warning = err.Error()
			return plan, nil
		}
		return plan, err
	}
	plan.Applied = true
	plan.Rule = &rule
	return plan, nil
}
