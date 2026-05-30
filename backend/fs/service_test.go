package fs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// fakeProvider is an in-memory FileProvider for testing the service's safety
// logic without an SSH/SFTP server.
type fakeProvider struct {
	files    map[string][]byte
	modTimes map[string]time.Time
	symlinks map[string]string  // path -> canonical target (for RealPath)
	dirs     map[string][]Entry // dir path -> its entries (for List/Search)
	removed  []string
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{files: map[string][]byte{}, modTimes: map[string]time.Time{}, symlinks: map[string]string{}, dirs: map[string][]Entry{}}
}

func (f *fakeProvider) List(_ context.Context, dir string) ([]Entry, bool, error) {
	return f.dirs[dir], false, nil
}
func (f *fakeProvider) Stat(_ context.Context, p string) (Entry, error) {
	b, ok := f.files[p]
	if !ok {
		return Entry{}, errors.New("not found")
	}
	return Entry{Name: p, ModTime: f.modTimes[p], Size: int64(len(b))}, nil
}
func (f *fakeProvider) RealPath(_ context.Context, p string) (string, error) {
	if t, ok := f.symlinks[p]; ok {
		return t, nil
	}
	if _, ok := f.files[p]; ok {
		return p, nil
	}
	return "", errors.New("not found")
}
func (f *fakeProvider) OpenRead(_ context.Context, p string) (io.ReadCloser, error) {
	b, ok := f.files[p]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (f *fakeProvider) OpenWrite(_ context.Context, p string) (io.WriteCloser, error) {
	return &fakeWriter{f: f, path: p}, nil
}
func (f *fakeProvider) OpenWriteAt(_ context.Context, p string, offset int64) (io.WriteCloser, error) {
	return &fakeWriter{f: f, path: p, offset: offset, at: true}, nil
}
func (f *fakeProvider) Mkdir(context.Context, string) error { return nil }
func (f *fakeProvider) Rename(_ context.Context, oldPath, newPath string) error {
	b, ok := f.files[oldPath]
	if !ok {
		return errors.New("not found")
	}
	f.files[newPath] = b
	f.modTimes[newPath] = time.Now()
	delete(f.files, oldPath)
	return nil
}
func (f *fakeProvider) Remove(_ context.Context, p string, _ bool) error {
	f.removed = append(f.removed, p)
	delete(f.files, p)
	return nil
}

type fakeWriter struct {
	f      *fakeProvider
	path   string
	buf    bytes.Buffer
	offset int64
	at     bool // offset-write mode (resumable chunk)
}

func (w *fakeWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *fakeWriter) Close() error {
	if w.at && w.offset > 0 {
		// Append at offset: keep the first w.offset bytes, then the new chunk.
		prev := w.f.files[w.path]
		if int64(len(prev)) < w.offset {
			prev = append(prev, make([]byte, w.offset-int64(len(prev)))...)
		}
		w.f.files[w.path] = append(append([]byte(nil), prev[:w.offset]...), w.buf.Bytes()...)
	} else {
		w.f.files[w.path] = append([]byte(nil), w.buf.Bytes()...)
	}
	w.f.modTimes[w.path] = time.Now()
	return nil
}

func newTestService(fp *fakeProvider, uploadMax int64) *Service {
	if uploadMax <= 0 {
		uploadMax = DefaultUploadMax
	}
	return &Service{
		uploadMax: uploadMax,
		open: func(context.Context, string) (FileProvider, io.Closer, error) {
			return fp, io.NopCloser(nil), nil
		},
	}
}

func TestResumableUploadChunks(t *testing.T) {
	fp := newFakeProvider()
	s := newTestService(fp, 0)
	ctx := context.Background()
	const target = "/home/kayla/big.bin"
	partial := target + partialSuffix

	// Fresh upload: status is 0.
	if got, _ := s.UploadStatus(ctx, "n1", target); got != 0 {
		t.Fatalf("initial status = %d, want 0", got)
	}

	// Chunk 1 at offset 0.
	rec, err := s.UploadChunk(ctx, "n1", target, 0, strings.NewReader("hello "), 1<<20)
	if err != nil || rec != 6 {
		t.Fatalf("chunk1 = (%d, %v), want (6, nil)", rec, err)
	}
	// Resume: status reflects 6 received.
	if got, _ := s.UploadStatus(ctx, "n1", target); got != 6 {
		t.Errorf("status after chunk1 = %d, want 6", got)
	}
	// Wrong offset is rejected with the true received count.
	if _, err := s.UploadChunk(ctx, "n1", target, 3, strings.NewReader("x"), 1<<20); err != ErrOffsetMismatch {
		t.Errorf("bad offset err = %v, want ErrOffsetMismatch", err)
	}
	// Chunk 2 at offset 6.
	if rec, err = s.UploadChunk(ctx, "n1", target, 6, strings.NewReader("world"), 1<<20); err != nil || rec != 11 {
		t.Fatalf("chunk2 = (%d, %v), want (11, nil)", rec, err)
	}
	if string(fp.files[partial]) != "hello world" {
		t.Fatalf("partial content = %q", fp.files[partial])
	}

	// Finish renames partial -> target.
	size, err := s.UploadFinish(ctx, "n1", target, false)
	if err != nil || size != 11 {
		t.Fatalf("finish = (%d, %v), want (11, nil)", size, err)
	}
	if string(fp.files[target]) != "hello world" {
		t.Errorf("target content = %q", fp.files[target])
	}
	if _, ok := fp.files[partial]; ok {
		t.Error("partial should be gone after finish")
	}
}

func TestResumableFinishExistsGuard(t *testing.T) {
	fp := newFakeProvider()
	s := newTestService(fp, 0)
	ctx := context.Background()
	const target = "/srv/data.bin"
	fp.files[target] = []byte("existing")

	if _, err := s.UploadChunk(ctx, "n1", target, 0, strings.NewReader("new"), 1<<20); err != nil {
		t.Fatalf("chunk: %v", err)
	}
	// Without overwrite, finishing over an existing file is refused.
	if _, err := s.UploadFinish(ctx, "n1", target, false); err != ErrExists {
		t.Errorf("finish no-overwrite = %v, want ErrExists", err)
	}
	// With overwrite, it replaces.
	if _, err := s.UploadFinish(ctx, "n1", target, true); err != nil {
		t.Fatalf("finish overwrite: %v", err)
	}
	if string(fp.files[target]) != "new" {
		t.Errorf("target after overwrite = %q", fp.files[target])
	}
}

func TestWriteAtomicAndRead(t *testing.T) {
	fp := newFakeProvider()
	s := newTestService(fp, 0)
	if err := s.Write(context.Background(), "n1", "/home/kayla/config.yml", []byte("hello: world"), nil); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if string(fp.files["/home/kayla/config.yml"]) != "hello: world" {
		t.Errorf("content not written via temp+rename: %q", fp.files["/home/kayla/config.yml"])
	}
	// The temp file must have been renamed away, not left behind.
	for p := range fp.files {
		if strings.Contains(p, ".stratum-tmp-") {
			t.Errorf("temp file left behind: %s", p)
		}
	}
}

func TestWriteDeniedPath(t *testing.T) {
	fp := newFakeProvider()
	s := newTestService(fp, 0)
	if err := s.Write(context.Background(), "n1", "/etc/passwd", []byte("x"), nil); !errors.Is(err, ErrDenied) {
		t.Errorf("write to /etc/passwd err = %v, want ErrDenied", err)
	}
}

func TestWriteThroughSymlinkToDeniedTargetRejected(t *testing.T) {
	fp := newFakeProvider()
	// /home/kayla/evil is a symlink whose RealPath resolves into /etc.
	fp.symlinks["/home/kayla/evil"] = "/etc/shadow"
	s := newTestService(fp, 0)
	if err := s.Write(context.Background(), "n1", "/home/kayla/evil", []byte("x"), nil); !errors.Is(err, ErrDenied) {
		t.Errorf("write through symlink to /etc err = %v, want ErrDenied", err)
	}
}

func TestWriteStalePrecondition(t *testing.T) {
	fp := newFakeProvider()
	fp.files["/home/kayla/f"] = []byte("old")
	fp.modTimes["/home/kayla/f"] = time.Now() // modified "now"
	s := newTestService(fp, 0)

	stale := time.Now().Add(-time.Hour) // client read it an hour ago
	if err := s.Write(context.Background(), "n1", "/home/kayla/f", []byte("new"), &stale); !errors.Is(err, ErrStale) {
		t.Errorf("stale write err = %v, want ErrStale", err)
	}
}

func TestUploadOverCapRejected(t *testing.T) {
	fp := newFakeProvider()
	s := newTestService(fp, 16) // 16-byte cap
	_, err := s.Upload(context.Background(), "n1", "/home/kayla/big", strings.NewReader(strings.Repeat("A", 100)))
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("over-cap upload err = %v, want ErrTooLarge", err)
	}
	// The partial temp must have been discarded.
	if len(fp.removed) == 0 {
		t.Error("expected temp file removal after over-cap upload")
	}
}

