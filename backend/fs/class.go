// Package fs is the filesystem-browser service: SFTP-backed listing, read,
// atomic write, upload, rename, delete, and mkdir over a node's SSH connection,
// with permission/ownership classification and write-path safety guards.
package fs

import (
	"fmt"
	"os"
)

// Ownership/permission classes used for UI color-coding.
const (
	ClassDir           = "dir"
	ClassSymlink       = "symlink"
	ClassWorldWritable = "world_writable"
	ClassSetuid        = "setuid"
	ClassSetgid        = "setgid"
	ClassSticky        = "sticky"
	ClassExec          = "exec"
)

// ModeStrings renders a file mode as a 4-digit octal string (with special bits)
// and a conventional 10-char rwx string (e.g. "0644" / "-rw-r--r--",
// "4755" / "-rwsr-xr-x").
func ModeStrings(mode os.FileMode) (octal, rwx string) {
	perm := uint32(mode.Perm())
	var special uint32
	if mode&os.ModeSetuid != 0 {
		special |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		special |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		special |= 0o1000
	}
	octal = fmt.Sprintf("%04o", perm|special)

	b := []byte("----------")
	switch {
	case mode&os.ModeDir != 0:
		b[0] = 'd'
	case mode&os.ModeSymlink != 0:
		b[0] = 'l'
	}
	const rwxBits = "rwxrwxrwx"
	for i := 0; i < 9; i++ {
		if perm&(1<<uint(8-i)) != 0 {
			b[i+1] = rwxBits[i]
		}
	}
	if mode&os.ModeSetuid != 0 {
		b[3] = specialChar(b[3], 's', 'S')
	}
	if mode&os.ModeSetgid != 0 {
		b[6] = specialChar(b[6], 's', 'S')
	}
	if mode&os.ModeSticky != 0 {
		b[9] = specialChar(b[9], 't', 'T')
	}
	return octal, string(b)
}

// specialChar replaces an x-position with the lowercase form when execute is set
// or the uppercase form when it is not (the conventional ls -l rendering).
func specialChar(cur, lower, upper byte) byte {
	if cur == 'x' {
		return lower
	}
	return upper
}

// Classes returns the ownership/permission classes for a mode.
func Classes(mode os.FileMode) []string {
	var c []string
	if mode&os.ModeDir != 0 {
		c = append(c, ClassDir)
	}
	if mode&os.ModeSymlink != 0 {
		c = append(c, ClassSymlink)
	}
	if mode.Perm()&0o002 != 0 {
		c = append(c, ClassWorldWritable)
	}
	if mode&os.ModeSetuid != 0 {
		c = append(c, ClassSetuid)
	}
	if mode&os.ModeSetgid != 0 {
		c = append(c, ClassSetgid)
	}
	if mode&os.ModeSticky != 0 {
		c = append(c, ClassSticky)
	}
	if mode&os.ModeDir == 0 && mode&os.ModeSymlink == 0 && mode.Perm()&0o111 != 0 {
		c = append(c, ClassExec)
	}
	return c
}
