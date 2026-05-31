package proxy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kaylaehman/stratum/backend/db"
)

func init() { Register(&Cloudflared{}) }

// configSizeLimit caps how many bytes we read from a cloudflared config file.
const configSizeLimit = 1 << 20 // 1 MiB

// Cloudflared reads tunnel ingress rules from the cloudflared config file.
// It is read-only: tunnel ingress is owned by the config file or the Cloudflare
// Zero Trust dashboard, never mutated by Stratum.
//
// Detection covers two deployment styles:
//   - Docker container (image matches cloudflare/cloudflared or cloudflared)
//   - Host systemd/init service (detected by presence of config file at a
//     standard path; no container image required)
//
// Dashboard-managed tunnels (token-based, no local ingress block) signal
// DashboardManaged=true in the returned status so the UI shows a clear
// explanation instead of an empty rules list.
type Cloudflared struct{}

func (c *Cloudflared) Name() string            { return "cloudflared" }
func (c *Cloudflared) ImagePatterns() []string { return []string{"cloudflare/cloudflared", "cloudflared"} }
func (c *Cloudflared) Capabilities() Capabilities {
	return Capabilities{List: true} // read-only; ingress is config/dashboard-owned
}

// ListRules returns the ingress rules found in the cloudflared config file.
// When the config exists but has no ingress block (dashboard-managed tunnel),
// it returns an empty slice and no error.
func (c *Cloudflared) ListRules(ctx context.Context, conn Conn) ([]Rule, error) {
	if conn.ReadFile == nil {
		return nil, fmt.Errorf("cloudflared: host file access not available (no SSH credentials configured)")
	}
	cfgPath, err := locateCloudflaredConfig(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("cloudflared: %w", err)
	}
	return readCloudflaredRules(ctx, conn, cfgPath)
}

// IsDashboardManaged probes the config at cfgPath and reports whether the
// tunnel uses dashboard-managed ingress (config file present but no ingress
// block). Returns false on any read or parse error (conservative).
func (c *Cloudflared) IsDashboardManaged(ctx context.Context, conn Conn) bool {
	if conn.ReadFile == nil {
		return false
	}
	cfgPath, err := locateCloudflaredConfig(ctx, conn)
	if err != nil {
		return false
	}
	rc, err := conn.ReadFile(ctx, cfgPath)
	if err != nil {
		return false
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, configSizeLimit))
	if err != nil {
		return false
	}
	var cfg cloudflaredConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return false
	}
	// A config that parses successfully but has no ingress entries is the
	// hallmark of a token/dashboard-managed tunnel.
	return len(cfg.Ingress) == 0
}

// ProbeHostService checks whether cloudflared is installed as a host service
// (not as a Docker container) by attempting to open its config file at one of
// the standard paths. Returns true when at least one candidate is readable.
// Called by the service layer when no cloudflared container was found.
func (c *Cloudflared) ProbeHostService(ctx context.Context, conn Conn) bool {
	if conn.ReadFile == nil {
		return false
	}
	for _, p := range defaultConfigPaths() {
		rc, err := conn.ReadFile(ctx, p)
		if err == nil {
			rc.Close()
			return true
		}
	}
	return false
}

func (c *Cloudflared) CreateRule(context.Context, Conn, Rule) (Rule, error) {
	return Rule{}, ErrUnsupported
}
func (c *Cloudflared) UpdateRule(context.Context, Conn, string, Rule) error { return ErrUnsupported }
func (c *Cloudflared) DeleteRule(context.Context, Conn, string) error       { return ErrUnsupported }

// locateCloudflaredConfig tries candidate paths in order (mount-derived first,
// then standard defaults) and returns the first openable path.
func locateCloudflaredConfig(ctx context.Context, conn Conn) (string, error) {
	all := append(conn.MountCandidates, defaultConfigPaths()...)
	for _, p := range all {
		rc, err := conn.ReadFile(ctx, p)
		if err == nil {
			rc.Close()
			return p, nil
		}
	}
	return "", fmt.Errorf("config file not found (tried: %s)", strings.Join(all, ", "))
}

