package ssh

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath" // ONLY in tests: local temp-dir manipulation on the Windows host
	"strings"
	"testing"

	"github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"
)

// ── Guard test ────────────────────────────────────────────────────────────────

// TestNoFilepathInSFTPGo asserts that sftp.go does not import the filepath
// package. Importing filepath corrupts POSIX remote paths on Windows by
// rewriting forward slashes to backslashes.
func TestNoFilepathInSFTPGo(t *testing.T) {
	data, err := os.ReadFile("sftp.go")
	if err != nil {
		t.Fatalf("read sftp.go: %v", err)
	}
	// Check for the import statement form, not the package name in comments.
	if strings.Contains(string(data), `"path/filepath"`) {
		t.Fatal(`sftp.go must NOT import "path/filepath" — use "path" for remote POSIX paths`)
	}
}

// ── In-process SSH+SFTP server fixture ───────────────────────────────────────

// sftpFixture is a running in-process SSH server that handles a single
// "sftp" subsystem request against a temporary directory on the local host.
// The SSH server uses InsecureIgnoreHostKey on the client side — this is
// acceptable in tests only, as documented below.
type sftpFixture struct {
	client  *gossh.Client
	tempDir string
	cleanup func()
}

// newSFTPFixture starts a minimal SSH server listening on 127.0.0.1:0,
// generates a fresh ed25519 host key, and returns a connected sftpFixture.
// The sftp server is rooted at a temp directory; use fix.rel() for paths.
func newSFTPFixture(t *testing.T) *sftpFixture {
	t.Helper()

	// Generate server host key.
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := gossh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}

	// Temp dir that the sftp server will serve.
	tmpDir := t.TempDir()

	// SSH server config: accept any password (test-only).
	serverCfg := &gossh.ServerConfig{
		PasswordCallback: func(conn gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			return nil, nil // accept all
		},
	}
	serverCfg.AddHostKey(hostSigner)

	// Listen on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	// Accept and serve one connection in the background.
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return // listener closed
		}
		sshConn, chans, reqs, err := gossh.NewServerConn(conn, serverCfg)
		if err != nil {
			return
		}
		go gossh.DiscardRequests(reqs)
		defer sshConn.Close()

		for newCh := range chans {
			if newCh.ChannelType() != "session" {
				_ = newCh.Reject(gossh.UnknownChannelType, "unknown channel type")
				continue
			}
			ch, requests, err := newCh.Accept()
			if err != nil {
				return
			}
			go func(ch gossh.Channel, requests <-chan *gossh.Request) {
				for req := range requests {
					if req.Type == "subsystem" && len(req.Payload) > 4 {
						name := string(req.Payload[4:])
						if name == "sftp" {
							_ = req.Reply(true, nil)
							// Serve sftp rooted at tmpDir so relative paths work.
							srv, err := sftp.NewServer(ch,
								sftp.WithServerWorkingDirectory(tmpDir),
							)
							if err != nil {
								return
							}
							_ = srv.Serve()
							_ = srv.Close()
							return
						}
					}
					if req.WantReply {
						_ = req.Reply(false, nil)
					}
				}
			}(ch, requests)
		}
	}()

	// Connect a client.
	// NOTE: InsecureIgnoreHostKey is used here IN TESTS ONLY.
	// Production code (Dial) always pins the host key — never uses InsecureIgnoreHostKey.
	addr := ln.Addr().String()
	clientCfg := &gossh.ClientConfig{
		User:            "test",
		Auth:            []gossh.AuthMethod{gossh.Password("test")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // test-only
	}
	client, err := gossh.Dial("tcp", addr, clientCfg)
	if err != nil {
		ln.Close()
		t.Fatalf("dial ssh server: %v", err)
	}

	return &sftpFixture{
		client:  client,
		tempDir: tmpDir,
		cleanup: func() {
			client.Close()
			ln.Close()
			<-done
		},
	}
}

// local returns the local OS path for the given path components under tempDir.
func (f *sftpFixture) local(rel ...string) string {
	return filepath.Join(append([]string{f.tempDir}, rel...)...)
}

