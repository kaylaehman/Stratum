package drexport

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	dnspkg "github.com/KAE-Labs/stratum/backend/dns"
	proxypkg "github.com/KAE-Labs/stratum/backend/proxy"
	volumespkg "github.com/KAE-Labs/stratum/backend/volumes"
)

// DNSQuerier provides per-node DNS status (records). *dns.Service satisfies this.
type DNSQuerier interface {
	Status(ctx context.Context, nodeID string) (dnspkg.Status, error)
}

// ProxyQuerier provides per-node proxy status (rules). *proxy.Service satisfies this.
type ProxyQuerier interface {
	Status(ctx context.Context, nodeID string) (proxypkg.Status, error)
}

// VolumeQuerier provides per-node volume listing. *volumes.Service satisfies this.
type VolumeQuerier interface {
	ListForNode(ctx context.Context, nodeID string) ([]volumespkg.VolumeView, error)
}

// Service builds disaster-recovery manifests. All inputs are read-only.
type Service struct {
	store   StoreReader
	dns     DNSQuerier   // may be nil
	proxy   ProxyQuerier // may be nil
	volumes VolumeQuerier // may be nil
}

// New constructs the service. dns, proxy, and volumes may be nil; those
// sections will be empty in the manifest.
func New(store StoreReader, dns DNSQuerier, proxy ProxyQuerier, vols VolumeQuerier) *Service {
	return &Service{store: store, dns: dns, proxy: proxy, volumes: vols}
}

// Build assembles and returns a Manifest from the current state of all stores.
// It is read-only: no side effects, no mutations.
func (s *Service) Build(ctx context.Context) (Manifest, error) {
	m := Manifest{
		GeneratedAt: time.Now().UTC(),
		Version:     1,
	}

	nodes, nameByID, err := s.buildNodes(ctx)
	if err != nil {
		return Manifest{}, fmt.Errorf("drexport: nodes: %w", err)
	}
	m.Nodes = nodes

	m.Secrets, err = s.buildSecretRefs(ctx)
	if err != nil {
		return Manifest{}, fmt.Errorf("drexport: secrets: %w", err)
	}

	m.Certs, err = s.buildCerts(ctx, nameByID)
	if err != nil {
		return Manifest{}, fmt.Errorf("drexport: certs: %w", err)
	}

	// Best-effort: DNS / proxy / volumes tolerate individual node failures.
	m.DNS = s.buildDNS(ctx, m.Nodes)
	m.ProxyRoutes = s.buildProxy(ctx, m.Nodes)
	m.Volumes = s.buildVolumes(ctx, m.Nodes)
	m.Stacks = s.buildStacks(ctx, m.Nodes, nameByID)

	return m, nil
}

// buildNodes converts raw db.Node rows into NodeEntry values. It returns the
// slice and a nodeID→name lookup used by other sections.
func (s *Service) buildNodes(ctx context.Context) ([]NodeEntry, map[string]string, error) {
	rows, err := s.store.ListNodes(ctx)
	if err != nil {
		return nil, nil, err
	}
	entries := make([]NodeEntry, 0, len(rows))
	nameByID := make(map[string]string, len(rows))
	for _, n := range rows {
		nameByID[n.ID] = n.Name
		caps := parseCapabilities(n.CapabilitiesJSON)
		entries = append(entries, NodeEntry{
			ID:           n.ID,
			Name:         n.Name,
			Host:         n.Host,
			Port:         n.Port,
			Type:         n.Type,
			OSType:       n.OSType,
			AuthMethod:   n.AuthMethod,
			Capabilities: caps,
			Status:       n.Status,
		})
	}
	return entries, nameByID, nil
}

// buildSecretRefs lists all groups + their key names. Values are NEVER fetched.
func (s *Service) buildSecretRefs(ctx context.Context) ([]SecretRef, error) {
	groups, err := s.store.ListSecretGroups(ctx)
	if err != nil {
		return nil, err
	}
	var refs []SecretRef
	for _, g := range groups {
		// ListSecretKeysByGroup returns only id+key; the encrypted blob is absent.
		rows, err := s.store.ListSecretKeysByGroup(ctx, g.ID)
		if err != nil {
			continue // best-effort per group
		}
		for _, r := range rows {
			refs = append(refs, SecretRef{
				GroupID:   g.ID,
				GroupName: g.Name,
				KeyName:   r.Key,
			})
		}
	}
	return refs, nil
}

