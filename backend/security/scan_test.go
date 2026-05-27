package security

import (
	"sort"
	"testing"

	"github.com/kaylaehman/stratum/backend/docker"
)

func TestScanPrivilegedDoesNotReadCapAdd(t *testing.T) {
	// A privileged container with an EMPTY CapAdd must still be flagged with the
	// full curated cap set — never a false "no dangerous caps".
	f, _ := Scan(docker.InspectInfo{Privileged: true, CapAdd: []string{}}, 0, true)
	if !f.Privileged {
		t.Fatal("privileged not flagged")
	}
	if len(f.DangerousCaps) != len(curatedCaps) {
		t.Errorf("privileged should imply all %d curated caps, got %d", len(curatedCaps), len(f.DangerousCaps))
	}
}

func TestScanCapAddAll(t *testing.T) {
	f, _ := Scan(docker.InspectInfo{CapAdd: []string{"ALL"}}, 1000, false)
	if !f.CapAddAll || len(f.DangerousCaps) != len(curatedCaps) {
		t.Errorf("--cap-add=ALL should be privileged-equivalent, got %+v", f)
	}
}

func TestScanDangerousCapsNormalized(t *testing.T) {
	f, _ := Scan(docker.InspectInfo{CapAdd: []string{"CAP_SYS_ADMIN", "NET_RAW", "CHOWN" /* not curated */}}, 1000, false)
	sort.Strings(f.DangerousCaps)
	if len(f.DangerousCaps) != 2 || f.DangerousCaps[0] != "NET_RAW" || f.DangerousCaps[1] != "SYS_ADMIN" {
		t.Errorf("expected [NET_RAW SYS_ADMIN], got %v", f.DangerousCaps)
	}
}

func TestScanCapDropSubtracted(t *testing.T) {
	// --cap-add=ALL --cap-drop=NET_RAW: still cap-add-all, but the per-cap detail
	// must not list a cap the operator explicitly dropped.
	f, _ := Scan(docker.InspectInfo{CapAdd: []string{"ALL"}, CapDrop: []string{"CAP_NET_RAW"}}, 0, false)
	if !f.CapAddAll {
		t.Fatal("cap-add=ALL should still set CapAddAll even with a drop")
	}
	for _, c := range f.DangerousCaps {
		if c == "NET_RAW" {
			t.Errorf("dropped cap NET_RAW must not appear in DangerousCaps: %v", f.DangerousCaps)
		}
	}
	if len(f.DangerousCaps) != len(curatedCaps)-1 {
		t.Errorf("expected all curated caps minus 1, got %d of %d", len(f.DangerousCaps), len(curatedCaps))
	}
}

func TestScanSecurityOptAndNamespaces(t *testing.T) {
	f, _ := Scan(docker.InspectInfo{
		SecurityOpt: []string{"seccomp=unconfined", "apparmor:unconfined"},
		PidMode:     "host",
		NetworkMode: "host",
		UsernsMode:  "host",
		Devices:     []string{"/dev/sda:/dev/sda"},
	}, 0, true)
	if !f.SeccompUnconfined || !f.ApparmorUnconfined {
		t.Error("seccomp/apparmor unconfined not detected (both separators)")
	}
	if !f.PidHost || !f.NetHost || !f.UsernsHost {
		t.Error("host namespaces not detected")
	}
	if len(f.Devices) != 1 || !f.RunsAsRoot {
		t.Errorf("devices/root not detected: %+v", f)
	}
}

func TestScanCleanContainer(t *testing.T) {
	f, _ := Scan(docker.InspectInfo{NetworkMode: "bridge", PidMode: ""}, 1000, false)
	if f.HasFlags() {
		t.Errorf("a non-privileged non-root bridged container should have no flags, got %+v", f)
	}
}

func TestClassifyPorts(t *testing.T) {
	ports := []docker.PortBinding{
		{ContainerPort: 80, Protocol: "tcp", HostIP: "0.0.0.0", HostPort: 8080},
		{ContainerPort: 5432, Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 5432},
		{ContainerPort: 9000, Protocol: "tcp", HostIP: "192.168.1.50", HostPort: 9000},
		{ContainerPort: 53, Protocol: "udp", HostIP: "::", HostPort: 53},
		{ContainerPort: 22, Protocol: "tcp", HostIP: "::1", HostPort: 2222},
	}
	got := classifyPorts(ports)
	want := []string{IfaceAll, IfaceLoopback, IfaceExternal, IfaceAll, IfaceLoopback}
	for i, e := range got {
		if e.InterfaceClass != want[i] {
			t.Errorf("port %d (%s) class = %q, want %q", e.HostPort, e.HostIP, e.InterfaceClass, want[i])
		}
	}
}
