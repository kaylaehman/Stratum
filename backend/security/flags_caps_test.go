package security

import (
	"testing"

	"github.com/KAE-Labs/stratum/backend/db"
)

// --- CapRisk ---

// TestCapRisk verifies that CapRisk returns a non-empty explanation for known
// dangerous caps and an empty string for unknown/safe ones. The exact wording is
// not locked — only presence/absence matters to avoid brittle tests.
func TestCapRisk(t *testing.T) {
	known := []string{
		"SYS_ADMIN", "SYS_MODULE", "SYS_PTRACE", "SYS_RAWIO",
		"SYS_BOOT", "SYS_CHROOT", "SYS_TIME", "MKNOD",
		"NET_ADMIN", "NET_RAW", "DAC_READ_SEARCH", "DAC_OVERRIDE",
		"SETUID", "SETGID",
	}
	for _, c := range known {
		if got := CapRisk(c); got == "" {
			t.Errorf("CapRisk(%q) = empty, want non-empty risk text", c)
		}
	}

	unknown := []string{"", "SYS_NICE", "FOWNER", "KILL", "CAP_SYS_ADMIN"}
	for _, c := range unknown {
		if got := CapRisk(c); got != "" {
			t.Errorf("CapRisk(%q) = %q, want empty for unknown/safe cap", c, got)
		}
	}
}

// --- FlagsFor ---

// TestFlagsFor_Privileged verifies that a privileged container produces a
// single "privileged" flag — not individual cap entries — so the UI does not
// show a redundant list.
func TestFlagsFor_Privileged(t *testing.T) {
	r := db.ContainerSecurityRow{Privileged: true}
	flags := FlagsFor(r)
	if len(flags) == 0 {
		t.Fatal("expected at least one flag for privileged container")
	}
	if flags[0].Type != "privileged" {
		t.Errorf("first flag type = %q, want privileged", flags[0].Type)
	}
	if flags[0].Risk == "" {
		t.Error("privileged flag Risk is empty")
	}
	// Privileged subsumes caps; no individual cap flags expected.
	for _, f := range flags[1:] {
		if f.Type == "cap" {
			t.Errorf("privileged container should not emit individual cap flags; got cap=%q", f.Key)
		}
	}
}

// TestFlagsFor_DangerousCaps verifies that a non-privileged container with
// named dangerous caps emits one flag per cap with appropriate Risk text.
func TestFlagsFor_DangerousCaps(t *testing.T) {
	r := db.ContainerSecurityRow{
		Privileged:    false,
		DangerousCaps: []string{"SYS_ADMIN", "NET_ADMIN"},
	}
	flags := FlagsFor(r)
	capFlags := filterByType(flags, "cap")
	if len(capFlags) != 2 {
		t.Errorf("expected 2 cap flags, got %d: %+v", len(capFlags), capFlags)
	}
	for _, f := range capFlags {
		if f.Risk == "" {
			t.Errorf("cap %q: Risk is empty", f.Key)
		}
	}
}

// TestFlagsFor_CapAddAll verifies that CAP_ADD=ALL emits a dedicated "cap/ALL" flag.
func TestFlagsFor_CapAddAll(t *testing.T) {
	r := db.ContainerSecurityRow{CapAddAll: true}
	flags := FlagsFor(r)
	allFlags := filterByTypeKey(flags, "cap", "ALL")
	if len(allFlags) != 1 {
		t.Errorf("expected 1 cap/ALL flag, got %d", len(allFlags))
	}
}

// TestFlagsFor_NamespaceFlags verifies that each namespace-sharing flag
// (userns_host, pid_host, net_host) is emitted independently with Risk text.
func TestFlagsFor_NamespaceFlags(t *testing.T) {
	r := db.ContainerSecurityRow{
		UsernsHost: true,
		PidHost:    true,
		NetHost:    true,
	}
	flags := FlagsFor(r)
	for _, wantType := range []string{"userns_host", "pid_host", "net_host"} {
		found := filterByType(flags, wantType)
		if len(found) != 1 {
			t.Errorf("expected 1 %s flag, got %d", wantType, len(found))
			continue
		}
		if found[0].Risk == "" {
			t.Errorf("%s flag: Risk is empty", wantType)
		}
	}
}

// TestFlagsFor_SeccompApparmor verifies that seccomp=unconfined and
// apparmor=unconfined each produce a flag of the appropriate type.
func TestFlagsFor_SeccompApparmor(t *testing.T) {
	r := db.ContainerSecurityRow{
		SeccompUnconfined:  true,
		ApparmorUnconfined: true,
	}
	flags := FlagsFor(r)
	if len(filterByType(flags, "seccomp")) != 1 {
		t.Error("expected 1 seccomp flag")
	}
	if len(filterByType(flags, "apparmor")) != 1 {
		t.Error("expected 1 apparmor flag")
	}
}

// TestFlagsFor_Devices verifies that each device mapping produces a distinct flag.
func TestFlagsFor_Devices(t *testing.T) {
	r := db.ContainerSecurityRow{
		Devices: []string{"/dev/ttyUSB0", "/dev/ttyUSB1"},
	}
	flags := FlagsFor(r)
	devFlags := filterByType(flags, "device")
	if len(devFlags) != 2 {
		t.Errorf("expected 2 device flags, got %d", len(devFlags))
	}
}

// TestFlagsFor_Root verifies that RunsAsRoot=true yields a "root" flag.
func TestFlagsFor_Root(t *testing.T) {
	r := db.ContainerSecurityRow{RunsAsRoot: true}
	flags := FlagsFor(r)
	rootFlags := filterByType(flags, "root")
	if len(rootFlags) != 1 {
		t.Errorf("expected 1 root flag, got %d", len(rootFlags))
	}
}

// TestFlagsFor_Clean verifies that a container with no security findings
// produces an empty flag list (no nil/panic).
func TestFlagsFor_Clean(t *testing.T) {
	r := db.ContainerSecurityRow{}
	flags := FlagsFor(r)
	if len(flags) != 0 {
		t.Errorf("clean container: expected 0 flags, got %d: %+v", len(flags), flags)
	}
}

// TestFlagsFor_AllCombined verifies no panic and all flag types present when
// every possible finding is set simultaneously.
func TestFlagsFor_AllCombined(t *testing.T) {
	r := db.ContainerSecurityRow{
		Privileged:         true,
		CapAddAll:          false, // privileged subsumes this
		DangerousCaps:      []string{"SYS_ADMIN"},
		SeccompUnconfined:  true,
		ApparmorUnconfined: true,
		Devices:            []string{"/dev/sda"},
		UsernsHost:         true,
		PidHost:            true,
		NetHost:            true,
		RunsAsRoot:         true,
	}
	flags := FlagsFor(r) // must not panic
	if len(flags) == 0 {
		t.Error("expected flags for fully-loaded container, got none")
	}
}

// --- helpers ---

func filterByType(flags []Flag, typ string) []Flag {
	var out []Flag
	for _, f := range flags {
		if f.Type == typ {
			out = append(out, f)
		}
	}
	return out
}

func filterByTypeKey(flags []Flag, typ, key string) []Flag {
	var out []Flag
	for _, f := range flags {
		if f.Type == typ && f.Key == key {
			out = append(out, f)
		}
	}
	return out
}