// readCloudflaredRules opens, parses, and converts the config file at cfgPath.
func readCloudflaredRules(ctx context.Context, conn Conn, cfgPath string) ([]Rule, error) {
	rc, err := conn.ReadFile(ctx, cfgPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", cfgPath, err)
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, configSizeLimit))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cfgPath, err)
	}
	return parseCloudflaredConfig(raw)
}

// cloudflaredConfig is the minimal subset of the cloudflared config we care about.
type cloudflaredConfig struct {
	Ingress []cloudflaredIngress `yaml:"ingress"`
}

type cloudflaredIngress struct {
	Hostname string `yaml:"hostname"`
	Service  string `yaml:"service"`
	// Path is an optional match predicate; surfaced in SourcePath when set.
	Path string `yaml:"path,omitempty"`
}

// parseCloudflaredConfig parses a cloudflared YAML config and returns one Rule
// per ingress entry that has a hostname. The catch-all (no hostname) is
// skipped. When the config has no ingress block at all the tunnel is treated as
// dashboard-managed: zero rules are returned and a log line is emitted.
func parseCloudflaredConfig(data []byte) ([]Rule, error) {
	var cfg cloudflaredConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Ingress) == 0 {
		slog.Info("cloudflared: no ingress block found — tunnel is likely dashboard-managed; returning zero rules")
		return []Rule{}, nil
	}

	rules := make([]Rule, 0, len(cfg.Ingress))
	for i, entry := range cfg.Ingress {
		if entry.Hostname == "" {
			// Catch-all (last entry with only service:) — skip.
			continue
		}
		rules = append(rules, Rule{
			ID:          fmt.Sprintf("cf-%d", i),
			AdapterType: "cloudflared",
			SourceHost:  entry.Hostname,
			SourcePath:  entry.Path,
			TargetURL:   entry.Service,
			// cloudflared always terminates TLS at the edge — the public
			// hostname is served over HTTPS via the Cloudflare network.
			SSLEnabled: true,
		})
	}
	return rules, nil
}

// defaultConfigPaths returns the standard locations cloudflared uses for its
// config file, in preference order. Covers:
//   - System-wide: /etc/cloudflared/ (most common for service installs)
//   - Root user home: /root/.cloudflared/ (cloudflared service install default)
//   - Non-root user home: ~/.cloudflared/ resolved as /home/<user>/.cloudflared/
//     is not enumerable over SSH, but /usr/local/etc/cloudflared/ covers
//     BSD-style and some manual installs.
func defaultConfigPaths() []string {
	return []string{
		"/etc/cloudflared/config.yml",
		"/etc/cloudflared/config.yaml",
		"/root/.cloudflared/config.yml",
		"/root/.cloudflared/config.yaml",
		"/usr/local/etc/cloudflared/config.yml",
		"/usr/local/etc/cloudflared/config.yaml",
		"/home/cloudflared/.cloudflared/config.yml",
		"/home/cloudflared/.cloudflared/config.yaml",
	}
}

// mountBasedCandidates examines a list of bind mounts and returns candidate
// config file paths derived from any mount that targets a known cloudflared
// config directory. Used by the service layer to populate Conn.MountCandidates.
func mountBasedCandidates(mounts []db.MountRow) []string {
	const cfgDir = "/etc/cloudflared"
	var out []string
	for _, m := range mounts {
		dest := path.Clean(m.Destination)
		// Mount targets the config dir itself: source/config.yml etc.
		if dest == cfgDir {
			src := strings.TrimRight(m.Source, "/")
			out = append(out,
				src+"/config.yml",
				src+"/config.yaml",
			)
			continue
		}
		// Mount targets one of the config files directly.
		if dest == cfgDir+"/config.yml" || dest == cfgDir+"/config.yaml" {
			out = append(out, m.Source)
		}
	}
	return out
}
