// Package security classifies container security posture: dangerous capabilities
// and namespace flags (Feature 19) and published-port exposure (Feature 18).
package security

import (
	"sort"
	"strings"

	"github.com/kaylaehman/stratum/backend/docker"
)

// SecurityFlags is a container's classified security posture.
type SecurityFlags struct {
	Privileged         bool     `json:"privileged"`
	CapAddAll          bool     `json:"cap_add_all"`
	DangerousCaps      []string `json:"dangerous_caps"`
	SeccompUnconfined  bool     `json:"seccomp_unconfined"`
	ApparmorUnconfined bool     `json:"apparmor_unconfined"`
	Devices            []string `json:"devices"`
	UsernsHost         bool     `json:"userns_host"`
	PidHost            bool     `json:"pid_host"`
	NetHost            bool     `json:"net_host"`
	RunsAsRoot         bool     `json:"runs_as_root"`
	RunUID             int      `json:"run_uid"`
}

// HasFlags reports whether any security flag is set (i.e. worth a shield badge).
func (f SecurityFlags) HasFlags() bool {
	return f.Privileged || f.CapAddAll || len(f.DangerousCaps) > 0 ||
		f.SeccompUnconfined || f.ApparmorUnconfined || len(f.Devices) > 0 ||
		f.UsernsHost || f.PidHost || f.NetHost || f.RunsAsRoot
}

// Scan classifies a container's security posture from its inspect data and its
// resolved effective run identity (runUID + isRoot, from SP4). Privileged is
// evaluated FIRST: a privileged container is treated as having the full curated
// cap set and disabled seccomp/apparmor — its empty CapAdd must NOT be read as
// "no dangerous caps".
func Scan(info docker.InspectInfo, runUID int, isRoot bool) (SecurityFlags, []PortExposure) {
	f := SecurityFlags{
		RunUID:     runUID,
		RunsAsRoot: isRoot,
		PidHost:    isHostMode(info.PidMode),
		NetHost:    isHostMode(info.NetworkMode),
		UsernsHost: isHostMode(info.UsernsMode),
		Devices:    info.Devices,
	}
	if f.Devices == nil {
		f.Devices = []string{}
	}
	f.SeccompUnconfined = hasSecurityOpt(info.SecurityOpt, "seccomp", "unconfined")
	f.ApparmorUnconfined = hasSecurityOpt(info.SecurityOpt, "apparmor", "unconfined")

	if info.Privileged {
		f.Privileged = true
		f.DangerousCaps = allCuratedCaps()
	} else {
		var caps []string
		for _, c := range info.CapAdd {
			n := normalizeCap(c)
			if n == "ALL" {
				f.CapAddAll = true
				caps = allCuratedCaps()
				break
			}
			if _, ok := curatedCaps[n]; ok {
				caps = append(caps, n)
			}
		}
		f.DangerousCaps = caps
	}
	if f.DangerousCaps == nil {
		f.DangerousCaps = []string{}
	}
	sort.Strings(f.DangerousCaps)

	return f, classifyPorts(info.Ports)
}

// isHostMode reports whether a PidMode/NetworkMode/UsernsMode string is "host".
func isHostMode(mode string) bool { return mode == "host" }

// hasSecurityOpt reports whether SecurityOpt contains key=value (e.g.
// "seccomp=unconfined"). Docker may render it "seccomp=unconfined" or
// "seccomp:unconfined" depending on version; accept both separators.
func hasSecurityOpt(opts []string, key, value string) bool {
	for _, o := range opts {
		o = strings.TrimSpace(o)
		if o == key+"="+value || o == key+":"+value {
			return true
		}
	}
	return false
}