// buildCerts projects db.CertInfo rows to CertEntry. Private key material is
// structurally absent from db.CertInfo (the scanner only parses the leaf cert,
// never the key).
func (s *Service) buildCerts(ctx context.Context, nameByID map[string]string) ([]CertEntry, error) {
	rows, err := s.store.ListCerts(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]CertEntry, 0, len(rows))
	for _, c := range rows {
		entries = append(entries, CertEntry{
			NodeID:   c.NodeID,
			NodeName: nameByID[c.NodeID],
			Domain:   c.Domain,
			SANs:     c.SANs,
			Issuer:   c.Issuer,
			Path:     c.Path,
			NotAfter: c.NotAfter,
		})
	}
	return entries, nil
}

// buildDNS queries the DNS status for each node and aggregates records.
func (s *Service) buildDNS(ctx context.Context, nodes []NodeEntry) []DNSEntry {
	if s.dns == nil {
		return nil
	}
	var entries []DNSEntry
	for _, n := range nodes {
		st, err := s.dns.Status(ctx, n.ID)
		if err != nil || st.Detected == "" {
			continue
		}
		for _, rec := range st.Records {
			entries = append(entries, DNSEntry{
				NodeID:      n.ID,
				NodeName:    n.Name,
				AdapterType: st.Detected,
				RecordType:  rec.Type,
				Name:        rec.Name,
				Value:       rec.Value,
				TTL:         rec.TTL,
			})
		}
	}
	return entries
}

// buildProxy queries the proxy status for each node and aggregates routes.
func (s *Service) buildProxy(ctx context.Context, nodes []NodeEntry) []ProxyEntry {
	if s.proxy == nil {
		return nil
	}
	var entries []ProxyEntry
	for _, n := range nodes {
		st, err := s.proxy.Status(ctx, n.ID)
		if err != nil || st.Detected == "" {
			continue
		}
		for _, rule := range st.Rules {
			entries = append(entries, ProxyEntry{
				NodeID:      n.ID,
				NodeName:    n.Name,
				AdapterType: st.Detected,
				SourceHost:  rule.SourceHost,
				SourcePath:  rule.SourcePath,
				TargetURL:   rule.TargetURL,
				SSLEnabled:  rule.SSLEnabled,
			})
		}
	}
	return entries
}

// buildVolumes queries each node for its volumes.
func (s *Service) buildVolumes(ctx context.Context, nodes []NodeEntry) []VolumeEntry {
	if s.volumes == nil {
		return nil
	}
	var entries []VolumeEntry
	for _, n := range nodes {
		vols, err := s.volumes.ListForNode(ctx, n.ID)
		if err != nil {
			continue
		}
		for _, v := range vols {
			entries = append(entries, VolumeEntry{
				NodeID:     n.ID,
				NodeName:   n.Name,
				Name:       v.Name,
				Driver:     v.Driver,
				Mountpoint: v.Mountpoint,
				Status:     v.Status,
				SizeBytes:  v.SizeBytes,
			})
		}
	}
	return entries
}

// buildStacks reads containers to discover compose projects per node, then
// fetches the env-key names (never values) for each project.
func (s *Service) buildStacks(ctx context.Context, nodes []NodeEntry, nameByID map[string]string) []StackEntry {
	var entries []StackEntry
	for _, n := range nodes {
		containers, err := s.store.ListContainersByNode(ctx, n.ID)
		if err != nil {
			continue
		}
		// Collect unique project names.
		projects := map[string]struct{}{}
		for _, c := range containers {
			if c.ComposeProject != "" {
				projects[c.ComposeProject] = struct{}{}
			}
		}
		// Sort for deterministic output.
		sortedProjects := make([]string, 0, len(projects))
		for p := range projects {
			sortedProjects = append(sortedProjects, p)
		}
		sort.Strings(sortedProjects)

		for _, project := range sortedProjects {
			envRows, err := s.store.ListStackEnvVars(ctx, n.ID, project)
			var keys []string
			if err == nil {
				for _, r := range envRows {
					keys = append(keys, r.Key)
				}
				sort.Strings(keys)
			}
			entries = append(entries, StackEntry{
				NodeID:      n.ID,
				NodeName:    nameByID[n.ID],
				Project:     project,
				ComposePath: "", // path discovery requires live SSH; omit in static export
				EnvKeys:     keys,
			})
		}
	}
	return entries
}

// --- format helpers ---

