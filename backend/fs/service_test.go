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
	symlinks map[string]string // path -> canonical target (for RealPath)
	removed  []string
}

func newFakeProvider() *fakeProvider {
	return &fakeProvider{files: map[string][]byte{}, modTimes: map[string]time.Time{}, symlinks: map[string]string{}}
}

func (f *fakeProvider) List(context.Context, string) ([]Entry, bool, error) { return nil, false, nil }
func (f *fakeProvider) Stat(_ context.Context, p string) (Entry, error) {
	if _, ok := f.files[p]; !ok {
		return Entry{}, errors.New("not found")
	}
	return Entry{Name: p, ModTime: f.modTimes[p]}, nil
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
	f    *fakeProvider
	path string
	buf  bytes.Buffer
}

func (w *fakeWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *fakeWriter) Close() error {
	w.f.files[w.path] = append([]byte(nil), w.buf.Bytes()...)
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
