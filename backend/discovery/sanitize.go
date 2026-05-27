// Package discovery probes a host to detect its type and capabilities. This
// file holds error sanitization: transport errors often embed host/port/user/
// URL context, which must never be persisted to nodes.last_error or returned in
// API responses. SanitizeProbeError maps any error to a stable category.
package discovery

import "strings"

// Probe error categories. These are the only strings ever persisted/returned.
const (
	ErrCategorySSHAuthFailed    = "ssh_auth_failed"
	ErrCategorySSHHostKey       = "ssh_host_key_mismatch"
	ErrCategorySSHUnreachable   = "ssh_unreachable"
	ErrCategoryDockerUnreachable = "docker_unreachable"
	ErrCategoryTLS              = "tls_error"
	ErrCategoryProxmoxUnauthed  = "proxmox_unauthed"
	ErrCategoryProxmoxUnreachable = "proxmox_unreachable"
	ErrCategoryTimeout          = "timeout"
	ErrCategoryUnknown          = "unknown_error"
)

// SanitizeProbeError maps a raw transport error to a stable category string,
// stripping any host/port/username/URL detail. Returns "" for a nil error.
//
// Matching is on lowercased substrings of the error text. Order matters: more
// specific signals (auth, host key) are checked before generic ones
// (unreachable, timeout).
func SanitizeProbeError(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(msg, "knownhosts") || strings.Contains(msg, "host key mismatch") ||
		strings.Contains(msg, "key is unknown") || strings.Contains(msg, "host key for"):
		return ErrCategorySSHHostKey
	case strings.Contains(msg, "unable to authenticate") || strings.Contains(msg, "auth fail") ||
		strings.Contains(msg, "permission denied") || strings.Contains(msg, "no supported methods"):
		return ErrCategorySSHAuthFailed
	case strings.Contains(msg, "x509") || strings.Contains(msg, "tls") ||
		strings.Contains(msg, "certificate"):
		return ErrCategoryTLS
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "authentication failure"):
		return ErrCategoryProxmoxUnauthed
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "i/o timeout"):
		return ErrCategoryTimeout
	case strings.Contains(msg, "ssh:") || strings.Contains(msg, "handshake"):
		return ErrCategorySSHUnreachable
	case strings.Contains(msg, "docker") || strings.Contains(msg, "/var/run/docker.sock"):
		return ErrCategoryDockerUnreachable
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "no such host") || strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "connectex") || strings.Contains(msg, "dial"):
		return ErrCategorySSHUnreachable
	default:
		return ErrCategoryUnknown
	}
}
