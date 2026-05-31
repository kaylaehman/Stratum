package proxy

// resolve.go — maps a rule's TargetURL to a known container, so the UI can
// deep-link from a proxy route straight to the resource it serves.
//
// Matching is intentionally store-only (no live Docker calls): it joins the
// proxy rules against the container inventory + the published-port index that
// Stratum already maintains (db.PortExposureRow, the same source the ports
// audit uses). This keeps Status cheap and side-effect free.

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/kaylaehman/stratum/backend/db"
)

// targetEndpoint is the host+port parsed out of a rule's TargetURL.
type targetEndpoint struct {
	host string // hostname or IP literal; "" when unparseable
	port int    // 0 when no port was present
}

// parseTargetURL extracts the host and port from a proxy target reference. It
// accepts the common forms a proxy config produces:
//
//	http://host:port   https://host:port
//	http://host        https://host         (port defaults to scheme: 80/443)
//	host:port          host                 (scheme-less, as cloudflared/NPM may emit)
//	tcp://host:port                          (any scheme; host:port still extracted)
//
// Returns host="" when nothing usable could be parsed (e.g. "http_status:404").
func parseTargetURL(raw string) targetEndpoint {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return targetEndpoint{}
	}

	// cloudflared catch-all / special services like "http_status:404" are not
	// real endpoints — reject them explicitly (the left part is not a host).
	if strings.HasPrefix(raw, "http_status:") {
		return targetEndpoint{}
	}

	// If there's no scheme, give url.Parse one so it populates Host rather than
	// stuffing everything into Path. We only do this when the string looks like
	// a bare authority (host or host:port), not an absolute path.
	work := raw
	scheme := ""
	if i := strings.Index(work, "://"); i >= 0 {
		scheme = strings.ToLower(work[:i])
	} else {
		work = "//" + work
	}

	u, err := url.Parse(work)
	if err != nil || u.Host == "" {
		// Fall back to a manual host:port split for inputs url.Parse rejects.
		return splitHostPort(strings.TrimPrefix(raw, scheme+"://"))
	}

	host := u.Hostname() // strips brackets from IPv6 and drops any port
	portStr := u.Port()
	port := 0
	if portStr != "" {
		port, _ = strconv.Atoi(portStr)
	} else {
		switch scheme {
		case "https":
			port = 443
		case "http":
			port = 80
		}
	}
	return targetEndpoint{host: host, port: port}
}

// splitHostPort is a permissive fallback that pulls host+port from a bare
// "host:port" or "host" string when url.Parse can't.
func splitHostPort(s string) targetEndpoint {
	s = strings.TrimSpace(s)
	if s == "" {
		return targetEndpoint{}
	}
	// Strip any path/query suffix.
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if h, p, err := net.SplitHostPort(s); err == nil {
		port, _ := strconv.Atoi(p)
		return targetEndpoint{host: h, port: port}
	}
	return targetEndpoint{host: strings.Trim(s, "[]")}
}

// loopbackHosts are the host forms that mean "this node" for a localhost_port
// match. 0.0.0.0 / :: are included: a proxy pointing at 0.0.0.0:PORT is really
// pointing at a port published on this host.
var loopbackHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
	"0.0.0.0":   true,
	"::":        true,
}

// resolveTarget matches a single rule's TargetURL against this node's container
// inventory + published-port index. Returns nil when no confident match exists.
//
// Match precedence (first match wins):
//   - localhost/loopback host + port → container on this node publishing that
//     HostPort                                            → MatchLocalhostPort
//   - non-IP hostname → container on this node named exactly host
//     (Docker service/container name)                     → MatchContainerName
//   - IP literal (non-loopback) + port → container on this node whose published
//     binding HostIP == that IP and HostPort == port      → MatchHostIPPort
//
// v1 limitation: matching is scoped to the CURRENT node only. The Cloudflared
// "http://service:port" form virtually always refers to a container on the same
// Docker host as the tunnel, and localhost/IP forms are host-local by
// definition, so current-node scope resolves the realistic cases.
func resolveTarget(targetURL, nodeID string, containers []db.Container, ports []db.PortExposureRow) *ResolvedTarget {
	ep := parseTargetURL(targetURL)
	if ep.host == "" {
		return nil
	}

	host := strings.ToLower(ep.host)
	ip := net.ParseIP(ep.host)

	switch {
	case loopbackHosts[host] && ep.port > 0:
		if c := containerByPublishedPort(nodeID, ep.port, "", containers, ports); c != nil {
			return &ResolvedTarget{NodeID: nodeID, ContainerID: c.ID, Name: c.Name, MatchKind: MatchLocalhostPort}
		}

	case ip == nil: // a name, not an IP literal
		if c := containerByName(nodeID, ep.host, containers); c != nil {
			return &ResolvedTarget{NodeID: nodeID, ContainerID: c.ID, Name: c.Name, MatchKind: MatchContainerName}
		}

	case ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() && ep.port > 0:
		if c := containerByPublishedPort(nodeID, ep.port, ep.host, containers, ports); c != nil {
			return &ResolvedTarget{NodeID: nodeID, ContainerID: c.ID, Name: c.Name, MatchKind: MatchHostIPPort}
		}
	}
	return nil
}

// containerByName returns the container on nodeID whose Name equals host
// (case-insensitive), or nil. Docker container/service names are the host part
// of intra-network URLs like "http://jellyfin:8096".
func containerByName(nodeID, host string, containers []db.Container) *db.Container {
	for i := range containers {
		if containers[i].NodeID == nodeID && strings.EqualFold(containers[i].Name, host) {
			return &containers[i]
		}
	}
	return nil
}

// containerByPublishedPort returns the container on nodeID that publishes
// hostPort. When wantIP is non-empty the binding's HostIP must equal it (the
// host_ip_port case); when wantIP is "" any binding on that port matches (the
// localhost_port case — a port published to 0.0.0.0/127.0.0.1 is reachable as
// localhost). Returns nil when no container in inventory owns the matched port.
func containerByPublishedPort(nodeID string, hostPort int, wantIP string, containers []db.Container, ports []db.PortExposureRow) *db.Container {
	for _, p := range ports {
		if p.NodeID != nodeID || p.HostPort != hostPort {
			continue
		}
		if wantIP != "" && p.HostIP != wantIP {
			continue
		}
		if c := containerByID(nodeID, p.ContainerID, containers); c != nil {
			return c
		}
	}
	return nil
}

// containerByID returns the container with inventory ID id on nodeID, or nil.
func containerByID(nodeID, id string, containers []db.Container) *db.Container {
	for i := range containers {
		if containers[i].NodeID == nodeID && containers[i].ID == id {
			return &containers[i]
		}
	}
	return nil
}
