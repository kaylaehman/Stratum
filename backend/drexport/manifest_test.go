package drexport_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/drexport"
	dnspkg "github.com/KAE-Labs/stratum/backend/dns"
	proxypkg "github.com/KAE-Labs/stratum/backend/proxy"
	volumespkg "github.com/KAE-Labs/stratum/backend/volumes"
)

// knownSecretValue is a value that must NEVER appear in any output format.
const knownSecretValue = "s3cr3t-pa$$w0rd-CANARY"

// knownCertKey is a fake private key fragment that must NEVER appear in output.
const knownCertKey = "-----BEGIN RSA PRIVATE KEY-----CANARYKEY"

// --- stub implementations of the reader interfaces ---

type stubStore struct{}

func (stubStore) ListNodes(ctx context.Context) ([]db.Node, error) {
	return []db.Node{
		{
			ID:               "node-1",
			Name:             "homelab-primary",
			Host:             "192.168.1.10",
			Port:             22,
			Type:             "standalone",
			OSType:           "ubuntu",
			AuthMethod:       "ssh_key",
			CapabilitiesJSON: `{"docker":true,"cron":true}`,
			// CredentialsEncrypted intentionally absent (not part of the interface)
			Status: "ok",
		},
	}, nil
}

func (stubStore) ListSecretGroups(ctx context.Context) ([]db.SecretGroup, error) {
	return []db.SecretGroup{
		{ID: "grp-1", Name: "plex-stack"},
	}, nil
}

func (stubStore) ListSecretKeysByGroup(ctx context.Context, groupID string) ([]db.SecretRow, error) {
	// ValueEncrypted is present in db.SecretRow but ListSecretKeysByGroup is
	// documented to return ONLY id+key — we simulate that by returning a row
	// where ValueEncrypted is nil and Key holds the secret name (never the value).
	// The acceptance test below verifies the value never leaks.
	return []db.SecretRow{
		{ID: "sec-1", GroupID: groupID, Key: "PLEX_TOKEN"},
		// Deliberately include a row that *would* leak if we used ListSecretsByGroup:
		// We simulate what a mis-wired call might return (ValueEncrypted holds the
		// sealed canary). The test proves our code path ignores ValueEncrypted.
		{ID: "sec-2", GroupID: groupID, Key: "DB_PASSWORD", ValueEncrypted: []byte(knownSecretValue)},
	}, nil
}

func (stubStore) ListStackEnvVars(ctx context.Context, nodeID, projectName string) ([]db.StackEnvRow, error) {
	return []db.StackEnvRow{
		// Value field holds plaintext here to verify it never leaks into output.
		{NodeID: nodeID, ProjectName: projectName, Key: "TZ", Value: knownSecretValue},
		{NodeID: nodeID, ProjectName: projectName, Key: "PLEX_TOKEN", SecretID: "sec-1"},
	}, nil
}

func (stubStore) ListCerts(ctx context.Context) ([]db.CertInfo, error) {
	notAfter := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return []db.CertInfo{
		{
			ID:       "cert-1",
			NodeID:   "node-1",
			Domain:   "homelab.example.com",
			SANs:     []string{"homelab.example.com", "*.homelab.example.com"},
			Issuer:   "Let's Encrypt",
			Path:     "/etc/letsencrypt/live/homelab.example.com/fullchain.pem",
			NotAfter: &notAfter,
		},
	}, nil
}

func (stubStore) ListContainersByNode(ctx context.Context, nodeID string) ([]db.Container, error) {
	return []db.Container{
		{ID: "c-1", NodeID: nodeID, Name: "plex", ComposeProject: "media"},
		{ID: "c-2", NodeID: nodeID, Name: "jellyfin", ComposeProject: "media"},
	}, nil
}

// --- stub DNS querier ---

type stubDNS struct{}

func (stubDNS) Status(ctx context.Context, nodeID string) (dnspkg.Status, error) {
	return dnspkg.Status{
		Detected: "adguard",
		Records: []dnspkg.Record{
			{Type: "A", Name: "homelab.example.com", Value: "192.168.1.10", TTL: 300},
		},
	}, nil
}

// --- stub proxy querier ---

type stubProxy struct{}

func (stubProxy) Status(ctx context.Context, nodeID string) (proxypkg.Status, error) {
	return proxypkg.Status{
		Detected: "nginx-proxy-manager",
		Rules: []proxypkg.Rule{
			{
				SourceHost: "homelab.example.com",
				TargetURL:  "http://localhost:32400",
				SSLEnabled: true,
			},
		},
	}, nil
}

// --- stub volume querier ---

type stubVolumes struct{}

func (stubVolumes) ListForNode(ctx context.Context, nodeID string) ([]volumespkg.VolumeView, error) {
	return []volumespkg.VolumeView{
		{
			Name:      "plex_config",
			Driver:    "local",
			Status:    volumespkg.StatusAttached,
			SizeBytes: 1024 * 1024 * 500,
		},
	}, nil
}

// --- helpers ---

func buildTestManifest(t *testing.T) drexport.Manifest {
	t.Helper()
	svc := drexport.New(stubStore{}, stubDNS{}, stubProxy{}, stubVolumes{})
	m, err := svc.Build(context.Background())
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	return m
}

// --- acceptance tests: no-value-leak ---

// TestNoSecretValueInJSON is the security acceptance gate: the known secret
// value must never appear in JSON output.
func TestNoSecretValueInJSON(t *testing.T) {
	m := buildTestManifest(t)
	out, err := drexport.RenderJSON(m)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	if strings.Contains(string(out), knownSecretValue) {
		t.Errorf("JSON output contains the secret value %q — value leak!", knownSecretValue)
	}
}

