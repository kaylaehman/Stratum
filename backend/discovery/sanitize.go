// Package discovery probes a host to detect its type and capabilities. This
// file holds error sanitization: transport errors often embed host/port/user/
// URL context, which must never be persisted to nodes.last_error or returned in
// API responses. SanitizeProbeError maps any error to a stable category and a
// short, host-free, user-actionable hint.
package discovery

import "strings"

// Probe error categories. These are the only strings ever persisted/returned.
const (
	// SSH auth — subcategorized so the UI can guide the user to the actual fix.
	// Generic ssh_auth_failed is the fallback when we can't tell the sub-cause.
	ErrCategorySSHAuthFailed         = "ssh_auth_failed"
	ErrCategorySSHAuthPubkeyRejected = "ssh_auth_pubkey_rejected"
	ErrCategorySSHAuthPasswordWrong  = "ssh_auth_password_rejected"
	ErrCategorySSHPassphraseRequired = "ssh_passphrase_required"
	ErrCategorySSHPassphraseWrong    = "ssh_passphrase_wrong"

	ErrCategorySSHHostKey     = "ssh_host_key_mismatch"
	ErrCategorySSHUnreachable = "ssh_unreachable"

	// ssh_target_is_self — the probe target resolves to the machine Stratum
	// itself runs on. Distinct from ssh_unreachable: we never dialed, so a
	// "couldn't connect" hint would be misleading.
	ErrCategorySSHTargetSelf = "ssh_target_is_self"
	// ssh_detect_failed — the SSH session was established and the host key was
	// captured, but the capability-detection step didn't complete. Distinct
	// from ssh_unreachable, which has a specific TCP-layer meaning.
	ErrCategorySSHDetectFailed = "ssh_detect_failed"

	ErrCategoryDockerUnreachable  = "docker_unreachable"
	ErrCategoryTLS                = "tls_error"
	ErrCategoryProxmoxUnauthed    = "proxmox_unauthed"
	ErrCategoryProxmoxUnreachable = "proxmox_unreachable"
	ErrCategoryTimeout            = "timeout"
	ErrCategoryUnknown            = "unknown_error"
)

// SanitizeProbeError maps a raw transport error to a stable category and a
// short hint suitable for display in the UI. The category never leaks
// host/port/user/URL detail; the hint is hand-authored per category and is
// likewise host-free.
//
// Matching is on lowercased substrings of the error text. Order matters: more
// specific signals (passphrase, pubkey, host key) are checked before generic
// ones (auth, unreachable, timeout).
//
// Returns ("", "") for a nil error.
func SanitizeProbeError(err error) (category, hint string) {
	if err == nil {
		return "", ""
	}
	msg := strings.ToLower(err.Error())

	switch {
	// Self-target and detection-after-handshake are checked first: both can
	// otherwise be swallowed by the generic "ssh:" / "dial" cases below and
	// mislabeled as ssh_unreachable, which carries a specific TCP-layer meaning.
	case strings.Contains(msg, "stratum host itself"):
		return ErrCategorySSHTargetSelf, hintFor(ErrCategorySSHTargetSelf)
	case strings.Contains(msg, "detection session failed"):
		return ErrCategorySSHDetectFailed, hintFor(ErrCategorySSHDetectFailed)

	// SSH key file problems surface before the network handshake — the
	// x/crypto/ssh package returns "ssh: this private key is passphrase
	// protected" when ParsePrivateKey is called on an encrypted key without
	// one, and "x509: decryption password incorrect" (or similar) when the
	// supplied passphrase is wrong.
	case strings.Contains(msg, "passphrase protected") ||
		strings.Contains(msg, "encrypted") && strings.Contains(msg, "key"):
		return ErrCategorySSHPassphraseRequired, hintFor(ErrCategorySSHPassphraseRequired)
	case strings.Contains(msg, "decryption password") ||
		strings.Contains(msg, "incorrect passphrase") ||
		strings.Contains(msg, "x509: decryption password"):
		return ErrCategorySSHPassphraseWrong, hintFor(ErrCategorySSHPassphraseWrong)

	case strings.Contains(msg, "knownhosts") || strings.Contains(msg, "host key mismatch") ||
		strings.Contains(msg, "key is unknown") || strings.Contains(msg, "host key for"):
		return ErrCategorySSHHostKey, hintFor(ErrCategorySSHHostKey)

	// SSH auth — narrow first. x/crypto/ssh exposes "attempted methods
	// [none publickey]" or "[none password]" in its handshake error when the
	// server rejects every method it offered, which lets us tell which
	// credential type was actually tried.
	case strings.Contains(msg, "attempted methods") && strings.Contains(msg, "publickey"):
		return ErrCategorySSHAuthPubkeyRejected, hintFor(ErrCategorySSHAuthPubkeyRejected)
	case strings.Contains(msg, "attempted methods") && strings.Contains(msg, "password"):
		return ErrCategorySSHAuthPasswordWrong, hintFor(ErrCategorySSHAuthPasswordWrong)
	case strings.Contains(msg, "unable to authenticate") || strings.Contains(msg, "auth fail") ||
		strings.Contains(msg, "permission denied") || strings.Contains(msg, "no supported methods"):
		return ErrCategorySSHAuthFailed, hintFor(ErrCategorySSHAuthFailed)

	case strings.Contains(msg, "x509") || strings.Contains(msg, "tls") ||
		strings.Contains(msg, "certificate") ||
		// Docker TLS config build failures (CA PEM won't parse, or a client cert
		// was supplied without its key). These surface BEFORE any network I/O, so
		// they must be categorized as a TLS/cert problem rather than falling
		// through to docker_unreachable on the "docker" substring below.
		strings.Contains(msg, "ca pem") ||
		(strings.Contains(msg, "client cert") && strings.Contains(msg, "key")):
		return ErrCategoryTLS, hintFor(ErrCategoryTLS)
	case strings.Contains(msg, "401") || strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "authentication failure"):
		return ErrCategoryProxmoxUnauthed, hintFor(ErrCategoryProxmoxUnauthed)
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "i/o timeout"):
		return ErrCategoryTimeout, hintFor(ErrCategoryTimeout)
	case strings.Contains(msg, "ssh:") || strings.Contains(msg, "handshake"):
		return ErrCategorySSHUnreachable, hintFor(ErrCategorySSHUnreachable)
	case strings.Contains(msg, "docker") || strings.Contains(msg, "/var/run/docker.sock"):
		return ErrCategoryDockerUnreachable, hintFor(ErrCategoryDockerUnreachable)
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "no route to host") ||
		strings.Contains(msg, "no such host") || strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "connectex") || strings.Contains(msg, "dial"):
		return ErrCategorySSHUnreachable, hintFor(ErrCategorySSHUnreachable)
	default:
		return ErrCategoryUnknown, hintFor(ErrCategoryUnknown)
	}
}

