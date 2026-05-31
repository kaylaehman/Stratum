package initdetect

import (
	"errors"
	"os"
	"testing"

	stratumv1 "github.com/kaylaehman/stratum/proto/gen/stratum/v1"
)

// TestDetectSystemdViaRunDir simulates a host that has /run/systemd/private.
func TestDetectSystemdViaRunDir(t *testing.T) {
	origStat := osStatFile
	origLookup := lookupPath
	origRun := runCommand
	defer func() {
		osStatFile = origStat
		lookupPath = origLookup
		runCommand = origRun
	}()

	osStatFile = func(name string) (os.FileInfo, error) {
		if name == "/run/systemd/private" {
			return nil, nil // stat succeeds
		}
		return nil, os.ErrNotExist
	}
	runCommand = func(name string, args ...string) (string, error) {
		if name == "systemctl" {
			return "systemd 252 (252-13+deb12u1)", nil
		}
		return "", errors.New("unexpected command")
	}
	lookupPath = func(file string) (string, error) { return "", errors.New("not found") }

	sys, desc := Detect()
	if sys != stratumv1.InitSystem_INIT_SYSTEM_SYSTEMD {
		t.Errorf("Detect() = %v, want SYSTEMD", sys)
	}
	if desc == "" {
		t.Error("description must not be empty")
	}
}

// TestDetectOpenRC simulates a host with no systemd but openrc available.
func TestDetectOpenRC(t *testing.T) {
	origStat := osStatFile
	origLookup := lookupPath
	origRun := runCommand
	defer func() {
		osStatFile = origStat
		lookupPath = origLookup
		runCommand = origRun
	}()

	osStatFile = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	lookupPath = func(file string) (string, error) {
		if file == "openrc" {
			return "/sbin/openrc", nil
		}
		return "", errors.New("not found")
	}
	runCommand = func(name string, args ...string) (string, error) {
		if name == "openrc" {
			return "openrc 0.44.8", nil
		}
		return "", errors.New("unexpected command")
	}

	sys, desc := Detect()
	if sys != stratumv1.InitSystem_INIT_SYSTEM_OPENRC {
		t.Errorf("Detect() = %v, want OPENRC", sys)
	}
	if desc == "" {
		t.Error("description must not be empty")
	}
}

// TestDetectOther simulates a host with neither systemd nor openrc.
func TestDetectOther(t *testing.T) {
	origStat := osStatFile
	origLookup := lookupPath
	defer func() {
		osStatFile = origStat
		lookupPath = origLookup
	}()

	osStatFile = func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	lookupPath = func(_ string) (string, error) { return "", errors.New("not found") }

	sys, _ := Detect()
	if sys != stratumv1.InitSystem_INIT_SYSTEM_OTHER {
		t.Errorf("Detect() = %v, want OTHER", sys)
	}
}
