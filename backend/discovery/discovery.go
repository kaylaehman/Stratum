package discovery

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/KAE-Labs/stratum/backend/capabilities"
	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/proxmox"
	"github.com/KAE-Labs/stratum/backend/ssh"
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
	ReachableSSH       bool
	SSHHostKeyMismatch bool // presented SSH key did not match the pinned key
	SSHHostKeySHA256   string
	SSHHostKeyLine     string
	DockerVersion      string
	ProxmoxVersion     string
	PerProbeError      map[string]string // {"ssh":"ssh_auth_pubkey_rejected", ...}
	// PerProbeHint is a parallel map: same keys as PerProbeError, each value a
	// short, host-free, user-actionable English sentence. Surfaced in the UI
	// so the user knows what to change next.
	PerProbeHint       map[string]string
}

type sshProbe struct {
	reachable   bool
	detection   ssh.Detection
	hostKeySHA  string
	hostKeyLn   string
	keyMismatch bool
	errCat      string
	errHint     string
}

type dockerProbe struct {
	reachable bool
	version   string
	errCat    string
	errHint   string
}

type pveProbe struct {
	reachable bool
	version   string
	status    string // confirmed | unauthed
	errCat    string
	errHint   string
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

// errTargetIsSelf is produced (and then sanitized) when the probe target
// resolves to the Stratum host's own address. We never dial in that case:
// connecting to ourselves can't yield a registerable remote node, and when
// nothing listens on the local SSH port it only produces a misleading
// "unreachable". The message text drives SanitizeProbeError's categorization.
var errTargetIsSelf = errors.New("ssh: probe target resolves to the stratum host itself")

func probeSSH(ctx context.Context, t Target) sshProbe {
	if t.SSHCreds.User == "" {
		return sshProbe{}
	}
	if isSelfTarget(ctx, t.Host) {
		cat, hint := SanitizeProbeError(errTargetIsSelf)
		return sshProbe{errCat: cat, errHint: hint}
	}
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	det, hk, err := ssh.Detect(cctx, t.Host, t.SSHPort, t.SSHCreds, t.PinnedHostKey)
	if err != nil {
		cat, hint := SanitizeProbeError(err)
		// Surface whatever host-key info we captured before the failure: when
		// auth fails AFTER the host-key callback fired, Detect still returns
		// the captured key so the wizard can show the fingerprint and the
		// user can verify (and re-probe) with corrected credentials.
		return sshProbe{
			keyMismatch: errors.Is(err, ssh.ErrHostKeyMismatch),
			hostKeySHA:  hk.SHA256,
			hostKeyLn:   hk.KnownHostsLine,
			errCat:      cat,
			errHint:     hint,
		}
	}
	return sshProbe{reachable: true, detection: det, hostKeySHA: hk.SHA256, hostKeyLn: hk.KnownHostsLine}
}

// isSelfTarget reports whether host refers to the machine Stratum runs on:
// a localhost literal, or a name/IP that resolves to a loopback address or to
// one of this host's own interface addresses. Resolution failures return false
// — we let the real dial surface the actual network error rather than guessing.
func isSelfTarget(ctx context.Context, host string) bool {
	h := strings.Trim(strings.TrimSpace(host), "[]") // tolerate IPv6 brackets
	if h == "" {
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	targetIPs := resolveIPs(ctx, h)
	if len(targetIPs) == 0 {
		return false
	}
	for _, ip := range targetIPs {
		if ip.IsLoopback() {
			return true
		}
	}
	return anyIPMatches(targetIPs, localIPs())
}

// resolveIPs resolves host (a literal IP returns itself without DNS) to its IPs,
// bounded by a short timeout. Returns nil on any error.
func resolveIPs(ctx context.Context, host string) []net.IP {
	rctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(rctx, host)
	if err != nil {
		return nil
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips
}

// localIPs returns the IP addresses bound to this host's network interfaces.
func localIPs() []net.IP {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		switch v := a.(type) {
		case *net.IPNet:
			ips = append(ips, v.IP)
		case *net.IPAddr:
			ips = append(ips, v.IP)
		}
	}
	return ips
}

// anyIPMatches reports whether any IP in a equals any IP in b.
func anyIPMatches(a, b []net.IP) bool {
	for _, x := range a {
		for _, y := range b {
			if x.Equal(y) {
				return true
			}
		}
	}
	return false
}

func probeDocker(ctx context.Context, t Target) dockerProbe {
	if t.DockerEndpoint == "" {
		return dockerProbe{} // no explicit endpoint; Docker presence inferred from SSH signal
	}
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	cli, err := docker.New(t.DockerEndpoint, t.DockerTLS)
	if err != nil {
		cat, hint := SanitizeProbeError(err)
		return dockerProbe{errCat: cat, errHint: hint}
	}
	defer cli.Close()
	ver, err := cli.Ping(cctx)
	if err != nil {
		cat, hint := SanitizeProbeError(err)
		return dockerProbe{errCat: cat, errHint: hint}
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
		cat, hint := SanitizeProbeError(err)
		return pveProbe{errCat: cat, errHint: hint}
	}
	return pveProbe{reachable: true, version: ver, status: string(status)}
}

func classify(sp sshProbe, dp dockerProbe, pp pveProbe) Result {
	r := Result{PerProbeError: map[string]string{}, PerProbeHint: map[string]string{}}

	r.OSType = parseOSType(sp.detection.OSReleaseRaw)
	r.ReachableSSH = sp.reachable
	r.SSHHostKeyMismatch = sp.keyMismatch
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
		if sp.errHint != "" {
			r.PerProbeHint["ssh"] = sp.errHint
		}
	}
	if dp.errCat != "" {
		r.PerProbeError["docker"] = dp.errCat
		if dp.errHint != "" {
			r.PerProbeHint["docker"] = dp.errHint
		}
	}
	if pp.errCat != "" {
		r.PerProbeError["proxmox"] = pp.errCat
		if pp.errHint != "" {
			r.PerProbeHint["proxmox"] = pp.errHint
		}
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
