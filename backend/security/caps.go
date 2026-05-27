package security

import "strings"

// curatedCaps maps dangerous Linux capabilities (bare names, no CAP_ prefix) to
// a plain-language risk explanation. These are the caps worth flagging when
// added via --cap-add; a privileged container is treated as having all of them.
var curatedCaps = map[string]string{
	"SYS_ADMIN":      "near-root: mount filesystems, change namespaces — the broadest single capability, often a container-escape vector",
	"SYS_MODULE":     "load/unload kernel modules — full host kernel compromise",
	"SYS_PTRACE":     "trace/attach to any process — can read secrets from other processes and the host",
	"SYS_RAWIO":      "raw I/O port and memory access — can bypass kernel protections",
	"SYS_BOOT":       "reboot the host",
	"SYS_CHROOT":     "use chroot — can be combined with other caps to escape",
	"NET_ADMIN":      "reconfigure host networking, firewall, interfaces",
	"NET_RAW":        "craft raw packets — enables spoofing and some network attacks",
	"DAC_READ_SEARCH": "bypass file read permission checks — read any file on shared mounts",
	"DAC_OVERRIDE":   "bypass all file permission checks — read/write any file the process can reach",
	"SETUID":         "change process UID — can become any user",
	"SETGID":         "change process GID — can join any group",
}

// normalizeCap strips a CAP_ prefix and upper-cases (Docker accepts both forms).
func normalizeCap(c string) string {
	c = strings.ToUpper(strings.TrimSpace(c))
	return strings.TrimPrefix(c, "CAP_")
}

// CapRisk returns the risk explanation for a dangerous cap (bare name), or "".
func CapRisk(bareName string) string { return curatedCaps[bareName] }

// allCuratedCaps returns every curated cap name (used for a privileged container,
// which effectively has them all and disables seccomp/apparmor/device limits).
func allCuratedCaps() []string {
	out := make([]string, 0, len(curatedCaps))
	for c := range curatedCaps {
		out = append(out, c)
	}
	return out
}
