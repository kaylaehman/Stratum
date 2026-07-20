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

// sshPrivKeyNames are the conventional basenames of SSH private keys.
var sshPrivKeyNames = map[string]struct{}{
	"id_rsa": {}, "id_dsa": {}, "id_ecdsa": {}, "id_ed25519": {},
}

// IsSensitiveRead reports whether a path is a credential file (the shadow
// password database, an SSH private key, or the TLS private-key store) that must
// never be served through the file browser — regardless of role. This is a
// narrow, read-side complement to IsDenied: unlike deniedRoots it does NOT block
// all of /etc, so legitimate config-file browsing still works; it blocks only the
// specific secret-bearing files. Callers pass a cleaned path.
func IsSensitiveRead(p string) bool {
	clean := path.Clean(p)
	switch clean {
	case "/etc/shadow", "/etc/gshadow", "/etc/shadow-", "/etc/gshadow-":
		return true
	}
	if clean == "/etc/ssl/private" || strings.HasPrefix(clean, "/etc/ssl/private/") {
		return true
	}
	base := path.Base(clean)
	if _, ok := sshPrivKeyNames[base]; ok {
		return true
	}
	// Private key material inside any .ssh directory (custom-named keys, *.pem/*.key),
	// excluding public keys.
	if strings.Contains(clean, "/.ssh/") && !strings.HasSuffix(base, ".pub") &&
		(strings.HasPrefix(base, "id_") || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key")) {
		return true
	}
	return false
}
