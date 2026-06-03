// Internal tests for env-var service methods (ListEnvVars, RevealEnvVar,
// buildEnvArgs, resolveValue) and Lifecycle with a valid action.
// Uses package stacks (not stacks_test) so the unexported types and helpers
// in create_test.go (buildTestService, fakeFileIO, minimalStore) are available.
package stacks

import (
	"context"
	"errors"
	"strings"
	"testing"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/crypto"
)

// newTestCipher creates a Cipher for tests using a fixed 32-byte key.
func newTestCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	c, err := crypto.New(key)
	if err != nil {
		t.Fatalf("crypto.New: %v", err)
	}
	return c
}

// sealValue encrypts a plaintext using the cipher; helper for test setup.
func sealValue(t *testing.T, cipher *crypto.Cipher, plaintext string) []byte {
	t.Helper()
	blob, err := cipher.Seal([]byte(plaintext))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	return blob
}

// --- ListEnvVars ---

// TestListEnvVars_MasksSecretBacked verifies that secret-backed env vars are
// returned with Masked=true and no plaintext Value.
func TestListEnvVars_MasksSecretBacked(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	_ = store.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "proj", Key: "API_KEY", SecretID: "sec-1",
	})

	vars, err := svc.ListEnvVars(context.Background(), "n1", "proj")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	if len(vars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(vars))
	}
	if !vars[0].Masked {
		t.Error("secret-backed var: Masked should be true")
	}
	if vars[0].Value != "" {
		t.Errorf("secret-backed var: Value should be empty, got %q", vars[0].Value)
	}
	if vars[0].SecretID != "sec-1" {
		t.Errorf("SecretID: got %q, want sec-1", vars[0].SecretID)
	}
}

// TestListEnvVars_MasksPlaintext verifies that plain (non-secret) env vars with
// a stored value are returned with Masked=true and no plaintext in the Value field.
func TestListEnvVars_MasksPlaintext(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	_ = store.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "proj", Key: "DB_PASS", Value: "secret123",
	})

	vars, err := svc.ListEnvVars(context.Background(), "n1", "proj")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	if len(vars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(vars))
	}
	if !vars[0].Masked {
		t.Error("plain var with value: Masked should be true")
	}
	if vars[0].Value != "" {
		t.Errorf("plain var: Value should be empty in list, got %q", vars[0].Value)
	}
}

// TestListEnvVars_Empty verifies that an empty project returns an empty slice
// without error.
func TestListEnvVars_Empty(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	vars, err := svc.ListEnvVars(context.Background(), "n1", "empty-project")
	if err != nil {
		t.Fatalf("ListEnvVars: %v", err)
	}
	if len(vars) != 0 {
		t.Errorf("expected 0 vars, got %d", len(vars))
	}
}

// --- RevealEnvVar ---

// TestRevealEnvVar_PlainValue verifies that a plain (non-secret) env var is
// returned in plaintext by RevealEnvVar.
func TestRevealEnvVar_PlainValue(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	_ = store.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "proj", Key: "HOST", Value: "localhost",
	})

	got, err := svc.RevealEnvVar(context.Background(), "n1", "proj", "HOST")
	if err != nil {
		t.Fatalf("RevealEnvVar: %v", err)
	}
	if got != "localhost" {
		t.Errorf("RevealEnvVar: got %q, want localhost", got)
	}
}

// TestRevealEnvVar_MissingKey verifies that RevealEnvVar returns ErrNotFound
// when the key does not exist.
func TestRevealEnvVar_MissingKey(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	_, err := svc.RevealEnvVar(context.Background(), "n1", "proj", "NONEXISTENT")
	if !errors.Is(err, appdb.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing key, got %v", err)
	}
}

// TestRevealEnvVar_SecretBacked verifies that a secret-backed var is decrypted
// correctly via RevealEnvVar.
func TestRevealEnvVar_SecretBacked(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	cipher := newTestCipher(t)
	svc := &Service{store: store, files: fio, cipher: cipher}

	sealed := sealValue(t, cipher, "topsecret")
	store.secrets["sec-1"] = appdb.SecretRow{
		ID: "sec-1", GroupID: "g1", Key: "API_KEY", ValueEncrypted: sealed,
	}
	_ = store.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "proj", Key: "API_KEY", SecretID: "sec-1",
	})

	got, err := svc.RevealEnvVar(context.Background(), "n1", "proj", "API_KEY")
	if err != nil {
		t.Fatalf("RevealEnvVar (secret): %v", err)
	}
	if got != "topsecret" {
		t.Errorf("RevealEnvVar (secret): got %q, want topsecret", got)
	}
}

// TestRevealEnvVar_SecretBackedNotFound verifies that when the secret ID
// referenced by an env var does not exist in the vault, ErrNotFound is propagated.
func TestRevealEnvVar_SecretBackedNotFound(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	_ = store.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "proj", Key: "TOKEN", SecretID: "nonexistent-secret",
	})

	_, err := svc.RevealEnvVar(context.Background(), "n1", "proj", "TOKEN")
	if !errors.Is(err, appdb.ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing secret, got %v", err)
	}
}

// --- buildEnvArgs ---

