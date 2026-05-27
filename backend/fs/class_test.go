package fs

import (
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestModeStrings(t *testing.T) {
	cases := []struct {
		mode      os.FileMode
		wantOctal string
		wantRWX   string
	}{
		{0o644, "0644", "-rw-r--r--"},
		{0o755, "0755", "-rwxr-xr-x"},
		{0o600, "0600", "-rw-------"},
		{os.ModeDir | 0o755, "0755", "drwxr-xr-x"},
		{os.ModeSymlink | 0o777, "0777", "lrwxrwxrwx"},
		{os.ModeSetuid | 0o755, "4755", "-rwsr-xr-x"},
		{os.ModeSetgid | 0o755, "2755", "-rwxr-sr-x"},
		{os.ModeDir | os.ModeSticky | 0o777, "1777", "drwxrwxrwt"},
	}
	for _, c := range cases {
		octal, rwx := ModeStrings(c.mode)
		if octal != c.wantOctal {
			t.Errorf("ModeStrings(%v) octal = %q, want %q", c.mode, octal, c.wantOctal)
		}
		if rwx != c.wantRWX {
			t.Errorf("ModeStrings(%v) rwx = %q, want %q", c.mode, rwx, c.wantRWX)
		}
	}
}

func TestModeStringsSetuidNoExec(t *testing.T) {
	// setuid without owner-execute renders 'S' (capital).
	_, rwx := ModeStrings(os.ModeSetuid | 0o644)
	if rwx != "-rwSr--r--" {
		t.Errorf("setuid-no-exec rwx = %q, want -rwSr--r--", rwx)
	}
}

func TestClasses(t *testing.T) {
	cases := []struct {
		mode os.FileMode
		want []string
	}{
		{0o644, nil},
		{0o755, []string{ClassExec}},
		{os.ModeDir | 0o755, []string{ClassDir}},
		{0o666 | 0o001, []string{ClassExec, ClassWorldWritable}}, // 0o667: world-write + exec(other)
		{os.ModeSetuid | 0o4755, []string{ClassSetuid, ClassExec}},
		{os.ModeSymlink | 0o777, []string{ClassSymlink, ClassWorldWritable}},
	}
	for _, c := range cases {
		got := Classes(c.mode)
		sort.Strings(got)
		want := append([]string{}, c.want...)
		sort.Strings(want)
		if len(got) == 0 && len(want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Classes(%v) = %v, want %v", c.mode, got, want)
		}
	}
}

func TestValidatePath(t *testing.T) {
	good := map[string]string{
		"/etc/hosts":   "/etc/hosts",
		"/home/kayla/": "/home/kayla",
		"/a/../b":      "/b",
	}
	for in, want := range good {
		got, err := ValidatePath(in)
		if err != nil || got != want {
			t.Errorf("ValidatePath(%q) = %q, %v; want %q, nil", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "relative/path", "./x", "a\x00b"} {
		if _, err := ValidatePath(bad); err == nil {
			t.Errorf("ValidatePath(%q): expected error", bad)
		}
	}
	// A NUL in an absolute path is still rejected.
	if _, err := ValidatePath("/etc/ho\x00sts"); err == nil {
		t.Error("ValidatePath with NUL: expected error")
	}
}

func TestIsDenied(t *testing.T) {
	denied := []string{"/", "/etc", "/etc/hosts", "/boot/grub", "/usr/bin/x", "/proc/1", "/dev/sda"}
	for _, p := range denied {
		if !IsDenied(p) {
			t.Errorf("IsDenied(%q) = false, want true", p)
		}
	}
	allowed := []string{"/home/kayla/file", "/srv/data", "/opt/app/config.yml", "/etcfoo", "/var/log/x"}
	for _, p := range allowed {
		if IsDenied(p) {
			t.Errorf("IsDenied(%q) = true, want false", p)
		}
	}
}
