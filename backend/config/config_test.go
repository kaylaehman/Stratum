package config

import (
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

func TestLoadInvalidPort(t *testing.T) {
	baseEnv(t)
	t.Setenv("PORT", "70000")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "PORT") {
		t.Errorf("expected PORT error, got %v", err)
	}
}