// TestBuildEnvArgs_PlainVars verifies that plain (non-secret) env vars are
// expanded to --env KEY=VALUE pairs in deterministic (sorted) order.
func TestBuildEnvArgs_PlainVars(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	vars := []EnvVar{
		{Key: "Z_KEY", Value: "z-value"},
		{Key: "A_KEY", Value: "a-value"},
	}
	args, err := svc.buildEnvArgs(context.Background(), vars)
	if err != nil {
		t.Fatalf("buildEnvArgs: %v", err)
	}
	// Expected: ["--env","A_KEY=a-value","--env","Z_KEY=z-value"] (sorted)
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	for i := 0; i < len(args); i += 2 {
		if args[i] != "--env" {
			t.Errorf("args[%d] = %q, want --env", i, args[i])
		}
		if !strings.Contains(args[i+1], "=") {
			t.Errorf("args[%d] = %q, expected KEY=VALUE form", i+1, args[i+1])
		}
	}
	// Verify sorted order (A_KEY before Z_KEY).
	if !strings.HasPrefix(args[1], "A_KEY=") {
		t.Errorf("sorted: first key-value should be A_KEY=..., got %q", args[1])
	}
	if !strings.HasPrefix(args[3], "Z_KEY=") {
		t.Errorf("sorted: second key-value should be Z_KEY=..., got %q", args[3])
	}
}

// TestBuildEnvArgs_Empty verifies that an empty var list returns nil (not an error).
func TestBuildEnvArgs_Empty(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	args, err := svc.buildEnvArgs(context.Background(), nil)
	if err != nil {
		t.Fatalf("buildEnvArgs(nil): %v", err)
	}
	if len(args) != 0 {
		t.Errorf("expected empty args, got %v", args)
	}
}

// TestBuildEnvArgs_SecretMissingReturnsError verifies that a secret-backed var
// whose secret does not exist causes buildEnvArgs to return an error rather than
// silently emitting an empty value.
func TestBuildEnvArgs_SecretMissingReturnsError(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: newTestCipher(t)}

	vars := []EnvVar{{Key: "TOKEN", SecretID: "missing-id"}}
	_, err := svc.buildEnvArgs(context.Background(), vars)
	if err == nil {
		t.Error("expected error for missing secret, got nil")
	}
}

// --- Lifecycle with valid actions ---

// TestLifecycle_ValidActionsCallDocker verifies that the three valid lifecycle
// actions (stop, start, restart) each produce a `docker compose -p <project> <action>`
// invocation. We use a fakeFileIO stub so no real SSH is needed.
func TestLifecycle_ValidActions(t *testing.T) {
	for _, action := range []string{"stop", "start", "restart"} {
		fio := newFakeFileIO()
		store := newMinimalStore()
		svc := &Service{store: store, files: fio, cipher: nil}

		_, err := svc.Lifecycle(context.Background(), "n1", "myproject", action)
		if err != nil {
			t.Errorf("Lifecycle(action=%q): unexpected error: %v", action, err)
			continue
		}

		// Verify that docker was called with the action.
		var foundAction bool
		for _, call := range fio.execCalls {
			if call.cmd == "docker" {
				for _, arg := range call.args {
					if arg == action {
						foundAction = true
					}
				}
			}
		}
		if !foundAction {
			t.Errorf("Lifecycle(action=%q): docker not called with action", action)
		}
	}
}

// TestLifecycle_ProjectSanitized verifies that the project name passed to
// docker compose is sanitized (traversal chars removed).
func TestLifecycle_ProjectSanitized(t *testing.T) {
	fio := newFakeFileIO()
	store := newMinimalStore()
	svc := &Service{store: store, files: fio, cipher: nil}

	// "evil..proj" sanitizes to "evil--proj".
	_, _ = svc.Lifecycle(context.Background(), "n1", "evil..proj", "stop")

	for _, call := range fio.execCalls {
		if call.cmd != "docker" {
			continue
		}
		for _, arg := range call.args {
			if strings.Contains(arg, "..") {
				t.Errorf("docker arg %q contains '..' — project not sanitized", arg)
			}
		}
	}
}

// --- ReadCompose ---

// TestReadCompose_ReadsFile verifies that ReadCompose delegates to files.Preview
// and returns the content.
func TestReadCompose_ReadsFile(t *testing.T) {
	fio := newFakeFileIO()
	fio.writes["/opt/proj/docker-compose.yml"] = []byte("services:\n  web:\n    image: nginx\n")
	svc := &Service{store: newMinimalStore(), files: fio, cipher: nil}

	content, err := svc.ReadCompose(context.Background(), "n1", "/opt/proj/docker-compose.yml")
	if err != nil {
		t.Fatalf("ReadCompose: %v", err)
	}
	if !strings.Contains(string(content), "nginx") {
		t.Errorf("ReadCompose: content does not contain expected data: %s", content)
	}
}

// TestReadCompose_NotFound verifies that ReadCompose returns an error when the
// file does not exist (no panic).
func TestReadCompose_NotFound(t *testing.T) {
	fio := newFakeFileIO()
	svc := &Service{store: newMinimalStore(), files: fio, cipher: nil}

	_, err := svc.ReadCompose(context.Background(), "n1", "/nonexistent/docker-compose.yml")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}