// hintFor maps a category to its short, host-free, user-actionable hint.
// Hints are intentionally specific: the goal is the user knowing what to
// change next, not a generic "auth failed" restated.
func hintFor(cat string) string {
	switch cat {
	case ErrCategorySSHAuthPubkeyRejected:
		return "The host rejected your public key. Confirm the matching public key is in the user's ~/.ssh/authorized_keys on the target, that the file is mode 600 and ~/.ssh is mode 700, and that PubkeyAuthentication is enabled in sshd_config."
	case ErrCategorySSHAuthPasswordWrong:
		return "Password rejected. Verify the password is correct and that PasswordAuthentication is enabled in sshd_config (it's commonly disabled by default on cloud images)."
	case ErrCategorySSHPassphraseRequired:
		return "This private key is encrypted but no passphrase was supplied. Fill in the Key passphrase field."
	case ErrCategorySSHPassphraseWrong:
		return "The passphrase didn't decrypt the private key. Confirm it matches what you used at ssh-keygen time."
	case ErrCategorySSHAuthFailed:
		return "SSH authentication failed. Check the username, the credential type (key vs password), and whether the target permits the chosen method (e.g. PermitRootLogin, PubkeyAuthentication)."
	case ErrCategorySSHHostKey:
		return "The host presented a key that doesn't match the one previously pinned for this node. If you intentionally re-keyed the host, re-probe to inspect the new fingerprint before accepting."
	case ErrCategorySSHUnreachable:
		return "Couldn't establish a TCP connection to the SSH port. Verify the host, port, and that the SSH daemon is running and reachable from this network."
	case ErrCategorySSHTargetSelf:
		return "This host resolves to the machine Stratum itself runs on. Stratum can't register its own host over SSH through this probe path — manage the local host directly, or register it from a different Stratum instance."
	case ErrCategorySSHDetectFailed:
		return "Connected and captured the host key, but the capability-detection command didn't complete. The SSH transport works — confirm the login user can run a non-interactive shell command (no forced command or restricted shell) and re-probe."
	case ErrCategoryDockerUnreachable:
		return "The Docker endpoint didn't respond. Confirm the daemon is running and the endpoint URL is correct."
	case ErrCategoryTLS:
		return "TLS verification failed. The server's certificate is untrusted or invalid; if intentional (homelab self-signed), enable the insecure flag for this endpoint."
	case ErrCategoryProxmoxUnauthed:
		return "Proxmox rejected the API token. Check the token ID, secret, and that the token has at least PVEAuditor on /."
	case ErrCategoryProxmoxUnreachable:
		return "The Proxmox API endpoint didn't respond. Verify the URL (https://host:8006) and that the network is reachable."
	case ErrCategoryTimeout:
		return "The probe timed out before the host responded. Network is slow, blocked, or the host is offline."
	default:
		return "The probe failed for an unknown reason. Check the backend logs for the full error."
	}
}
