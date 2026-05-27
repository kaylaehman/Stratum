package ssh

// SFTP operations layer for Stratum.
//
// Remote path manipulation MUST use stdlib path (not path/filepath).
// path/filepath is FORBIDDEN here: it rewrites slashes to backslashes on Windows,
// corrupting POSIX remote paths. A guard test asserts this file does not import
// the filepath package.
//
// Each non-streaming operation opens a per-operation *sftp.Client and defers
// its close. Streaming operations (OpenRead / Create) return a struct whose
// Close method owns and closes both the *sftp.File and the *sftp.Client.

import (
	"io"
	"os"
	"path"
	"sort"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPReadDir lists a directory's contents. Symlinks in the directory are not
// followed (sftp.ReadDir uses Lstat internally for the entries themselves).
func SFTPReadDir(client *ssh.Client, dir string) ([]os.FileInfo, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	defer sc.Close()
	return sc.ReadDir(dir)
}

// SFTPLstat stats a path without following a final symlink.
func SFTPLstat(client *ssh.Client, p string) (os.FileInfo, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	defer sc.Close()
	return sc.Lstat(p)
}

// SFTPReadLink returns a symlink's target.
func SFTPReadLink(client *ssh.Client, p string) (string, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return "", err
	}
	defer sc.Close()
	return sc.ReadLink(p)
}

// SFTPRealPath canonicalizes a remote path (resolves symlinks server-side).
// Use before writes to ensure the caller operates on the canonical location.
func SFTPRealPath(client *ssh.Client, p string) (string, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return "", err
	}
	defer sc.Close()
	return sc.RealPath(p)
}

// SFTPMkdir creates a directory and all required parents (MkdirAll).
func SFTPMkdir(client *ssh.Client, p string) error {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sc.Close()
	return sc.MkdirAll(p)
}

// SFTPRename renames (or moves) oldPath to newPath on the remote host.
func SFTPRename(client *ssh.Client, oldPath, newPath string) error {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sc.Close()
	return sc.Rename(oldPath, newPath)
}

// SFTPRemove removes a file, or a directory tree when recursive is true.
// A non-recursive call on a non-empty directory returns an error.
func SFTPRemove(client *ssh.Client, p string, recursive bool) error {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return err
	}
	defer sc.Close()

	if !recursive {
		return sc.Remove(p)
	}

	return removeAll(sc, p)
}

// removeAll performs a post-order recursive removal using the sftp.Client
// Walker. Files are deleted first; then directories are removed deepest-first.
func removeAll(sc *sftp.Client, root string) error {
	// Collect all entries in walk order (lexical, pre-order).
	type entry struct {
		path  string
		isDir bool
	}
	var entries []entry

	w := sc.Walk(root)
	for w.Step() {
		if err := w.Err(); err != nil {
			return err
		}
		entries = append(entries, entry{
			path:  w.Path(),
			isDir: w.Stat().IsDir(),
		})
	}

	// Delete files first (any order).
	for _, e := range entries {
		if !e.isDir {
			if err := sc.Remove(e.path); err != nil {
				return err
			}
		}
	}

	// Delete directories deepest-first: reverse lexical order so children
	// come before parents (since Walk is pre-order lexical, a simple reverse
	// gives us correct post-order for directory removal).
	dirs := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.isDir {
			dirs = append(dirs, e.path)
		}
	}
	// Sort descending: longer (deeper) paths sort before shorter parent paths.
	sort.Slice(dirs, func(i, j int) bool { return dirs[i] > dirs[j] })

	for _, d := range dirs {
		if err := sc.RemoveDirectory(d); err != nil {
			return err
		}
	}
	return nil
}

// readCloser wraps an *sftp.File and the *sftp.Client that owns it.
// Close closes the file then the client, releasing the SSH channel.
type readCloser struct {
	file   *sftp.File
	client *sftp.Client
}

func (r *readCloser) Read(p []byte) (int, error) { return r.file.Read(p) }
func (r *readCloser) Close() error {
	fileErr := r.file.Close()
	clientErr := r.client.Close()
	if fileErr != nil {
		return fileErr
	}
	return clientErr
}

// SFTPOpenRead opens a remote file for streaming read.
// The returned io.ReadCloser owns and closes the per-op sftp client on Close.
func SFTPOpenRead(client *ssh.Client, p string) (io.ReadCloser, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	f, err := sc.Open(p)
	if err != nil {
		sc.Close()
		return nil, err
	}
	return &readCloser{file: f, client: sc}, nil
}

// writeCloser wraps an *sftp.File and the *sftp.Client that owns it.
// Close closes the file then the client, releasing the SSH channel.
type writeCloser struct {
	file   *sftp.File
	client *sftp.Client
}

func (w *writeCloser) Write(p []byte) (int, error) { return w.file.Write(p) }
func (w *writeCloser) Close() error {
	fileErr := w.file.Close()
	clientErr := w.client.Close()
	if fileErr != nil {
		return fileErr
	}
	return clientErr
}

// SFTPCreate opens (or creates) a remote file for streaming write, truncating
// any existing content. The returned io.WriteCloser owns and closes the per-op
// sftp client on Close.
func SFTPCreate(client *ssh.Client, p string) (io.WriteCloser, error) {
	sc, err := sftp.NewClient(client)
	if err != nil {
		return nil, err
	}
	// MkdirAll parent so creates don't fail on missing intermediate dirs.
	if dir := path.Dir(p); dir != "." && dir != "/" {
		// Best-effort: ignore error (dir may already exist).
		_ = sc.MkdirAll(dir)
	}
	f, err := sc.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		sc.Close()
		return nil, err
	}
	return &writeCloser{file: f, client: sc}, nil
}
