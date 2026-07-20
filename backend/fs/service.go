package fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/permissions"
	appssh "github.com/kaylaehman/stratum/backend/ssh"
)

// DefaultUploadMax is the default per-upload byte cap (STRATUM_UPLOAD_MAX_BYTES).
const DefaultUploadMax = 500 << 20 // 500 MB

// PreviewMax caps the inline file preview.
const PreviewMax = 5 << 20 // 5 MB

// Service errors mapped to HTTP statuses by the API layer.
var (
	ErrDenied   = errors.New("fs: path is on the critical-path deny-list")
	ErrStale    = errors.New("fs: file modified since read (precondition failed)")
	ErrTooLarge = errors.New("fs: upload exceeds the size limit")
	// ErrOffsetMismatch is returned by UploadChunk when the requested offset
	// doesn't match the bytes already written (the caller should re-sync via
	// UploadStatus and resume).
	ErrOffsetMismatch = errors.New("fs: chunk offset does not match received bytes")
	// ErrExists is returned by UploadFinish when the target exists and overwrite
	// was not requested.
	ErrExists = errors.New("fs: destination already exists")
)

// providerOpener returns a FileProvider for a node plus a Closer to release the
// underlying connection. Overridable in tests.
type providerOpener func(ctx context.Context, nodeID string) (FileProvider, io.Closer, error)

// Service orchestrates filesystem operations with safety guards.
type Service struct {
	store     db.Store
	cipher    *crypto.Cipher
	userdb    *permissions.UserDB
	uploadMax int64
	open      providerOpener
}

// NewService builds the production service (SFTP-backed).
func NewService(store db.Store, cipher *crypto.Cipher, uploadMax int64) *Service {
	if uploadMax <= 0 {
		uploadMax = DefaultUploadMax
	}
	s := &Service{store: store, cipher: cipher, uploadMax: uploadMax}
	s.open = s.openSFTP
	s.userdb = permissions.NewUserDB(s.fetchFile, 5*time.Minute)
	return s
}

// UploadMax returns the configured per-write/upload byte cap.
func (s *Service) UploadMax() int64 { return s.uploadMax }

// ResolveUsers returns the host's UID->name / GID->name maps (for SP4's
// host-vs-container comparison).
func (s *Service) ResolveUsers(ctx context.Context, nodeID string) (*permissions.Maps, error) {
	return s.userdb.Resolve(ctx, nodeID)
}

// Exec runs a command on a node over SSH (every arg shell-quoted by the ssh
// package). Used by the diagnostic getfacl path. Callers pass "--" before path
// arguments.
func (s *Service) Exec(ctx context.Context, nodeID, cmd string, args ...string) (string, error) {
	node, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return "", err
	}
	creds, err := nodes.OpenCredentials(s.cipher, node.CredentialsEncrypted)
	if err != nil {
		return "", err
	}
	client, err := appssh.Dial(ctx, node.Host, node.Port, appssh.Credentials{
		User:          creds.SSHUser,
		Password:      creds.SSHPassword,
		PrivateKeyPEM: creds.SSHPrivateKey,
		Passphrase:    creds.SSHPassphrase,
	}, node.SSHHostKey)
	if err != nil {
		return "", err
	}
	defer client.Close()
	return appssh.Run(ctx, client, cmd, args...)
}

// DialSSH opens a raw SSH client to a node for streaming sessions. The
// interactive terminal needs a PTY/shell rather than the one-shot Exec or the
// SFTP helpers, so it dials directly; the caller must Close the returned client.
func (s *Service) DialSSH(ctx context.Context, nodeID string) (*ssh.Client, error) {
	node, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	creds, err := nodes.OpenCredentials(s.cipher, node.CredentialsEncrypted)
	if err != nil {
		return nil, err
	}
	return appssh.Dial(ctx, node.Host, node.Port, appssh.Credentials{
		User:          creds.SSHUser,
		Password:      creds.SSHPassword,
		PrivateKeyPEM: creds.SSHPrivateKey,
		Passphrase:    creds.SSHPassphrase,
	}, node.SSHHostKey)
}

