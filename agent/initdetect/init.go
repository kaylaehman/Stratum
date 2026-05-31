// Package initdetect probes the host to determine which init/service-manager
// is active. Detection order follows the agent spec: systemd → openrc →
// fallback (other). The result is surfaced via the DetectInit gRPC RPC and
// stored in node capabilities_json.
package initdetect

import (
	"os"
	"os/exec"
	"strings"

	stratumv1 "github.com/kaylaehman/stratum/proto/gen/stratum/v1"
)

// osExecutable and osStatFile are variables so tests can replace them without
// requiring root or a real systemd.
var (
	osStatFile  = os.Stat
	lookupPath  = exec.LookPath
	runCommand  = runCmd
)

// Detect probes the host and returns an InitSystem enum + description string.
func Detect() (stratumv1.InitSystem, string) {
	if isSystemd() {
		desc := systemdVersion()
		return stratumv1.InitSystem_INIT_SYSTEM_SYSTEMD, desc
	}
	if isOpenRC() {
		desc := openrcVersion()
		return stratumv1.InitSystem_INIT_SYSTEM_OPENRC, desc
	}
	return stratumv1.InitSystem_INIT_SYSTEM_OTHER, "unknown init system"
}

// isSystemd checks whether PID 1 is systemd.
func isSystemd() bool {
	// /run/systemd/private exists only on live systemd hosts.
	if _, err := osStatFile("/run/systemd/private"); err == nil {
		return true
	}
	// Fallback: check the symlink /proc/1/exe → .../systemd.
	link, err := os.Readlink("/proc/1/exe")
	if err != nil {
		return false
	}
	return strings.Contains(link, "systemd") && !strings.Contains(link, "systemd-")
}

// isOpenRC checks whether openrc is available as a binary.
func isOpenRC() bool {
	_, err := lookupPath("openrc")
	return err == nil
}

func systemdVersion() string {
	out, err := runCommand("systemctl", "--version")
	if err != nil {
		return "systemd"
	}
	first := strings.SplitN(strings.TrimSpace(out), "\n", 2)[0]
	return first
}

func openrcVersion() string {
	out, err := runCommand("openrc", "--version")
	if err != nil {
		return "openrc"
	}
	return strings.TrimSpace(out)
}

// runCmd executes a command and returns its combined output.
func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	b, err := cmd.Output()
	return string(b), err
}
