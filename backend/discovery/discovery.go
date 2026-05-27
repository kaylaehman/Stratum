package discovery

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/proxmox"
	"github.com/kaylaehman/stratum/backend/ssh"
)

// probeTimeout bounds each independent sub-probe.
const probeTimeout = 8 * time.Second

// Proxmox auth-status values persisted in capabilities_json (SP1 §5.1).
const (
	PVEStatusConfirmed  = "confirmed"
	PVEStatusUnauthed   = "unauthed"
	PVEStatusMarkerOnly = "marker_only"
	PVEStatusNone       = "none"
)

// Target is everything needed to probe a host. SSH is the baseline; Proxmox and
// Docker fields are optional and only probed when present.
type Target struct {
	Host          string
	SSHPort       int
	SSHCreds      ssh.Credentials
	PinnedHostKey string // knownhosts line; "" => first connect (TOFU)

	ProxmoxEndpoint    string
	ProxmoxTokenID     string
	ProxmoxSecret      string
	ProxmoxTLSInsecure bool

	DockerEndpoint string
	DockerTLS      *docker.TLS
}

// Result is the classified outcome of a probe. Errors are sanitized categories.
type Result struct {
	Type              string // proxmox | standalone | ssh
	OSType            string // debian|ubuntu|rhel|arch|alpine|other
	Caps              capabilities.Set
	ProxmoxAuthStatus string // confirmed|unauthed|marker_only|none
	ReachableSSH      bool
	SSHHostKeySHA256  string
	SSHHostKeyLine    string
	DockerVersion     string
	ProxmoxVersion    string
	PerProbeError     map[string]string // {"ssh":"ssh_auth_failed", ...}
}

type sshProbe struct {
	reachable  bool
	detection  ssh.Detection
	hostKeySHA string
	hostKeyLn  string
	errCat     string
}

type dockerProbe struct {
	reachable bool
	version   string
	errCat    string
}

type pveProbe struct {
	reachable bool
	version   string
	status    string // confirmed | unauthed
	errCat    string
}

// Probe runs the SSH, Docker, and Proxmox sub-probes concurrently (each with its
// own timeout) and classifies the result. Sub-probes are independent: one
// failing never short-circuits the others — all errors are collected.
func Probe(ctx context.Context, t Target) Result {
	var (
		sp sshProbe
		dp dockerProbe
		pp pveProbe
		wg sync.WaitGroup
	)
	wg.Add(3)
	go func() { defer wg.Done(); sp = probeSSH(ctx, t) }()
	go func() { defer wg.Done(); dp = probeDocker(ctx, t) }()
	go func() { defer wg.Done(); pp = probePVE(ctx, t) }()
	wg.Wait()
	return classify(sp, dp, pp)
}

func probeSSH(ctx context.Context, t Target) sshProbe {
	if t.SSHCreds.User == "" {
		return sshProbe{}
	}
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	det, hk, err := ssh.Detect(cctx, t.Host, t.SSHPort, t.SSHCreds, t.PinnedHostKey)
	if err != nil {
		return sshProbe{errCat: SanitizeProbeError(err)}
	}
	return sshProbe{reachable: true, detection: det, hostKeySHA: hk.SHA256, hostKeyLn: hk.KnownHostsLine}
}

func probeDocker(ctx context.Context, t Target) dockerProbe {
	if t.DockerEndpoint == "" {
		return dockerProbe{} // no explicit endpoint; Docker presence inferred from SSH signal
	}
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cli, err := docker.New(t.DockerEndpoint, t.DockerTLS)
	if err != nil {
		return dockerProbe{errCat: SanitizeProbeError(err)}
	}
	defer cli.Close()
	ver, err := cli.Ping(cctx)
	if err != nil {
		return dockerProbe{errCat: SanitizeProbeError(err)}
	}
	return dockerProbe{reachable: true, version: ver}
}

func probePVE(ctx context.Context, t Target) pveProbe {
	if t.ProxmoxEndpoint == "" || t.ProxmoxTokenID == "" {
		return pveProbe{}
	}
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cli := proxmox.New(t.ProxmoxEndpoint, t.ProxmoxTokenID, t.ProxmoxSecret, t.ProxmoxTLSInsecure)
	ver, status, err := cli.Version(cctx)
	if err != nil {
		return pveProbe{errCat: SanitizeProbeError(err)}
	}
	return pveProbe{reachable: true, version: ver, status: string(status)}
}

func classify(sp sshProbe, dp dockerProbe, pp pveProbe) Result {
	r := Result{PerProbeError: map[string]string{}}

	r.OSType = parseOSType(sp.detection.OSReleaseRaw)
	r.ReachableSSH = sp.reachable
	r.SSHHostKeySHA256 = sp.hostKeySHA
	r.SSHHostKeyLine = sp.hostKeyLn
	r.DockerVersion = dp.version
	r.ProxmoxVersion = pp.version

	caps := capabilities.Set{
		Systemd: sp.detection.HasSystemctl && sp.detection.HasRunSystemd,
		Cron:    sp.detection.HasCrontab,
		Docker:  dp.reachable || sp.detection.HasDocker,
	}

	// Proxmox signal + auth status (priority: API confirmed/unauthed > /etc/pve marker).
	authStatus := PVEStatusNone
	switch {
	case pp.reachable && pp.status == string(proxmox.AuthConfirmed):
		authStatus = PVEStatusConfirmed
	case pp.reachable && pp.status == string(proxmox.AuthUnauthed):
		authStatus = PVEStatusUnauthed
	case sp.detection.HasEtcPve:
		authStatus = PVEStatusMarkerOnly
	}
	caps.Proxmox = authStatus != PVEStatusNone
	r.ProxmoxAuthStatus = authStatus
	r.Caps = caps

	if sp.errCat != "" {
		r.PerProbeError["ssh"] = sp.errCat
	}
	if dp.errCat != "" {
		r.PerProbeError["docker"] = dp.errCat
	}
	if pp.errCat != "" {
		r.PerProbeError["proxmox"] = pp.errCat
	}

	// Type classification: Proxmox > standalone (Docker) > ssh.
	switch {
	case caps.Proxmox:
		r.Type = "proxmox"
	case caps.Docker:
		r.Type = "standalone"
	default:
		r.Type = "ssh"
	}
	return r
}

// parseOSType maps /etc/os-release ID / ID_LIKE to the node os_type enum.
func parseOSType(osRelease string) string {
	if strings.TrimSpace(osRelease) == "" {
		return "other"
	}
	fields := map[string]string{}
	for _, line := range strings.Split(osRelease, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		fields[k] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	candidates := strings.Fields(strings.ToLower(fields["ID"] + " " + fields["ID_LIKE"]))
	for _, c := range candidates {
		switch c {
		case "ubuntu":
			return "ubuntu"
		case "debian":
			return "debian"
		case "rhel", "centos", "fedora", "rocky", "almalinux":
			return "rhel"
		case "arch", "archarm":
			return "arch"
		case "alpine":
			return "alpine"
		}
	}
	return "other"
}
