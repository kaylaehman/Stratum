package ssh

import (
	"context"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// Dial opens a live SSH client to host:port, verifying the presented host key
// against the pinned knownhosts line (hard-fail on mismatch via the same
// callback Detect uses). The caller is responsible for closing the returned
// client. Used by the SFTP layer (SP3); host-key TOFU acceptance happens at
// node registration (SP1), so Dial always pins.
func Dial(ctx context.Context, host string, port int, creds Credentials, pinnedKnownHostsLine string) (*ssh.Client, error) {
	authMethod, err := buildAuthMethod(creds)
	if err != nil {
		return nil, fmt.Errorf("ssh: build auth method: %w", err)
	}
	var captured HostKey
	var mismatch bool
	cb, err := buildHostKeyCallback(host, port, pinnedKnownHostsLine, &captured, &mismatch)
	if err != nil {
		return nil, fmt.Errorf("ssh: build host key callback: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User:            creds.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: cb,
		Timeout:         dialTimeout(ctx),
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	client, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		if mismatch {
			return nil, fmt.Errorf("%w (dial %s)", ErrHostKeyMismatch, addr)
		}
		return nil, fmt.Errorf("ssh: dial %s: %w", addr, err)
	}
	return client, nil
}