// RenderJSON serialises a manifest to indented JSON.
func RenderJSON(m Manifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// RenderYAML serialises a manifest to YAML.
func RenderYAML(m Manifest) ([]byte, error) {
	return yaml.Marshal(m)
}

// RenderMarkdown produces a human-readable rebuild guide from the manifest.
func RenderMarkdown(m Manifest) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# Stratum Disaster-Recovery Manifest\n\n")
	fmt.Fprintf(&b, "Generated: %s\n\n", m.GeneratedAt.Format(time.RFC3339))

	// Nodes
	fmt.Fprintf(&b, "## Nodes (%d)\n\n", len(m.Nodes))
	for _, n := range m.Nodes {
		fmt.Fprintf(&b, "### %s\n", n.Name)
		fmt.Fprintf(&b, "- **ID:** `%s`\n", n.ID)
		fmt.Fprintf(&b, "- **Host:** `%s`\n", n.Host)
		if n.Port != 0 {
			fmt.Fprintf(&b, "- **SSH Port:** %d\n", n.Port)
		}
		fmt.Fprintf(&b, "- **Type:** %s\n", n.Type)
		fmt.Fprintf(&b, "- **OS:** %s\n", n.OSType)
		fmt.Fprintf(&b, "- **Auth:** %s\n", n.AuthMethod)
		fmt.Fprintf(&b, "- **Status:** %s\n\n", n.Status)
	}

	// Stacks
	if len(m.Stacks) > 0 {
		fmt.Fprintf(&b, "## Compose Stacks (%d)\n\n", len(m.Stacks))
		for _, st := range m.Stacks {
			fmt.Fprintf(&b, "### %s / %s\n", st.NodeName, st.Project)
			if st.ComposePath != "" {
				fmt.Fprintf(&b, "- **Compose file:** `%s`\n", st.ComposePath)
			}
			if len(st.EnvKeys) > 0 {
				fmt.Fprintf(&b, "- **Env keys (values not stored):** %s\n", strings.Join(st.EnvKeys, ", "))
			}
			fmt.Fprintln(&b)
		}
	}

	// Volumes
	if len(m.Volumes) > 0 {
		fmt.Fprintf(&b, "## Volumes (%d)\n\n", len(m.Volumes))
		for _, v := range m.Volumes {
			fmt.Fprintf(&b, "- `%s` on **%s** — driver: %s, status: %s, size: %d bytes\n",
				v.Name, v.NodeName, v.Driver, v.Status, v.SizeBytes)
		}
		fmt.Fprintln(&b)
	}

	// DNS
	if len(m.DNS) > 0 {
		fmt.Fprintf(&b, "## DNS Records (%d)\n\n", len(m.DNS))
		for _, d := range m.DNS {
			fmt.Fprintf(&b, "- [%s] `%s` → `%s` (TTL %d) via %s on **%s**\n",
				d.RecordType, d.Name, d.Value, d.TTL, d.AdapterType, d.NodeName)
		}
		fmt.Fprintln(&b)
	}

	// Proxy routes
	if len(m.ProxyRoutes) > 0 {
		fmt.Fprintf(&b, "## Reverse-Proxy Routes (%d)\n\n", len(m.ProxyRoutes))
		for _, p := range m.ProxyRoutes {
			ssl := ""
			if p.SSLEnabled {
				ssl = " [SSL]"
			}
			fmt.Fprintf(&b, "- `%s%s` → `%s`%s via %s on **%s**\n",
				p.SourceHost, p.SourcePath, p.TargetURL, ssl, p.AdapterType, p.NodeName)
		}
		fmt.Fprintln(&b)
	}

	// Certs
	if len(m.Certs) > 0 {
		fmt.Fprintf(&b, "## Certificates (%d)\n\n", len(m.Certs))
		for _, c := range m.Certs {
			expiry := "unknown"
			if c.NotAfter != nil {
				expiry = c.NotAfter.Format("2006-01-02")
			}
			fmt.Fprintf(&b, "- `%s` (issuer: %s, expires: %s, path: `%s`) on **%s**\n",
				c.Domain, c.Issuer, expiry, c.Path, c.NodeName)
		}
		fmt.Fprintln(&b)
	}

	// Secret references
	if len(m.Secrets) > 0 {
		fmt.Fprintf(&b, "## Secret References (%d)\n\n", len(m.Secrets))
		fmt.Fprintf(&b, "> Values are **not** exported. Re-enter each secret after restore.\n\n")
		currentGroup := ""
		for _, sr := range m.Secrets {
			if sr.GroupName != currentGroup {
				fmt.Fprintf(&b, "### Group: %s\n", sr.GroupName)
				currentGroup = sr.GroupName
			}
			fmt.Fprintf(&b, "- `%s`\n", sr.KeyName)
		}
	}

	return []byte(b.String())
}

// parseCapabilities unmarshals capabilities_json into a generic map. Returns
// an empty map on any error so the node entry is still included.
func parseCapabilities(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]any{}
	}
	return m
}
