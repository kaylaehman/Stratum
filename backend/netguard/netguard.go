// Package netguard provides an SSRF-hardened HTTP transport. It is used for
// egress to endpoints whose address is influenced by configuration (e.g. the AI
// provider base URL), where a hostile or fat-fingered value could otherwise
// point a server-side request at cloud-metadata or internal services.
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// blocked reports whether ip is in a range that must not be reachable via a
// server-side request: private LAN (10/8, 172.16/12, 192.168/16, fc00::/7),
// link-local (169.254/16 — including the 169.254.169.254 cloud-metadata
// endpoint — and fe80::/10), or the unspecified address.
//
// Loopback is deliberately NOT blocked: a local Ollama runs there, and loopback
// is only reachable from this host anyway, so it carries little SSRF value. A
// blanket internal-IP block would break the common local-Ollama setup.
func blocked(ip net.IP) bool {
	return ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate()
}

// Transport clones http.DefaultTransport and replaces its dialer with one that
// resolves the target host, refuses any resolved address that blocked() rejects,
// and then dials that exact IP. Dialing the validated IP (rather than the
// hostname) closes the validate-then-dial TOCTOU window, so a name that resolves
// to an internal address — including via DNS rebinding after an earlier URL
// check — is still refused.
//
// Hosts in allow bypass the IP check (matched case-insensitively against the
// pre-resolution hostname). Use it to permit an intentionally-internal endpoint,
// e.g. a LAN-hosted Ollama at 192.168.x.y. Loopback hosts never need to be in
// allow.
func Transport(allow []string) *http.Transport {
	allowSet := make(map[string]struct{}, len(allow))
	for _, h := range allow {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			allowSet[h] = struct{}{}
		}
	}
	base := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if _, ok := allowSet[strings.ToLower(host)]; ok {
			return base.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ipa := range ips {
			if blocked(ipa.IP) {
				lastErr = fmt.Errorf("netguard: refusing to connect to internal address %s (host %q)", ipa.IP, host)
				continue
			}
			conn, derr := base.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
			if derr == nil {
				return conn, nil
			}
			lastErr = derr
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("netguard: no dialable address for %q", host)
		}
		return nil, lastErr
	}
	return t
}