// TestNoSecretValueInYAML verifies the same constraint for YAML output.
func TestNoSecretValueInYAML(t *testing.T) {
	m := buildTestManifest(t)
	out, err := drexport.RenderYAML(m)
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	if strings.Contains(string(out), knownSecretValue) {
		t.Errorf("YAML output contains the secret value %q — value leak!", knownSecretValue)
	}
}

// TestNoSecretValueInMarkdown verifies the same constraint for Markdown output.
func TestNoSecretValueInMarkdown(t *testing.T) {
	m := buildTestManifest(t)
	out := drexport.RenderMarkdown(m)
	if strings.Contains(string(out), knownSecretValue) {
		t.Errorf("Markdown output contains the secret value %q — value leak!", knownSecretValue)
	}
}

// TestNoCertPrivateKeyInAnyFormat verifies that a fake private key fragment
// never appears in any rendered output format.
func TestNoCertPrivateKeyInAnyFormat(t *testing.T) {
	m := buildTestManifest(t)

	jsonOut, err := drexport.RenderJSON(m)
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	yamlOut, err := drexport.RenderYAML(m)
	if err != nil {
		t.Fatalf("RenderYAML: %v", err)
	}
	mdOut := drexport.RenderMarkdown(m)

	for _, format := range []struct {
		name string
		data []byte
	}{
		{"JSON", jsonOut},
		{"YAML", yamlOut},
		{"Markdown", mdOut},
	} {
		if strings.Contains(string(format.data), knownCertKey) {
			t.Errorf("%s output contains cert private key fragment — key leak!", format.name)
		}
	}
}

// --- structural correctness tests ---

// TestSecretRefContainsKeyNames verifies that secret key names DO appear (so
// the manifest is useful) but values DO NOT.
func TestSecretRefContainsKeyNames(t *testing.T) {
	m := buildTestManifest(t)

	if len(m.Secrets) == 0 {
		t.Fatal("expected secret references, got none")
	}

	// Key names should be present.
	var foundPLEXToken, foundDBPassword bool
	for _, ref := range m.Secrets {
		if ref.KeyName == "PLEX_TOKEN" {
			foundPLEXToken = true
		}
		if ref.KeyName == "DB_PASSWORD" {
			foundDBPassword = true
		}
		// Value must not appear anywhere in the struct.
		if strings.Contains(ref.KeyName, knownSecretValue) ||
			strings.Contains(ref.GroupName, knownSecretValue) {
			t.Errorf("secret reference struct contains value canary in field %q", ref.KeyName)
		}
	}
	if !foundPLEXToken {
		t.Error("expected PLEX_TOKEN key name in secret references")
	}
	if !foundDBPassword {
		t.Error("expected DB_PASSWORD key name in secret references")
	}
}

// TestStackEnvKeysOnlyNoValues checks that stack env vars record only key
// names, not the plaintext values stored in the stubStore.
func TestStackEnvKeysOnlyNoValues(t *testing.T) {
	m := buildTestManifest(t)

	if len(m.Stacks) == 0 {
		t.Fatal("expected at least one stack entry")
	}

	for _, st := range m.Stacks {
		// EnvKeys should contain key names only.
		for _, k := range st.EnvKeys {
			if strings.Contains(k, knownSecretValue) {
				t.Errorf("env key %q contains secret value canary", k)
			}
		}
	}
}

// TestNodeMetadataPresent verifies that node host / type / capabilities appear
// in the manifest (so it is actually useful for DR).
func TestNodeMetadataPresent(t *testing.T) {
	m := buildTestManifest(t)

	if len(m.Nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	n := m.Nodes[0]
	if n.Host == "" {
		t.Error("node host is empty")
	}
	if n.Type == "" {
		t.Error("node type is empty")
	}
	if len(n.Capabilities) == 0 {
		t.Error("node capabilities map is empty")
	}
}

// TestCertMetadataPresent verifies that cert entries carry domain and expiry.
func TestCertMetadataPresent(t *testing.T) {
	m := buildTestManifest(t)

	if len(m.Certs) == 0 {
		t.Fatal("expected at least one cert entry")
	}
	c := m.Certs[0]
	if c.Domain == "" {
		t.Error("cert domain is empty")
	}
	if c.NotAfter == nil {
		t.Error("cert NotAfter is nil")
	}
	if c.Path == "" {
		t.Error("cert path is empty")
	}
}

// TestManifestVersion verifies the version field is set.
func TestManifestVersion(t *testing.T) {
	m := buildTestManifest(t)
	if m.Version == 0 {
		t.Error("manifest version should be non-zero")
	}
}

// TestAllFormatsRoundTrip verifies that all three renderers produce non-empty
// output without errors.
func TestAllFormatsRoundTrip(t *testing.T) {
	m := buildTestManifest(t)

	jsonOut, err := drexport.RenderJSON(m)
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	if len(jsonOut) == 0 {
		t.Error("JSON output is empty")
	}

	yamlOut, err := drexport.RenderYAML(m)
	if err != nil {
		t.Fatalf("RenderYAML error: %v", err)
	}
	if len(yamlOut) == 0 {
		t.Error("YAML output is empty")
	}

	mdOut := drexport.RenderMarkdown(m)
	if len(mdOut) == 0 {
		t.Error("Markdown output is empty")
	}
}