func TestUploadWithinCap(t *testing.T) {
	fp := newFakeProvider()
	s := newTestService(fp, 1<<20)
	n, err := s.Upload(context.Background(), "n1", "/srv/data/file.txt", strings.NewReader("payload"))
	if err != nil || n != 7 {
		t.Fatalf("Upload = %d, %v", n, err)
	}
	if string(fp.files["/srv/data/file.txt"]) != "payload" {
		t.Errorf("uploaded content wrong: %q", fp.files["/srv/data/file.txt"])
	}
}

func TestInvalidPathRejected(t *testing.T) {
	s := newTestService(newFakeProvider(), 0)
	if err := s.Write(context.Background(), "n1", "relative/path", []byte("x"), nil); !errors.Is(err, ErrInvalidPath) {
		t.Errorf("relative path err = %v, want ErrInvalidPath", err)
	}
}

func TestSearchRecursiveWalk(t *testing.T) {
	fp := newFakeProvider()
	fp.dirs["/srv"] = []Entry{
		{Name: "app", IsDir: true},
		{Name: "readme.md"},
		{Name: "notes.txt"},
	}
	fp.dirs["/srv/app"] = []Entry{
		{Name: "config.yaml"},
		{Name: "sub", IsDir: true},
		{Name: "extlink", IsDir: true, IsSymlink: true}, // symlinked dir: must not be descended
	}
	fp.dirs["/srv/app/sub"] = []Entry{
		{Name: "config.json"},
	}
	// If the symlinked dir were followed, this would surface as a spurious hit.
	fp.dirs["/srv/app/extlink"] = []Entry{
		{Name: "config.LEAKED"},
	}
	svc := newTestService(fp, 0)

	// Case-insensitive substring match across the subtree.
	hits, truncated, err := svc.Search(context.Background(), "n1", "/srv", "CONFIG")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if truncated {
		t.Errorf("unexpected truncation on a small tree")
	}
	got := map[string]string{}
	for _, h := range hits {
		got[h.Name] = h.RelDir
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d: %+v", len(hits), got)
	}
	if got["config.yaml"] != "app" {
		t.Errorf("config.yaml rel_dir = %q, want %q", got["config.yaml"], "app")
	}
	if got["config.json"] != "app/sub" {
		t.Errorf("config.json rel_dir = %q, want %q", got["config.json"], "app/sub")
	}
	if _, leaked := got["config.LEAKED"]; leaked {
		t.Error("search descended into a symlinked directory")
	}

	// A match in the root directory itself reports rel_dir ".".
	rootHits, _, err := svc.Search(context.Background(), "n1", "/srv", "readme")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(rootHits) != 1 || rootHits[0].RelDir != "." || rootHits[0].Path != "/srv/readme.md" {
		t.Errorf("root match = %+v, want one hit rel_dir='.' path='/srv/readme.md'", rootHits)
	}
}

func TestSearchEmptyQueryRejected(t *testing.T) {
	svc := newTestService(newFakeProvider(), 0)
	if _, _, err := svc.Search(context.Background(), "n1", "/srv", "   "); !errors.Is(err, ErrInvalidPath) {
		t.Errorf("blank query err = %v, want ErrInvalidPath", err)
	}
}
