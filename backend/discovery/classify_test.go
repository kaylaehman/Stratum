package discovery

import (
	"testing"

	"github.com/kaylaehman/stratum/backend/ssh"
)

func TestParseOSType(t *testing.T) {
	cases := []struct{ in, want string }{
		{`ID=ubuntu` + "\n" + `ID_LIKE=debian`, "ubuntu"},
		{`ID=debian`, "debian"},
		{`ID="rocky"` + "\n" + `ID_LIKE="rhel centos fedora"`, "rhel"},
		{`ID=almalinux`, "rhel"},
		{`ID=arch`, "arch"},
		{`ID=alpine`, "alpine"},
		{`ID=gentoo`, "other"},
		{``, "other"},
	}
	for _, c := range cases {
		if got := parseOSType(c.in); got != c.want {
			t.Errorf("parseOSType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassifyMatrix(t *testing.T) {
	dockerSSH := ssh.Detection{HasDocker: true, HasSystemctl: true, HasRunSystemd: true, HasCrontab: true}

	cases := []struct {
		name       string
		sp         sshProbe
		dp         dockerProbe
		pp         pveProbe
		wantType   string
		wantAuth   string
		wantDocker bool
		wantProx   bool
	}{
		{
			name:     "proxmox API confirmed",
			sp:       sshProbe{reachable: true, detection: ssh.Detection{HasEtcPve: true, HasSystemctl: true, HasRunSystemd: true}},
			pp:       pveProbe{reachable: true, status: "confirmed", version: "8.2"},
			wantType: "proxmox", wantAuth: "confirmed", wantProx: true,
		},
		{
			name:     "proxmox API 401 unauthed still proxmox",
			sp:       sshProbe{reachable: true},
			pp:       pveProbe{reachable: true, status: "unauthed"},
			wantType: "proxmox", wantAuth: "unauthed", wantProx: true,
		},
		{
			name:     "proxmox marker only (etc/pve, no token)",
			sp:       sshProbe{reachable: true, detection: ssh.Detection{HasEtcPve: true}},
			wantType: "proxmox", wantAuth: "marker_only", wantProx: true,
		},
		{
			name:       "standalone via docker network probe",
			sp:         sshProbe{reachable: true},
			dp:         dockerProbe{reachable: true, version: "1.47"},
			wantType:   "standalone", wantAuth: "none", wantDocker: true,
		},
		{
			name:       "standalone via ssh docker signal only",
			sp:         sshProbe{reachable: true, detection: dockerSSH},
			wantType:   "standalone", wantAuth: "none", wantDocker: true,
		},
		{
			name:     "plain ssh host",
			sp:       sshProbe{reachable: true, detection: ssh.Detection{HasSystemctl: true, HasRunSystemd: true, HasCrontab: true}},
			wantType: "ssh", wantAuth: "none",
		},
		{
			name:     "fully unreachable defaults to ssh",
			sp:       sshProbe{errCat: "ssh_unreachable"},
			wantType: "ssh", wantAuth: "none",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := classify(c.sp, c.dp, c.pp)
			if r.Type != c.wantType {
				t.Errorf("Type = %q, want %q", r.Type, c.wantType)
			}
			if r.ProxmoxAuthStatus != c.wantAuth {
				t.Errorf("ProxmoxAuthStatus = %q, want %q", r.ProxmoxAuthStatus, c.wantAuth)
			}
			if r.Caps.Docker != c.wantDocker {
				t.Errorf("Caps.Docker = %v, want %v", r.Caps.Docker, c.wantDocker)
			}
			if r.Caps.Proxmox != c.wantProx {
				t.Errorf("Caps.Proxmox = %v, want %v", r.Caps.Proxmox, c.wantProx)
			}
		})
	}
}

func TestClassifyCollectsErrors(t *testing.T) {
	r := classify(
		sshProbe{errCat: "ssh_auth_failed"},
		dockerProbe{errCat: "docker_unreachable"},
		pveProbe{errCat: "tls_error"},
	)
	if r.PerProbeError["ssh"] != "ssh_auth_failed" ||
		r.PerProbeError["docker"] != "docker_unreachable" ||
		r.PerProbeError["proxmox"] != "tls_error" {
		t.Errorf("errors not all collected: %+v", r.PerProbeError)
	}
}