// StatEntry returns a single file's Entry (owner/group resolved) for SP4's
// per-file access analysis.
func (s *Service) StatEntry(ctx context.Context, nodeID, p string) (Entry, error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return Entry{}, err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return Entry{}, err
	}
	defer closer.Close()
	e, err := prov.Stat(ctx, clean)
	if err != nil {
		return Entry{}, err
	}
	if maps, err := s.userdb.Resolve(ctx, nodeID); err == nil {
		e.Owner = maps.UIDToName[e.UID]
		e.Group = maps.GIDToName[e.GID]
	}
	return e, nil
}

func (s *Service) openSFTP(ctx context.Context, nodeID string) (FileProvider, io.Closer, error) {
	node, err := s.store.GetNode(ctx, nodeID)
	if err != nil {
		return nil, nil, err
	}
	creds, err := nodes.OpenCredentials(s.cipher, node.CredentialsEncrypted)
	if err != nil {
		return nil, nil, err
	}
	client, err := appssh.Dial(ctx, node.Host, node.Port, appssh.Credentials{
		User:          creds.SSHUser,
		Password:      creds.SSHPassword,
		PrivateKeyPEM: creds.SSHPrivateKey,
		Passphrase:    creds.SSHPassphrase,
	}, node.SSHHostKey)
	if err != nil {
		return nil, nil, err
	}
	return NewSFTPProvider(client), client, nil
}

func (s *Service) fetchFile(ctx context.Context, nodeID, p string) ([]byte, error) {
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	rc, err := prov.OpenRead(ctx, p)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 8<<20)) // passwd/group are small
}

// List returns a directory's entries with resolved owner/group names.
func (s *Service) List(ctx context.Context, nodeID, p string) ([]Entry, bool, error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return nil, false, err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return nil, false, err
	}
	defer closer.Close()

	entries, truncated, err := prov.List(ctx, clean)
	if err != nil {
		return nil, false, err
	}
	if maps, err := s.userdb.Resolve(ctx, nodeID); err == nil {
		for i := range entries {
			entries[i].Owner = maps.UIDToName[entries[i].UID]
			entries[i].Group = maps.GIDToName[entries[i].GID]
		}
	}
	return entries, truncated, nil
}

// SearchHit is one Search match: the matched entry plus where it lives.
type SearchHit struct {
	Entry
	Path   string `json:"path"`    // absolute path of the matched entry
	RelDir string `json:"rel_dir"` // parent dir relative to the search root ("." at root)
}

// Search bounds: a deep search runs over a single SSH connection on the remote
// host, so it must stay responsive and can never be turned into a full-disk
// crawl. The first cap hit ends the walk and marks the result truncated.
const (
	searchMaxDepth   = 8                // directory levels below the root
	searchMaxResults = 500              // hits returned before truncating
	searchMaxDirs    = 2000             // directories listed before truncating
	searchTimeout    = 20 * time.Second // wall-clock budget for the whole walk
)

