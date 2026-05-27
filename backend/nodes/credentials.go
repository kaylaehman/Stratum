// Package nodes handles node registration: credential sealing, discovery
// orchestration, and CRUD. Credentials are only ever persisted as an
// AES-256-GCM sealed blob via the crypto primitive — never in plaintext.
package nodes

import (
	"encoding/json"
	"fmt"

	"github.com/kaylaehman/stratum/backend/crypto"
)

// Auth method identifiers.
const (
	MethodSSHPassword = "ssh_password"
	MethodSSHKey      = "ssh_key"
)

// NodeCredentials is the secret material needed to reach a node. It is sealed
// (JSON -> crypto.Seal) into nodes.credentials_encrypted and decrypted only
// in-memory when a probe/connection needs it. It is never returned over the API
// or written to logs/activity detail.
type NodeCredentials struct {
	Method         string `json:"method"`
	SSHUser        string `json:"ssh_user,omitempty"`
	SSHPassword    string `json:"ssh_password,omitempty"`
	SSHPrivateKey  string `json:"ssh_private_key,omitempty"`
	SSHPassphrase  string `json:"ssh_passphrase,omitempty"`
	ProxmoxTokenID string `json:"proxmox_token_id,omitempty"`
	ProxmoxSecret  string `json:"proxmox_secret,omitempty"`
	DockerEndpoint string `json:"docker_endpoint,omitempty"`
	DockerTLSCA    string `json:"docker_tls_ca,omitempty"`
	DockerTLSCert  string `json:"docker_tls_cert,omitempty"`
	DockerTLSKey   string `json:"docker_tls_key,omitempty"`
}

// Seal marshals the credentials to JSON and encrypts them into an opaque blob.
func (c NodeCredentials) Seal(cipher *crypto.Cipher) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("nodes: marshal credentials: %w", err)
	}
	return cipher.Seal(b)
}

// OpenCredentials decrypts and unmarshals a sealed credential blob.
func OpenCredentials(cipher *crypto.Cipher, blob []byte) (NodeCredentials, error) {
	pt, err := cipher.Open(blob)
	if err != nil {
		return NodeCredentials{}, fmt.Errorf("nodes: open credentials: %w", err)
	}
	var c NodeCredentials
	if err := json.Unmarshal(pt, &c); err != nil {
		return NodeCredentials{}, fmt.Errorf("nodes: unmarshal credentials: %w", err)
	}
	return c, nil
}
