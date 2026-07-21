package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/crypto"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/discovery"
	"github.com/KAE-Labs/stratum/backend/docker"
)

// Errors surfaced by the service.
var (
	ErrHostKeyRequired = errors.New("nodes: accepted SSH host key is required to create a node")
	ErrHostKeyMismatch = errors.New("nodes: presented host key does not match the accepted key")
)

// Service is the node-registration use case layer.
type Service struct {
	store  db.Store
	cipher *crypto.Cipher
}

// NewService wires the node store and the credential cipher.
func NewService(store db.Store, cipher *crypto.Cipher) *Service {
	return &Service{store: store, cipher: cipher}
}

// ConnInput is the non-secret connection info plus credentials for a probe.
type ConnInput struct {
	Host               string
	SSHPort            int
	Credentials        NodeCredentials
	ProxmoxEndpoint    string
	ProxmoxTLSInsecure bool
	DockerEndpoint     string
	PinnedHostKey      string // knownhosts line to verify against; "" = first connect (TOFU)
}

// CreateInput adds the fields needed to persist a node.
type CreateInput struct {
	ConnInput
	Name            string
	AcceptedHostKey string // operator-confirmed knownhosts line (required when SSH is used)
	TypeOverride    string // optional: proxmox|standalone|ssh
}

// PreviewResult is the non-secret discovery outcome shown in the wizard.
type PreviewResult struct {
	Type              string            `json:"type"`
	OSType            string            `json:"os_type"`
	Capabilities      capabilities.Set  `json:"capabilities"`
	ProxmoxAuthStatus string            `json:"proxmox_auth_status"`
	ReachableSSH      bool              `json:"reachable_ssh"`
	SSHHostKeySHA256  string            `json:"ssh_host_key_sha256"`
	SSHHostKeyLine    string            `json:"ssh_host_key_line"`
	DockerVersion     string            `json:"docker_version,omitempty"`
	ProxmoxVersion    string            `json:"proxmox_version,omitempty"`
	ProbeErrors       map[string]string `json:"probe_errors,omitempty"`
	ProbeHints        map[string]string `json:"probe_hints,omitempty"`
}

