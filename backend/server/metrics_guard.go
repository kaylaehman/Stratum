package server

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

// metricsGuard protects the Prometheus /metrics endpoint, which exposes internal
// operational detail (node/container counts, scan timings, error rates) that
// should not be world-readable.
//
//   - When a token is configured, require `Authorization: Bearer <token>`
//     (constant-time compare) — this is what a remote Prometheus uses via its
//     scrape config's bearer_token.
//   - When no token is set, fall back to loopback-only: same-host scrapers (a
//     Prometheus sidecar, an SSH tunnel) keep working, but the endpoint is never
//     exposed on a public/LAN interface by default. For remote scraping across
//     hosts, set STRATUM_METRICS_TOKEN.
func metricsGuard(token string, next http.Handler) http.Handler {
	token = strings.TrimSpace(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" {
			const prefix = "Bearer "
			hdr := r.Header.Get("Authorization")
			presented := strings.TrimPrefix(hdr, prefix)
			if !strings.HasPrefix(hdr, prefix) ||
				subtle.ConstantTimeCompare([]byte(presented), []byte(token)) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		} else if !isLoopbackAddr(r.RemoteAddr) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// isLoopbackAddr reports whether an http.Request RemoteAddr ("host:port") is a
// loopback IP.
func isLoopbackAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
