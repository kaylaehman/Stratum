package fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"path"
	"time"

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

// Preview returns up to PreviewMax bytes of a file, or tooLarge if it exceeds
// the cap. It also returns the file's modtime for ETag/lost-update protection.
func (s *Service) Preview(ctx context.Context, nodeID, p string) (content []byte, tooLarge bool, modTime time.Time, err error) {
	clean, err := ValidatePath(p)
	if err != nil {
		return nil, false, time.Time{}, err
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
