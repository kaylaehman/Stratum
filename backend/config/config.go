// Package config loads and validates Stratum backend configuration from the
// environment. It fails fast at startup when a required value is missing or
// malformed, per CLAUDE.md's env spec.
package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// minJWTSecretLen is the minimum acceptable JWT_SECRET length in bytes. This is
// a floor, not an entropy guarantee — see §5.1 of the foundation design.
const minJWTSecretLen = 32

// encryptionKeyLen is the exact decoded length of ENCRYPTION_KEY (AES-256).
const encryptionKeyLen = 32

// Config is the validated backend configuration.
type Config struct {
	Port    int
	BaseURL string

	DatabaseURL string

	JWTSecret     []byte
	EncryptionKey []byte // exactly 32 bytes, hex-decoded from ENCRYPTION_KEY

	AgentCACertPath string
	AgentCAKeyPath  string

	// Optional, unused in SP0.
	TrivyPath      string
	AnthropicKey   string
	OllamaBaseURL  string

	// Optional first-run admin seed (CI/automation escape hatch). Empty unless set.
	AdminUser     string
	AdminPassword string
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	cfg := &Config{
		BaseURL:         os.Getenv("BASE_URL"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		AgentCACertPath: os.Getenv("AGENT_CA_CERT_PATH"),
		AgentCAKeyPath:  os.Getenv("AGENT_CA_KEY_PATH"),
		TrivyPath:       os.Getenv("TRIVY_PATH"),
		AnthropicKey:    os.Getenv("ANTHROPIC_API_KEY"),
		OllamaBaseURL:   os.Getenv("OLLAMA_BASE_URL"),
		AdminUser:       os.Getenv("STRATUM_ADMIN_USER"),
		AdminPassword:   os.Getenv("STRATUM_ADMIN_PASSWORD"),
	}

	// PORT (default 8080).
	portStr := os.Getenv("PORT")
	if portStr == "" {
		cfg.Port = 8080
	} else {
		p, err := strconv.Atoi(portStr)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("config: invalid PORT %q", portStr)
		}
		cfg.Port = p
	}

	if cfg.DatabaseURL == "" {
		return nil, errors.New("config: DATABASE_URL is required")
	}

	// JWT_SECRET (required, >= 32 bytes).
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < minJWTSecretLen {
		return nil, fmt.Errorf("config: JWT_SECRET must be at least %d bytes (generate via 'openssl rand -base64 32')", minJWTSecretLen)
	}
	cfg.JWTSecret = []byte(secret)

	// ENCRYPTION_KEY (required, exactly 32 bytes hex).
	keyHex := strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))
	if keyHex == "" {
		return nil, errors.New("config: ENCRYPTION_KEY is required (32 bytes hex)")
	}
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("config: ENCRYPTION_KEY must be valid hex: %w", err)
	}
	if len(key) != encryptionKeyLen {
		return nil, fmt.Errorf("config: ENCRYPTION_KEY must decode to exactly %d bytes, got %d", encryptionKeyLen, len(key))
	}
	cfg.EncryptionKey = key

	return cfg, nil
}
