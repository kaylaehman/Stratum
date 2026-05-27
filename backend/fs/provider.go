package fs

import (
	"context"
	"io"
	"time"
)

// Entry is one directory entry. Owner/Group are resolved by the service via
// permissions.UserDB; the provider leaves them empty.
type Entry struct {
	Name       string    `json:"name"`
	IsDir      bool      `json:"is_dir"`
	IsSymlink  bool      `json:"is_symlink"`
	LinkTarget string    `json:"link_target,omitempty"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"mod_time"`
	ModeOctal  string    `json:"mode_octal"`
	ModeRWX    string    `json:"mode_rwx"`
	UID        int       `json:"uid"`
	GID        int       `json:"gid"`
	Owner      string    `json:"owner,omitempty"`
	Group      string    `json:"group,omitempty"`
	Classes    []string  `json:"classes,omitempty"`
}

// FileProvider abstracts the file transport (SFTP today; docker-exec / agent
// later). Implementations operate on already-validated absolute POSIX paths.
type FileProvider interface {
	List(ctx context.Context, path string) (entries []Entry, truncated bool, err error)
	Stat(ctx context.Context, path string) (Entry, error)
	RealPath(ctx context.Context, path string) (string, error)
	OpenRead(ctx context.Context, path string) (io.ReadCloser, error)
	OpenWrite(ctx context.Context, path string) (io.WriteCloser, error)
	// OpenWriteAt opens for writing at a byte offset (resumable chunked upload);
	// offset==0 truncates.
	OpenWriteAt(ctx context.Context, path string, offset int64) (io.WriteCloser, error)
	Mkdir(ctx context.Context, path string) error
	Rename(ctx context.Context, oldPath, newPath string) error
	Remove(ctx context.Context, path string, recursive bool) error
}

// listSoftCap bounds the entries returned for one directory; beyond it the
// listing is truncated and the frontend notes truncation.
const listSoftCap = 10000
