package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	validSecret = "0123456789abcdef0123456789abcdef" // 32 bytes
	validKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
)

func baseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "sqlite:///data/stratum.db")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("ENCRYPTION_KEY", validKeyHex)
	// Ensure optional vars don't leak from the host env.
	t.Setenv("PORT", "")
}

func TestLoadValid(t *testing.T) {
	baseEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("default Port = %d, want 8080", cfg.Port)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("EncryptionKey len = %d, want 32", len(cfg.EncryptionKey))
	}
}

func TestLoadMissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", validSecret)
	t.Setenv("ENCRYPTION_KEY", validKeyHex)
	if _, err := Load(); err == nil {
		t.Error("expected error for missing DATABASE_URL")
	}
}

func TestLoadShortJWTSecret(t *testing.T) {
	baseEnv(t)
	t.Setenv("JWT_SECRET", "tooshort")
	if _, err := Load(); err == nil {
		t.Error("expected error for short JWT_SECRET")
	}
}

func TestLoadBadEncryptionKey(t *testing.T) {
	baseEnv(t)
	t.Setenv("ENCRYPTION_KEY", "not-hex")
	if _, err := Load(); err == nil {
		t.Error("expected error for non-hex ENCRYPTION_KEY")
	}
}

func TestLoadWrongLengthEncryptionKey(t *testing.T) {
	baseEnv(t)
	t.Setenv("ENCRYPTION_KEY", "00112233") // 4 bytes
	if _, err := Load(); err == nil {
		t.Error("expected error for wrong-length ENCRYPTION_KEY")
	}
}

// TestConfigErrorsNeverEchoSecrets enforces SECURITY.md: ENCRYPTION_KEY /
// JWT_SECRET values must never appear in error messages (logs surface these).
// Each case feeds a distinctive sentinel secret and asserts the validation error
// does not contain it — only a remediation hint.
func TestConfigErrorsNeverEchoSecrets(t *testing.T) {
	cases := []struct {
		name, envKey, value string
	}{
		{"short jwt secret", "JWT_SECRET", "SENTINEL-jwt-secret-value-xyz"},
		{"bad encryption key", "ENCRYPTION_KEY", "SENTINEL-encryption-key-zzz"},
		{"wrong-length encryption key", "ENCRYPTION_KEY", "deadbeefSENTINEL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseEnv(t)
			t.Setenv(tc.envKey, tc.value)
			_, err := Load()
			if err == nil {
				t.Fatalf("expected a validation error for %s", tc.name)
			}
			if strings.Contains(err.Error(), tc.value) {
				t.Errorf("error message leaks the %s value: %q", tc.envKey, err.Error())
			}
			if strings.Contains(err.Error(), "SENTINEL") {
				t.Errorf("error message leaks a secret fragment: %q", err.Error())
			}
		})
	}
}

func TestLoadInvalidPort(t *testing.T) {
	baseEnv(t)
	t.Setenv("PORT", "70000")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Errorf("expected PORT error, got %v", err)
	}
}

// --- ENCRYPTION_KEY_FILE tests ---

// writeKeyFile writes content to a temp file and returns the path.
// The file is removed when t ends.
func writeKeyFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "encryption.key")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeKeyFile: %v", err)
	}
	return path
}

// TestLoadEncryptionKeyFile verifies that ENCRYPTION_KEY_FILE is honoured as
// the primary key source (Docker/K8s secret mount path).
func TestLoadEncryptionKeyFile(t *testing.T) {
	baseEnv(t)
	// Clear the raw env var so only the file is active.
	t.Setenv("ENCRYPTION_KEY", "")
	path := writeKeyFile(t, validKeyHex+"\n") // trailing newline is trimmed
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with ENCRYPTION_KEY_FILE: %v", err)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("EncryptionKey len = %d, want 32", len(cfg.EncryptionKey))
	}
}

// TestLoadEncryptionKeyFileTakesPrecedence verifies that ENCRYPTION_KEY_FILE
// wins over ENCRYPTION_KEY when both are set, so operators can migrate to
// secret mounts without removing the old env var first.
func TestLoadEncryptionKeyFileTakesPrecedence(t *testing.T) {
	baseEnv(t)
	// ENCRYPTION_KEY is set to an invalid value; if Load uses the file it passes.
	t.Setenv("ENCRYPTION_KEY", "not-valid-hex-at-all")
	path := writeKeyFile(t, validKeyHex)
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("ENCRYPTION_KEY_FILE should take precedence, but got: %v", err)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("EncryptionKey len = %d, want 32", len(cfg.EncryptionKey))
	}
}

// TestLoadEncryptionKeyFileMissing verifies a clear error when the file path
// is set but the file does not exist.
func TestLoadEncryptionKeyFileMissing(t *testing.T) {
	baseEnv(t)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_FILE", "/nonexistent/path/encryption.key")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing ENCRYPTION_KEY_FILE, got nil")
	}
	if !strings.Contains(err.Error(), "ENCRYPTION_KEY_FILE") {
		t.Errorf("error should mention ENCRYPTION_KEY_FILE, got: %v", err)
	}
}

// TestLoadEncryptionKeyFileEmpty verifies a clear error when the file exists
// but contains only whitespace.
func TestLoadEncryptionKeyFileEmpty(t *testing.T) {
	baseEnv(t)
	t.Setenv("ENCRYPTION_KEY", "")
	path := writeKeyFile(t, "   \n")
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for empty ENCRYPTION_KEY_FILE, got nil")
	}
	if !strings.Contains(err.Error(), "ENCRYPTION_KEY_FILE") {
		t.Errorf("error should mention ENCRYPTION_KEY_FILE, got: %v", err)
	}
}

// TestLoadEncryptionKeyFileBadHex verifies that a file with a non-hex value
// produces a clear validation error.
func TestLoadEncryptionKeyFileBadHex(t *testing.T) {
	baseEnv(t)
	t.Setenv("ENCRYPTION_KEY", "")
	path := writeKeyFile(t, "not-valid-hex")
	t.Setenv("ENCRYPTION_KEY_FILE", path)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-hex ENCRYPTION_KEY_FILE, got nil")
	}
}

// TestLoadEncryptionKeyFallback verifies that omitting ENCRYPTION_KEY_FILE
// still works with the raw ENCRYPTION_KEY (back-compat).
func TestLoadEncryptionKeyFallback(t *testing.T) {
	baseEnv(t)
	t.Setenv("ENCRYPTION_KEY_FILE", "") // explicitly cleared

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load with raw ENCRYPTION_KEY (fallback): %v", err)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("EncryptionKey len = %d, want 32", len(cfg.EncryptionKey))
	}
}

// TestLoadNoEncryptionKeySource verifies that Load fails when neither
// ENCRYPTION_KEY nor ENCRYPTION_KEY_FILE is provided.
func TestLoadNoEncryptionKeySource(t *testing.T) {
	baseEnv(t)
	t.Setenv("ENCRYPTION_KEY", "")
	t.Setenv("ENCRYPTION_KEY_FILE", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when no ENCRYPTION_KEY source, got nil")
	}
	if !strings.Contains(err.Error(), "ENCRYPTION_KEY") {
		t.Errorf("error should mention ENCRYPTION_KEY, got: %v", err)
	}
}