// NodeView is the API representation of a node — never carries secrets.
type NodeView struct {
	ID                string           `json:"id"`
	Name              string           `json:"name"`
	Type              string           `json:"type"`
	Host              string           `json:"host"`
	Port              int              `json:"port"`
	AuthMethod        string           `json:"auth_method"`
	OSType            string           `json:"os_type,omitempty"`
	Capabilities      capabilities.Set `json:"capabilities"`
	ProxmoxEndpoint   string           `json:"proxmox_endpoint,omitempty"`
	DockerEndpoint    string           `json:"docker_endpoint,omitempty"`
	// LinkedVMID is the manual Proxmox-guest correlation (tri-state): null = AUTO
	// (frontend matches by name), 0 = NONE (force-unlinked), >=100 = explicit VMID.
	LinkedVMID        *int             `json:"linked_vmid,omitempty"`
	ProxmoxAuthStatus string           `json:"proxmox_auth_status"`
	Status            string           `json:"status"`
	LastError         string           `json:"last_error,omitempty"`
	LastSeen          *time.Time       `json:"last_seen,omitempty"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

// capsEnvelope is capabilities_json on disk: the Set plus proxmox_auth_status.
type capsEnvelope struct {
	capabilities.Set
	ProxmoxAuthStatus string `json:"proxmox_auth_status"`
}

func (in ConnInput) target() discovery.Target {
	c := in.Credentials
	t := discovery.Target{
		Host:               in.Host,
		SSHPort:            in.SSHPort,
		PinnedHostKey:      in.PinnedHostKey,
		ProxmoxEndpoint:    in.ProxmoxEndpoint,
		ProxmoxTokenID:     c.ProxmoxTokenID,
		ProxmoxSecret:      c.ProxmoxSecret,
		ProxmoxTLSInsecure: in.ProxmoxTLSInsecure,
		DockerEndpoint:     in.DockerEndpoint,
	}
	t.SSHCreds.User = c.SSHUser
	t.SSHCreds.Password = c.SSHPassword
	t.SSHCreds.PrivateKeyPEM = c.SSHPrivateKey
	t.SSHCreds.Passphrase = c.SSHPassphrase
	if c.DockerTLSCA != "" || c.DockerTLSCert != "" || c.DockerTLSKey != "" {
		t.DockerTLS = &docker.TLS{CA: c.DockerTLSCA, Cert: c.DockerTLSCert, Key: c.DockerTLSKey}
	}
	return t
}

func toPreview(r discovery.Result) PreviewResult {
	return PreviewResult{
		Type:              r.Type,
		OSType:            r.OSType,
		Capabilities:      r.Caps,
		ProxmoxAuthStatus: r.ProxmoxAuthStatus,
		ReachableSSH:      r.ReachableSSH,
		SSHHostKeySHA256:  r.SSHHostKeySHA256,
		SSHHostKeyLine:    r.SSHHostKeyLine,
		DockerVersion:     r.DockerVersion,
		ProxmoxVersion:    r.ProxmoxVersion,
		ProbeErrors:       r.PerProbeError,
		ProbeHints:        r.PerProbeHint,
	}
}

// ProbePreview runs discovery WITHOUT persisting. Powers the add-node wizard.
func (s *Service) ProbePreview(ctx context.Context, in ConnInput) PreviewResult {
	return toPreview(discovery.Probe(ctx, in.target()))
}

// Create probes the host (verifying the operator-accepted host key), seals the
// credentials, and persists the node. A node is never stored without an
// accepted host key when SSH credentials are supplied.
func (s *Service) Create(ctx context.Context, in CreateInput) (NodeView, error) {
	usesSSH := in.Credentials.SSHUser != ""
	if usesSSH && in.AcceptedHostKey == "" {
		return NodeView{}, ErrHostKeyRequired
	}

	// Verify against the accepted key (hard-fail on mismatch = MITM protection).
	in.PinnedHostKey = in.AcceptedHostKey
	res := discovery.Probe(ctx, in.ConnInput.target())
	if res.SSHHostKeyMismatch {
		return NodeView{}, ErrHostKeyMismatch
	}

	sealed, err := in.Credentials.Seal(s.cipher)
	if err != nil {
		return NodeView{}, err
	}

	nodeType := res.Type
	if in.TypeOverride != "" {
		nodeType = in.TypeOverride
	}
	status, lastErr, lastSeen := deriveStatus(res)

	capsJSON, err := json.Marshal(capsEnvelope{Set: res.Caps, ProxmoxAuthStatus: res.ProxmoxAuthStatus})
	if err != nil {
		return NodeView{}, err
	}

	n := db.Node{
		ID:                   uuid.NewString(),
		Name:                 in.Name,
		Type:                 nodeType,
		Host:                 in.Host,
		Port:                 in.SSHPort,
		AuthMethod:           in.Credentials.Method,
		OSType:               res.OSType,
		CapabilitiesJSON:     string(capsJSON),
		CredentialsEncrypted: sealed,
		CredentialsVersion:   1,
		SSHHostKey:           in.AcceptedHostKey,
		ProxmoxEndpoint:      in.ProxmoxEndpoint,
		ProxmoxTLSInsecure:   in.ProxmoxTLSInsecure,
		DockerEndpoint:       in.DockerEndpoint,
		Status:               status,
		LastError:            lastErr,
		LastSeen:             lastSeen,
	}
	if err := s.store.CreateNode(ctx, n); err != nil {
		return NodeView{}, err
	}
	return toView(n), nil
}

// UpdateConfigInput carries the editable connection config for an existing node.
// Only the Docker/Proxmox transport config is editable here — SSH credentials and
// the pinned host key are not changed by this path. Pointer fields distinguish
// "not supplied" (leave as-is) from "supplied empty" (clear). Docker TLS PEM
// material, when any of the three is non-empty, replaces the stored set; when all
// three are empty AND DockerTLSSupplied is true, the stored TLS material is cleared.
type UpdateConfigInput struct {
	Name               *string
	DockerEndpoint     *string
	ProxmoxEndpoint    *string
	ProxmoxTLSInsecure *bool

	// LinkedVMID tri-state edit. When LinkedVMIDSupplied is true, the stored
	// linked_vmid is replaced with LinkedVMID (nil = AUTO/clear, *0 = NONE,
	// *>=100 = explicit VMID). When false, the stored value is left as-is.
	LinkedVMIDSupplied bool
	LinkedVMID         *int

	DockerTLSSupplied bool
	DockerTLSCA       string
	DockerTLSCert     string
	DockerTLSKey      string
}

// UpdateConfig edits an existing node's name and Docker/Proxmox transport config,
// re-seals credentials when Docker TLS material is supplied, re-probes with the
// new config, and persists the refreshed status. It returns the updated view.
//
// It does NOT change SSH credentials or the pinned host key; those stay as stored.
func (s *Service) UpdateConfig(ctx context.Context, id string, in UpdateConfigInput) (NodeView, error) {
	n, err := s.store.GetNode(ctx, id)
	if err != nil {
		return NodeView{}, err
	}
	creds, err := OpenCredentials(s.cipher, n.CredentialsEncrypted)
	if err != nil {
		return NodeView{}, err
	}

	if in.Name != nil {
		n.Name = *in.Name
	}
	if in.DockerEndpoint != nil {
		n.DockerEndpoint = *in.DockerEndpoint
	}
	if in.ProxmoxEndpoint != nil {
		n.ProxmoxEndpoint = *in.ProxmoxEndpoint
	}
	if in.ProxmoxTLSInsecure != nil {
		n.ProxmoxTLSInsecure = *in.ProxmoxTLSInsecure
	}
	if in.LinkedVMIDSupplied {
		n.LinkedVMID = in.LinkedVMID
	}

	credsChanged := false
	if in.DockerTLSSupplied {
		creds.DockerTLSCA = in.DockerTLSCA
		creds.DockerTLSCert = in.DockerTLSCert
		creds.DockerTLSKey = in.DockerTLSKey
		credsChanged = true
	}
	if credsChanged {
		sealed, serr := creds.Seal(s.cipher)
		if serr != nil {
			return NodeView{}, serr
		}
		n.CredentialsEncrypted = sealed
	}

	// Re-probe with the new transport config (pinned to the stored host key) so
	// status/last_error reflect reality immediately after the edit.
	probeIn := ConnInput{
		Host:               n.Host,
		SSHPort:            n.Port,
		Credentials:        creds,
		ProxmoxEndpoint:    n.ProxmoxEndpoint,
		ProxmoxTLSInsecure: n.ProxmoxTLSInsecure,
		DockerEndpoint:     n.DockerEndpoint,
		PinnedHostKey:      n.SSHHostKey,
	}
	res := discovery.Probe(ctx, probeIn.target())
	if res.SSHHostKeyMismatch {
		n.Status = "error"
		n.LastError = discovery.ErrCategorySSHHostKey
		_ = s.store.UpdateNode(ctx, n)
		return NodeView{}, ErrHostKeyMismatch
	}

	capsJSON, err := json.Marshal(capsEnvelope{Set: res.Caps, ProxmoxAuthStatus: res.ProxmoxAuthStatus})
	if err != nil {
		return NodeView{}, err
	}
	n.CapabilitiesJSON = string(capsJSON)
	n.OSType = res.OSType
	n.Status, n.LastError, n.LastSeen = deriveStatus(res)

	if err := s.store.UpdateNode(ctx, n); err != nil {
		return NodeView{}, err
	}
	return toView(n), nil
}

// Reprobe re-runs detection using the stored credentials and updates the node.
func (s *Service) Reprobe(ctx context.Context, id string) (NodeView, error) {
	n, err := s.store.GetNode(ctx, id)
	if err != nil {
		return NodeView{}, err
	}
	creds, err := OpenCredentials(s.cipher, n.CredentialsEncrypted)
	if err != nil {
		return NodeView{}, err
	}
	in := ConnInput{
		Host:               n.Host,
		SSHPort:            n.Port,
		Credentials:        creds,
		ProxmoxEndpoint:    n.ProxmoxEndpoint,
		ProxmoxTLSInsecure: n.ProxmoxTLSInsecure,
		DockerEndpoint:     n.DockerEndpoint,
		PinnedHostKey:      n.SSHHostKey,
	}
	res := discovery.Probe(ctx, in.target())
	if res.SSHHostKeyMismatch {
		// Persist the mismatch as an error state rather than silently trusting.
		n.Status = "error"
		n.LastError = discovery.ErrCategorySSHHostKey
		_ = s.store.UpdateNode(ctx, n)
		return NodeView{}, ErrHostKeyMismatch
	}

	capsJSON, err := json.Marshal(capsEnvelope{Set: res.Caps, ProxmoxAuthStatus: res.ProxmoxAuthStatus})
	if err != nil {
		return NodeView{}, err
	}
	n.CapabilitiesJSON = string(capsJSON)
	n.OSType = res.OSType
	n.Status, n.LastError, n.LastSeen = deriveStatus(res)
	if err := s.store.UpdateNode(ctx, n); err != nil {
		return NodeView{}, err
	}
	return toView(n), nil
}

// ProbeReachability runs the same probe as Reprobe (SSH + Docker + Proxmox,
// using the pinned host key) but does NOT persist — it returns just whether the
// node is reachable and, if not, a sanitized error category. The inventory
// poller uses this so SSH-only nodes (and nodes whose Docker/Proxmox transport
// is down but SSH is up) are correctly marked reachable instead of always
// "unreachable" with an empty last_error.
func (s *Service) ProbeReachability(ctx context.Context, n db.Node) (status, lastErr string) {
	creds, err := OpenCredentials(s.cipher, n.CredentialsEncrypted)
	if err != nil {
		return "", ""
	}
	in := ConnInput{
		Host:               n.Host,
		SSHPort:            n.Port,
		Credentials:        creds,
		ProxmoxEndpoint:    n.ProxmoxEndpoint,
		ProxmoxTLSInsecure: n.ProxmoxTLSInsecure,
		DockerEndpoint:     n.DockerEndpoint,
		PinnedHostKey:      n.SSHHostKey,
	}
	st, le, _ := deriveStatus(discovery.Probe(ctx, in.target()))
	return st, le
}

// List returns all nodes (no secrets).
func (s *Service) List(ctx context.Context) ([]NodeView, error) {
	nodes, err := s.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]NodeView, 0, len(nodes))
	for _, n := range nodes {
		views = append(views, toView(n))
	}
	return views, nil
}

// Get returns one node (no secrets).
func (s *Service) Get(ctx context.Context, id string) (NodeView, error) {
	n, err := s.store.GetNode(ctx, id)
	if err != nil {
		return NodeView{}, err
	}
	return toView(n), nil
}

// Rename updates the node's display name.
func (s *Service) Rename(ctx context.Context, id, name string) (NodeView, error) {
	n, err := s.store.GetNode(ctx, id)
	if err != nil {
		return NodeView{}, err
	}
	n.Name = name
	if err := s.store.UpdateNode(ctx, n); err != nil {
		return NodeView{}, err
	}
	return toView(n), nil
}

// Delete removes a node.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.DeleteNode(ctx, id)
}

func deriveStatus(r discovery.Result) (status, lastErr string, lastSeen *time.Time) {
	apiOK := r.DockerVersion != "" || r.ProxmoxAuthStatus == discovery.PVEStatusConfirmed
	sshErr := r.PerProbeError["ssh"]
	// Reachable if any probe succeeded.
	if r.ReachableSSH || apiOK {
		now := time.Now()
		// Degraded: a management API (Docker/Proxmox) is reachable, but SSH was
		// attempted and failed. Host-level ops (terminal, host FS browse, scripts)
		// need SSH, so surface "degraded" + the SSH error instead of a flat "ok"
		// that hides the outage — or a misleading "unreachable" while VMs/containers
		// still enumerate.
		if apiOK && !r.ReachableSSH && sshErr != "" {
			return "degraded", sshErr, &now
		}
		return "ok", "", &now
	}
	// Pick a representative sanitized error if present.
	for _, key := range []string{"ssh", "docker", "proxmox"} {
		if c := r.PerProbeError[key]; c != "" {
			return "unreachable", c, nil
		}
	}
	return "unknown", "", nil
}

func toView(n db.Node) NodeView {
	var env capsEnvelope
	if n.CapabilitiesJSON != "" {
		_ = json.Unmarshal([]byte(n.CapabilitiesJSON), &env)
	}
	if env.ProxmoxAuthStatus == "" {
		env.ProxmoxAuthStatus = discovery.PVEStatusNone
	}
	return NodeView{
		ID:                n.ID,
		Name:              n.Name,
		Type:              n.Type,
		Host:              n.Host,
		Port:              n.Port,
		AuthMethod:        n.AuthMethod,
		OSType:            n.OSType,
		Capabilities:      env.Set,
		ProxmoxEndpoint:   n.ProxmoxEndpoint,
		DockerEndpoint:    n.DockerEndpoint,
		LinkedVMID:        n.LinkedVMID,
		ProxmoxAuthStatus: env.ProxmoxAuthStatus,
		Status:            n.Status,
		LastError:         n.LastError,
		LastSeen:          n.LastSeen,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
	}
}
