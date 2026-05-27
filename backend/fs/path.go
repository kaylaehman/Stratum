package fs

import (
	"errors"
	"path" // NOT path/filepath — remote paths are POSIX; filepath corrupts them on a Windows backend
	"strings"
)

// ErrInvalidPath is returned for empty, relative, or NUL-bearing paths.
var ErrInvalidPath = errors.New("fs: invalid path")

// ValidatePath checks a remote path is absolute and NUL-free, returning the
// cleaned form. It uses stdlib path (POSIX), never path/filepath.
func ValidatePath(p string) (string, error) {
	if p == "" || !path.IsAbs(p) {
		return "", ErrInvalidPath
	}
	if strings.IndexByte(p, 0) >= 0 {
		return "", ErrInvalidPath
	}
	return path.Clean(p), nil
}

// deniedRoots are critical paths that may never be written, deleted, or renamed
// (exact match or any descendant). A compile-time control, not API-configurable.
var deniedRoots = []string{
	"/etc", "/boot", "/bin", "/sbin", "/usr", "/lib", "/lib64", "/proc", "/sys", "/dev",
}

// IsDenied reports whether a path is the filesystem root, one of the critical
// roots, or a descendant of one. Callers pass an already-canonicalized path
// (post-RealPath) for write/delete/rename targets.
func IsDenied(p string) bool {
	clean := path.Clean(p)
	if clean == "/" {
		return true
	}
	for _, root := range deniedRoots {
		if clean == root || strings.HasPrefix(clean, root+"/") {
			return true
		}
	}
	return false
}
