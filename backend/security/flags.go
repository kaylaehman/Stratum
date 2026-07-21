package security

import "github.com/KAE-Labs/stratum/backend/db"

// Flag is one security finding with a stable identity (Type+Key) used for
// acknowledgement matching, plus plain-language risk text.
type Flag struct {
	Type string `json:"type"` // privileged | cap | seccomp | apparmor | device | userns_host | pid_host | net_host | root
	Key  string `json:"key"`  // e.g. SYS_ADMIN, the device mapping, or "" for singletons
	Risk string `json:"risk"`
}

// FlagsFor enumerates a container's security findings. A privileged container
// subsumes the individual caps (which Scan populated as the full set), so they
// are not re-listed — the single "privileged" flag stands for them.
func FlagsFor(r db.ContainerSecurityRow) []Flag {
	var fs []Flag
	if r.Privileged {
		fs = append(fs, Flag{"privileged", "", "container runs --privileged: full capability set, device access, and seccomp/AppArmor disabled — effectively root on the host"})
	} else {
		if r.CapAddAll {
			fs = append(fs, Flag{"cap", "ALL", "--cap-add=ALL grants every capability — equivalent to privileged"})
		}
		for _, c := range r.DangerousCaps {
			fs = append(fs, Flag{"cap", c, CapRisk(c)})
		}
	}
	if r.SeccompUnconfined {
		fs = append(fs, Flag{"seccomp", "", "seccomp=unconfined: the syscall filter is disabled, widening the kernel attack surface"})
	}
	if r.ApparmorUnconfined {
		fs = append(fs, Flag{"apparmor", "", "apparmor=unconfined: the AppArmor profile is disabled"})
	}
	for _, d := range r.Devices {
		fs = append(fs, Flag{"device", d, "a host device is exposed into the container (--device " + d + ")"})
	}
	if r.UsernsHost {
		fs = append(fs, Flag{"userns_host", "", "--userns=host: no user-namespace isolation; container UIDs map directly to host UIDs"})
	}
	if r.PidHost {
		fs = append(fs, Flag{"pid_host", "", "--pid=host: the container sees and can signal all host processes"})
	}
	if r.NetHost {
		fs = append(fs, Flag{"net_host", "", "--network=host: the container shares the host network namespace (no port isolation)"})
	}
	if r.RunsAsRoot {
		fs = append(fs, Flag{"root", "", "the container process runs as root (UID 0)"})
	}
	return fs
}