// Search walks the tree under root breadth-first and returns entries whose name
// contains query (case-insensitive). The walk reuses one SSH connection and is
// bounded by the search* caps and a deadline; truncated is true when any cap was
// hit before the tree was exhausted. Symlinked directories are not followed (loop
// guard) and the /proc and /sys pseudo-filesystems are never descended into.
func (s *Service) Search(ctx context.Context, nodeID, root, query string) ([]SearchHit, bool, error) {
	clean, err := ValidatePath(root)
	if err != nil {
		return nil, false, err
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil, false, ErrInvalidPath
	}

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return nil, false, err
	}
	defer closer.Close()

	var maps *permissions.Maps
	if s.userdb != nil {
		if m, rerr := s.userdb.Resolve(ctx, nodeID); rerr == nil {
			maps = m
		}
	}

	type queued struct {
		dir   string
		depth int
	}
	queue := []queued{{dir: clean, depth: 0}}
	hits := make([]SearchHit, 0, 64)
	dirsListed := 0
	truncated := false

	for len(queue) > 0 {
		if ctx.Err() != nil || len(hits) >= searchMaxResults || dirsListed >= searchMaxDirs {
			truncated = true
			break
		}
		cur := queue[0]
		queue = queue[1:]

		entries, listTrunc, lerr := prov.List(ctx, cur.dir)
		if lerr != nil {
			// A directory we can't read (permission denied, vanished mid-walk)
			// shouldn't abort the whole search — skip it and keep going.
			continue
		}
		dirsListed++
		if listTrunc {
			truncated = true
		}

		for i := range entries {
			e := entries[i]
			if strings.Contains(strings.ToLower(e.Name), needle) {
				if maps != nil {
					e.Owner = maps.UIDToName[e.UID]
					e.Group = maps.GIDToName[e.GID]
				}
				hits = append(hits, SearchHit{
					Entry:  e,
					Path:   path.Join(cur.dir, e.Name),
					RelDir: relDir(clean, cur.dir),
				})
				if len(hits) >= searchMaxResults {
					truncated = true
					break
				}
			}
			if e.IsDir && !e.IsSymlink && cur.depth < searchMaxDepth {
				child := path.Join(cur.dir, e.Name)
				if child == "/proc" || child == "/sys" {
					continue
				}
				queue = append(queue, queued{dir: child, depth: cur.depth + 1})
			}
		}
	}
	return hits, truncated, nil
}

// relDir renders dir relative to root: "." when equal, otherwise the path with
// the root prefix stripped (root=/var/www dir=/var/www/app -> "app").
func relDir(root, dir string) string {
	if dir == root {
		return "."
	}
	if root == "/" {
		return strings.TrimPrefix(dir, "/")
	}
	return strings.TrimPrefix(dir, root+"/")
}

// Preview returns up to PreviewMax bytes of a file, or tooLarge if it exceeds
// the cap. It also returns the file's modtime for ETag/lost-update protection.
func (s *Service) Preview(ctx context.Context, nodeID, p string) (content []byte, tooLarge bool, modTime time.Time, err error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return nil, false, time.Time{}, err
	}
	// Defense in depth on top of the operator role gate: never serve credential
	// files (shadow db, SSH/TLS private keys) through the browser, for any role.
	if IsSensitiveRead(clean) {
		return nil, false, time.Time{}, ErrDenied
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return nil, false, time.Time{}, err
	}
	defer closer.Close()

	st, err := prov.Stat(ctx, clean)
	if err != nil {
		return nil, false, time.Time{}, err
	}
	rc, err := prov.OpenRead(ctx, clean)
	if err != nil {
		return nil, false, time.Time{}, err
	}
	defer rc.Close()

	buf, err := io.ReadAll(io.LimitReader(rc, PreviewMax+1))
	if err != nil {
		return nil, false, time.Time{}, err
	}
	if int64(len(buf)) > PreviewMax {
		return nil, true, st.ModTime, nil
	}
	return buf, false, st.ModTime, nil
}

// Download streams a file. The returned ReadCloser closes both the file and the
// underlying connection.
func (s *Service) Download(ctx context.Context, nodeID, p string) (io.ReadCloser, error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return nil, err
	}
	if IsSensitiveRead(clean) {
		return nil, ErrDenied
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	rc, err := prov.OpenRead(ctx, clean)
	if err != nil {
		closer.Close()
		return nil, err
	}
	return &multiCloser{rc: rc, extra: closer}, nil
}

// Write atomically replaces a file's content (temp + rename). ifUnmodifiedSince,
// when non-nil, enforces lost-update protection (ErrStale if the file changed).
func (s *Service) Write(ctx context.Context, nodeID, p string, content []byte, ifUnmodifiedSince *time.Time) error {
	return s.streamWrite(ctx, nodeID, p, func(w io.Writer) (int64, error) {
		n, err := w.Write(content)
		return int64(n), err
	}, ifUnmodifiedSince, s.uploadMax)
}

