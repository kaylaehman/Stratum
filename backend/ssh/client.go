package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// ErrHostKeyMismatch is returned (wrapped) by Detect when the presented host key
// does not match the pinned key. Callers detect it with errors.Is rather than
// matching error text. Its message contains "host key mismatch" so error
// sanitizers can also categorize it.
var ErrHostKeyMismatch = errors.New("ssh: host key mismatch")

// Credentials for an SSH dial. Exactly one of Password / PrivateKeyPEM is used,
// per Method semantics handled by the caller.
type Credentials struct {
	User          string
	Password      string // for password auth
	PrivateKeyPEM string // for key auth (PEM)
	Passphrase    string // optional, for an encrypted private key
}

// Detection holds the positional results of the single chained detection command.
type Detection struct {
	OSReleaseRaw  string // raw contents of /etc/os-release (may be "")
	HasSystemctl  bool   // `command -v systemctl` succeeded
	HasRunSystemd bool   // `/run/systemd/system` dir exists
	HasCrontab    bool   // `command -v crontab` succeeded
	HasDocker     bool   // `command -v docker` succeeded
	HasEtcPve     bool   // `/etc/pve` dir exists (Proxmox marker)
}

// HostKey is the presented SSH host key, for TOFU.
type HostKey struct {
	SHA256         string // ssh.FingerprintSHA256 of the presented key (show to operator)
	KnownHostsLine string // knownhosts line to persist as nodes.ssh_host_key
}

const detectionCmd = `cat /etc/os-release 2>/dev/null; echo "---STRATUM---"; command -v systemctl >/dev/null 2>&1 && echo systemctl; test -d /run/systemd/system && echo run_systemd; command -v crontab >/dev/null 2>&1 && echo crontab; command -v docker >/dev/null 2>&1 && echo docker; test -d /etc/pve && echo etc_pve`

const stratumMarker = "---STRATUM---"

const defaultDialTimeout = 8 * time.Second

// Detect dials host:port with strict host-key handling, runs the chained
// detection command in ONE session, and returns parsed results + the host key.
//
//   pinnedKnownHostsLine == "" -> FIRST CONNECT (TOFU): capture & ACCEPT the
//       presented host key, returning it in HostKey for the operator to confirm.
//   pinnedKnownHostsLine != "" -> verify the presented key against the pinned
//       knownhosts line; if it differs, return an error whose text contains
//       "host key mismatch" (HARD FAIL — never silently accept).
//
// NEVER use ssh.InsecureIgnoreHostKey(). A HostKeyCallback is mandatory.
func Detect(ctx context.Context, host string, port int, creds Credentials, pinnedKnownHostsLine string) (Detection, HostKey, error) {
	authMethod, err := buildAuthMethod(creds)
	if err != nil {
		return Detection{}, HostKey{}, fmt.Errorf("ssh: build auth method: %w", err)
	}

	var capturedHostKey HostKey
	var hostKeyMismatch bool
	hostKeyCallback, err := buildHostKeyCallback(host, port, pinnedKnownHostsLine, &capturedHostKey, &hostKeyMismatch)
	if err != nil {
		return Detection{}, HostKey{}, fmt.Errorf("ssh: build host key callback: %w", err)
	}

	cfg := &ssh.ClientConfig{
		User:            creds.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
		Timeout:         dialTimeout(ctx),
	}

	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		// Detect the mismatch via the callback-set flag, not the error chain,
		// so it is reliable regardless of how the ssh library wraps the error.
		if hostKeyMismatch {
			return Detection{}, HostKey{}, fmt.Errorf("%w (dial %s)", ErrHostKeyMismatch, addr)
		}
		return Detection{}, HostKey{}, fmt.Errorf("ssh: dial %s: %w", addr, err)
	}
	defer client.Close()

	output, err := runSession(ctx, client)
	if err != nil {
		return Detection{}, HostKey{}, fmt.Errorf("ssh: run detection: %w", err)
	}

	return parseDetection(output), capturedHostKey, nil
}