// rel returns a POSIX-style relative path for use with the sftp server.
// The server is rooted at tempDir; paths are joined with forward slashes.
// Components must not contain drive letters or absolute path prefixes.
func (f *sftpFixture) rel(parts ...string) string {
	if len(parts) == 0 {
		return "."
	}
	return strings.Join(parts, "/")
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestSFTPReadDir(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	// Create two files in the temp root so ReadDir has something to list.
	if err := os.WriteFile(fix.local("alpha.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fix.local("beta.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	infos, err := SFTPReadDir(fix.client, ".")
	if err != nil {
		t.Fatalf("SFTPReadDir: %v", err)
	}

	names := make(map[string]bool, len(infos))
	for _, fi := range infos {
		names[fi.Name()] = true
	}
	for _, want := range []string{"alpha.txt", "beta.txt"} {
		if !names[want] {
			t.Errorf("expected %q in ReadDir results; got names: %v", want, names)
		}
	}
}

func TestSFTPLstat(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	if err := os.WriteFile(fix.local("lstatfile.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	fi, err := SFTPLstat(fix.client, fix.rel("lstatfile.txt"))
	if err != nil {
		t.Fatalf("SFTPLstat: %v", err)
	}
	if fi.Name() != "lstatfile.txt" {
		t.Errorf("Name = %q, want lstatfile.txt", fi.Name())
	}
	if fi.Size() != 5 {
		t.Errorf("Size = %d, want 5", fi.Size())
	}
}

func TestSFTPMkdir(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	if err := SFTPMkdir(fix.client, fix.rel("sub", "deep")); err != nil {
		t.Fatalf("SFTPMkdir: %v", err)
	}

	info, err := os.Stat(fix.local("sub", "deep"))
	if err != nil || !info.IsDir() {
		t.Errorf("expected sub/deep to be a directory after SFTPMkdir (err=%v)", err)
	}
}

func TestSFTPRename(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	if err := os.WriteFile(fix.local("old.txt"), []byte("rename me"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SFTPRename(fix.client, fix.rel("old.txt"), fix.rel("new.txt")); err != nil {
		t.Fatalf("SFTPRename: %v", err)
	}

	if _, err := os.Stat(fix.local("old.txt")); !os.IsNotExist(err) {
		t.Error("old path should no longer exist")
	}
	if _, err := os.Stat(fix.local("new.txt")); err != nil {
		t.Errorf("new path should exist: %v", err)
	}
}

func TestSFTPRemove_File(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	if err := os.WriteFile(fix.local("todelete.txt"), []byte("bye"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := SFTPRemove(fix.client, fix.rel("todelete.txt"), false); err != nil {
		t.Fatalf("SFTPRemove: %v", err)
	}
	if _, err := os.Stat(fix.local("todelete.txt")); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

func TestSFTPRemove_NonEmptyDirNonRecursiveFails(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	if err := os.MkdirAll(fix.local("nonempty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fix.local("nonempty", "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := SFTPRemove(fix.client, fix.rel("nonempty"), false)
	if err == nil {
		t.Error("expected error removing non-empty dir non-recursively")
	}
}

func TestSFTPRemove_Recursive(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	// Create: tree/a.txt  tree/sub/b.txt
	if err := os.MkdirAll(fix.local("tree", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, rel := range [][]string{{"tree", "a.txt"}, {"tree", "sub", "b.txt"}} {
		if err := os.WriteFile(fix.local(rel...), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := SFTPRemove(fix.client, fix.rel("tree"), true); err != nil {
		t.Fatalf("SFTPRemove recursive: %v", err)
	}
	if _, err := os.Stat(fix.local("tree")); !os.IsNotExist(err) {
		t.Error("tree directory should be fully removed")
	}
}

func TestSFTPCreateAndOpenRead_RoundTrip(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	content := []byte("hello from sftp round-trip test\n")

	// Write via SFTPCreate.
	wc, err := SFTPCreate(fix.client, fix.rel("roundtrip.txt"))
	if err != nil {
		t.Fatalf("SFTPCreate: %v", err)
	}
	if _, err := wc.Write(content); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("WriteCloser.Close: %v", err)
	}

	// Read back via SFTPOpenRead.
	rc, err := SFTPOpenRead(fix.client, fix.rel("roundtrip.txt"))
	if err != nil {
		t.Fatalf("SFTPOpenRead: %v", err)
	}
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if err := rc.Close(); err != nil {
		t.Fatalf("ReadCloser.Close: %v", err)
	}

	if !bytes.Equal(got, content) {
		t.Errorf("round-trip mismatch: got %q, want %q", got, content)
	}
}

func TestSFTPCreate_Truncates(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	writeContent := func(c []byte) {
		t.Helper()
		wc, err := SFTPCreate(fix.client, fix.rel("overwrite.txt"))
		if err != nil {
			t.Fatalf("SFTPCreate: %v", err)
		}
		if _, err := wc.Write(c); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := wc.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	writeContent([]byte("long initial content here"))
	writeContent([]byte("short"))

	rc, err := SFTPOpenRead(fix.client, fix.rel("overwrite.txt"))
	if err != nil {
		t.Fatalf("SFTPOpenRead: %v", err)
	}
	got, _ := io.ReadAll(rc)
	_ = rc.Close()

	if string(got) != "short" {
		t.Errorf("expected truncated content %q, got %q", "short", got)
	}
}

func TestSFTPRealPath(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	// RealPath on "." returns the server's CWD (tempDir).
	got, err := SFTPRealPath(fix.client, ".")
	if err != nil {
		t.Fatalf("SFTPRealPath: %v", err)
	}
	if got == "" {
		t.Error("RealPath returned empty string")
	}
}

// TestSFTPOpenRead_MissingFileErrors verifies SFTPOpenRead returns an error
// (not a panic or partial ReadCloser) when the remote file doesn't exist.
func TestSFTPOpenRead_MissingFileErrors(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	_, err := SFTPOpenRead(fix.client, fix.rel("does-not-exist.txt"))
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// ── Internal helper tests (run against sftp.NewClientPipe for speed) ──────────

// pipedSFTPClient creates a paired sftp server+client over net.Pipe,
// serving files relative to root. Returns the *sftp.Client; caller must close it.
func pipedSFTPClient(t *testing.T, root string) *sftp.Client {
	t.Helper()
	serverConn, clientConn := net.Pipe()

	type rwc struct {
		net.Conn
	}
	srv, err := sftp.NewServer(rwc{serverConn},
		sftp.WithServerWorkingDirectory(root),
	)
	if err != nil {
		t.Fatalf("sftp.NewServer: %v", err)
	}
	go func() {
		_ = srv.Serve()
		_ = srv.Close()
	}()

	sc, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		t.Fatalf("sftp.NewClientPipe: %v", err)
	}
	t.Cleanup(func() {
		sc.Close()
	})
	return sc
}

func TestRemoveAll_FilesAndDirs(t *testing.T) {
	tmp := t.TempDir()
	// tree: tmp/sub   tmp/a.txt   tmp/sub/b.txt
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		filepath.Join(tmp, "a.txt"),
		filepath.Join(sub, "b.txt"),
	} {
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// The sftp server is rooted at the parent of tmp; we pass the basename as root.
	parent := filepath.Dir(tmp)
	base := filepath.Base(tmp)
	sc := pipedSFTPClient(t, parent)

	if err := removeAll(sc, base); err != nil {
		t.Fatalf("removeAll: %v", err)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("tree should be fully removed")
	}
}

// TestPathImportIsUsed is a compile-time confirmation that the path package
// is used in sftp.go. If the import is removed the build fails; this test
// makes the intent explicit.
func TestPathImportIsUsed(t *testing.T) {
	data, err := os.ReadFile("sftp.go")
	if err != nil {
		t.Fatalf("read sftp.go: %v", err)
	}
	if !strings.Contains(string(data), `"path"`) {
		t.Error(`sftp.go should import "path" for remote POSIX path manipulation`)
	}
	// Also double-check no filepath import.
	if strings.Contains(string(data), `"path/filepath"`) {
		t.Fatal(`sftp.go must NOT import "path/filepath"`)
	}
}

// TestSFTPLstat_Directory verifies SFTPLstat returns IsDir=true for a directory.
func TestSFTPLstat_Directory(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	if err := os.Mkdir(fix.local("mydir"), 0o755); err != nil {
		t.Fatal(err)
	}

	fi, err := SFTPLstat(fix.client, fix.rel("mydir"))
	if err != nil {
		t.Fatalf("SFTPLstat dir: %v", err)
	}
	if !fi.IsDir() {
		t.Errorf("expected IsDir=true for directory, got false (name=%q)", fi.Name())
	}
}

// TestSFTPReadLink exercises SFTPReadLink. Symlinks require privileges on
// Windows, so the test is skipped when os.Symlink fails.
func TestSFTPReadLink(t *testing.T) {
	fix := newSFTPFixture(t)
	defer fix.cleanup()

	if err := os.WriteFile(fix.local("target.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(fix.local("target.txt"), fix.local("link.txt")); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	got, err := SFTPReadLink(fix.client, fix.rel("link.txt"))
	if err != nil {
		t.Fatalf("SFTPReadLink: %v", err)
	}
	// The target returned by the sftp server is typically the absolute path.
	// Verify it ends with "target.txt" (platform path separators vary).
	if !strings.HasSuffix(filepath.ToSlash(got), "target.txt") {
		t.Errorf("ReadLink returned %q, expected suffix target.txt", got)
	}
}

// Ensure fmt is referenced (used in package-level tests via t.Fatalf/Errorf,
// but we import it for the unused-import guard; Go collapses this at compile).
var _ = fmt.Sprintf