// Upload atomically streams r into a file (temp + rename), capped at uploadMax.
func (s *Service) Upload(ctx context.Context, nodeID, p string, r io.Reader) (int64, error) {
	var written int64
	err := s.streamWrite(ctx, nodeID, p, func(w io.Writer) (int64, error) {
		n, err := io.Copy(w, io.LimitReader(r, s.uploadMax+1))
		written = n
		if n > s.uploadMax {
			return n, ErrTooLarge
		}
		return n, err
	}, nil, s.uploadMax)
	return written, err
}

// ResumableMax is the total-size cap for a chunked/resumable upload (Feature
// F10: default 500MB per file).
const ResumableMax = 500 << 20

// partialSuffix marks the in-progress temp file for a resumable upload. The
// path is deterministic from the target so an interrupted upload resumes by
// stat-ing this file — the backend holds no per-upload session state.
const partialSuffix = ".stratum-upload"

func partialPath(clean string) string { return clean + partialSuffix }

// UploadStatus returns how many bytes of a resumable upload have already landed
// (the size of the partial temp file), or 0 if none exists. Used to resume.
func (s *Service) UploadStatus(ctx context.Context, nodeID, p string) (int64, error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return 0, err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	defer closer.Close()
	st, err := prov.Stat(ctx, partialPath(clean))
	if err != nil {
		return 0, nil // no partial yet => resume from 0
	}
	return st.Size, nil
}

// UploadChunk appends one chunk at the given offset to the resumable temp file.
// offset MUST equal the bytes already received (ErrOffsetMismatch otherwise);
// offset==0 starts fresh (truncating any stale partial). Returns the new total
// received. The chunk is streamed straight to the remote file — never buffered
// whole on the backend.
func (s *Service) UploadChunk(ctx context.Context, nodeID, p string, offset int64, r io.Reader, maxChunk int64) (int64, error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return 0, err
	}
	if offset < 0 {
		return 0, ErrOffsetMismatch
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	defer closer.Close()

	canonical := canonicalizeForWrite(ctx, prov, clean)
	if IsDenied(canonical) {
		return 0, ErrDenied
	}

	tmp := partialPath(clean)
	var have int64
	if st, serr := prov.Stat(ctx, tmp); serr == nil {
		have = st.Size
	}
	if offset != have {
		return have, ErrOffsetMismatch // caller must re-sync via UploadStatus
	}
	if offset+maxChunk > ResumableMax {
		return have, ErrTooLarge
	}

	w, err := prov.OpenWriteAt(ctx, tmp, offset)
	if err != nil {
		return have, err
	}
	n, copyErr := io.Copy(w, io.LimitReader(r, maxChunk+1))
	closeErr := w.Close()
	if copyErr != nil {
		return offset, copyErr
	}
	if closeErr != nil {
		return offset, closeErr
	}
	if n > maxChunk {
		return offset + maxChunk, ErrTooLarge
	}
	return offset + n, nil
}

// UploadFinish renames the completed partial onto the target path. When
// overwrite is false and the target already exists, it returns ErrExists.
func (s *Service) UploadFinish(ctx context.Context, nodeID, p string, overwrite bool) (int64, error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return 0, err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	defer closer.Close()

	canonical := canonicalizeForWrite(ctx, prov, clean)
	if IsDenied(canonical) {
		return 0, ErrDenied
	}
	tmp := partialPath(clean)
	st, err := prov.Stat(ctx, tmp)
	if err != nil {
		return 0, db.ErrNotFound // nothing staged
	}
	if !overwrite {
		if _, serr := prov.Stat(ctx, clean); serr == nil {
			return 0, ErrExists
		}
	}
	if err := prov.Rename(ctx, tmp, clean); err != nil {
		return 0, err
	}
	return st.Size, nil
}