// buildAuthMethod constructs an ssh.AuthMethod from the given credentials.
func buildAuthMethod(creds Credentials) (ssh.AuthMethod, error) {
	if creds.PrivateKeyPEM != "" {
		signer, err := parsePrivateKey(creds.PrivateKeyPEM, creds.Passphrase)
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	}
	return ssh.Password(creds.Password), nil
}

// parsePrivateKey parses a PEM-encoded private key, with optional passphrase.
func parsePrivateKey(pemData, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase([]byte(pemData), []byte(passphrase))
	}
	return ssh.ParsePrivateKey([]byte(pemData))
}

// buildHostKeyCallback returns the appropriate HostKeyCallback for TOFU or pinned-key verification.
func buildHostKeyCallback(host string, port int, pinnedLine string, captured *HostKey, mismatch *bool) (ssh.HostKeyCallback, error) {
	if pinnedLine == "" {
		return toFUCallback(host, port, captured), nil
	}
	return pinnedCallback(pinnedLine, mismatch)
}

// toFUCallback records the presented host key and always accepts it (TOFU).
func toFUCallback(host string, port int, captured *HostKey) ssh.HostKeyCallback {
	normalized := knownhosts.Normalize(net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		captured.SHA256 = ssh.FingerprintSHA256(key)
		captured.KnownHostsLine = knownhosts.Line([]string{normalized}, key)
		return nil
	}
}

// pinnedCallback verifies the presented key matches the pinned known_hosts line.
// On mismatch it sets *mismatch and returns an error wrapping ErrHostKeyMismatch.
func pinnedCallback(pinnedLine string, mismatch *bool) (ssh.HostKeyCallback, error) {
	pinnedKey, err := parsePinnedKey(pinnedLine)
	if err != nil {
		return nil, fmt.Errorf("parse pinned known_hosts line: %w", err)
	}

	return func(_ string, _ net.Addr, presented ssh.PublicKey) error {
		if !bytes.Equal(pinnedKey.Marshal(), presented.Marshal()) {
			*mismatch = true
			return fmt.Errorf("%w: presented fingerprint %s", ErrHostKeyMismatch, ssh.FingerprintSHA256(presented))
		}
		return nil
	}, nil
}

// parsePinnedKey extracts the public key from a known_hosts line.
func parsePinnedKey(line string) (ssh.PublicKey, error) {
	_, _, pubKey, _, _, err := ssh.ParseKnownHosts([]byte(line))
	if err != nil {
		return nil, fmt.Errorf("ssh.ParseKnownHosts: %w", err)
	}
	return pubKey, nil
}

// dialTimeout derives a timeout from the context deadline, falling back to 8s.
func dialTimeout(ctx context.Context) time.Duration {
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d > 0 {
			return d
		}
	}
	return defaultDialTimeout
}

// runSession opens a single SSH session, runs the detection command, and returns stdout.
func runSession(ctx context.Context, client *ssh.Client) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("new session: %w", err)
	}
	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf

	done := make(chan error, 1)
	go func() {
		done <- session.Run(detectionCmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return "", ctx.Err()
	case err := <-done:
		if err != nil {
			return "", fmt.Errorf("session run: %w", err)
		}
	}

	return buf.String(), nil
}

// parseDetection parses the raw output of the detection command.
// Text before "---STRATUM---" is OSReleaseRaw; after it, token lines set booleans.
func parseDetection(raw string) Detection {
	before, after, found := strings.Cut(raw, stratumMarker)
	if !found {
		// Fallback: treat everything as os-release, no tokens.
		return Detection{OSReleaseRaw: strings.TrimSpace(raw)}
	}

	d := Detection{
		OSReleaseRaw: strings.TrimSpace(before),
	}

	for _, line := range strings.Split(after, "\n") {
		switch strings.TrimSpace(line) {
		case "systemctl":
			d.HasSystemctl = true
		case "run_systemd":
			d.HasRunSystemd = true
		case "crontab":
			d.HasCrontab = true
		case "docker":
			d.HasDocker = true
		case "etc_pve":
			d.HasEtcPve = true
		}
	}

	return d
}