// UploadCancel discards an in-progress resumable upload's temp file.
func (s *Service) UploadCancel(ctx context.Context, nodeID, p string) error {
	clean, err := ValidatePath(p)
	if err != nil {
		return err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return err
	}
	defer closer.Close()
	return prov.Remove(ctx, partialPath(clean), false)
}

func (s *Service) streamWrite(ctx context.Context, nodeID, p string, copyFn func(io.Writer) (int64, error), ifUnmodifiedSince *time.Time, _ int64) error {
	clean, err := ValidatePath(p)
	if err != nil {
		return err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return err
	}
	defer closer.Close()

	canonical := canonicalizeForWrite(ctx, prov, clean)
	if IsDenied(canonical) {
		return ErrDenied
	}
	if ifUnmodifiedSince != nil {
		if st, err := prov.Stat(ctx, clean); err == nil {
			if st.ModTime.After(ifUnmodifiedSince.Add(time.Second)) {
				return ErrStale
			}
		}
	}

	tmp := tempName(clean)
	w, err := prov.OpenWrite(ctx, tmp)
	if err != nil {
		return err
	}
	_, copyErr := copyFn(w)
	closeErr := w.Close()
	if copyErr != nil || closeErr != nil {
		_ = prov.Remove(ctx, tmp, false) // discard partial temp; do not corrupt target
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return prov.Rename(ctx, tmp, clean)
}

// Mkdir creates a directory (deny-list checked).
func (s *Service) Mkdir(ctx context.Context, nodeID, p string) error {
	clean, err := ValidatePath(p)
	if err != nil {
		return err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return err
	}
	defer closer.Close()
	if IsDenied(canonicalizeForWrite(ctx, prov, clean)) {
		return ErrDenied
	}
	return prov.Mkdir(ctx, clean)
}

// Rename moves a file; both source and target are validated, canonicalized, and
// deny-list checked.
func (s *Service) Rename(ctx context.Context, nodeID, from, to string) error {
	cleanFrom, err := ValidatePath(from)
	if err != nil {
		return err
	}
	cleanTo, err := ValidatePath(to)
	if err != nil {
		return err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return err
	}
	defer closer.Close()
	if IsDenied(canonicalizeForWrite(ctx, prov, cleanFrom)) || IsDenied(canonicalizeForWrite(ctx, prov, cleanTo)) {
		return ErrDenied
	}
	return prov.Rename(ctx, cleanFrom, cleanTo)
}

// Remove deletes a path (deny-list checked; recursive required for non-empty dirs).
func (s *Service) Remove(ctx context.Context, nodeID, p string, recursive bool) error {
	clean, err := ValidatePath(p)
	if err != nil {
		return err
	}
	prov, closer, err := s.open(ctx, nodeID)
	if err != nil {
		return err
	}
	defer closer.Close()
	if IsDenied(canonicalizeForWrite(ctx, prov, clean)) {
		return ErrDenied
	}
	return prov.Remove(ctx, clean, recursive)
}

// canonicalizeForWrite resolves symlinks so the deny-list check sees the real
// target. If the path doesn't exist yet (new file), it canonicalizes the parent
// dir and re-appends the base name.
func canonicalizeForWrite(ctx context.Context, prov FileProvider, p string) string {
	if real, err := prov.RealPath(ctx, p); err == nil {
		return real
	}
	if realDir, err := prov.RealPath(ctx, path.Dir(p)); err == nil {
		return path.Join(realDir, path.Base(p))
	}
	return p
}

func tempName(target string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return path.Join(path.Dir(target), ".stratum-tmp-"+hex.EncodeToString(b[:]))
}

type multiCloser struct {
	rc    io.ReadCloser
	extra io.Closer
}

func (m *multiCloser) Read(p []byte) (int, error) { return m.rc.Read(p) }
func (m *multiCloser) Close() error {
	err := m.rc.Close()
	if cerr := m.extra.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}
