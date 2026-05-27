package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
)

// httpTimeout bounds a single provider call (the API handler also applies a
// request-scoped timeout).
const httpTimeout = 90 * time.Second

// Service resolves the configured provider and answers questions.
type Service struct {
	store  db.Store
	cipher *crypto.Cipher
	http   *http.Client

	// Environment defaults (CLAUDE env spec). Used only when the stored config
	// doesn't supply the corresponding value.
	envClaudeKey  string
	envOllamaURL  string
}

// New wires the service. envClaudeKey/envOllamaURL are optional defaults from
// the process environment.
func New(store db.Store, cipher *crypto.Cipher, envClaudeKey, envOllamaURL string) *Service {
	return &Service{
		store:        store,
		cipher:       cipher,
		http:         &http.Client{Timeout: httpTimeout},
		envClaudeKey: envClaudeKey,
		envOllamaURL: strings.TrimSpace(envOllamaURL),
	}
}

// ConfigView is the non-secret config returned to the UI.
type ConfigView struct {
	Provider      string `json:"provider"`
	OllamaBaseURL string `json:"ollama_base_url"`
	OllamaModel   string `json:"ollama_model"`
	ClaudeModel   string `json:"claude_model"`
	HasAPIKey     bool   `json:"has_api_key"`
	Configured    bool   `json:"configured"`
}

// ConfigUpdate is an admin's settings change. APIKey is optional: when nil the
// stored key is left unchanged; when a non-nil empty string, the key is cleared.
type ConfigUpdate struct {
	Provider      string  `json:"provider"`
	OllamaBaseURL string  `json:"ollama_base_url"`
	OllamaModel   string  `json:"ollama_model"`
	ClaudeModel   string  `json:"claude_model"`
	APIKey        *string `json:"api_key"`
}

// Config returns the current non-secret configuration view.
func (s *Service) Config(ctx context.Context) ConfigView {
	cfg, _ := s.store.GetAIConfig(ctx)
	v := ConfigView{
		Provider:      cfg.Provider,
		OllamaBaseURL: cfg.OllamaBaseURL,
		OllamaModel:   cfg.OllamaModel,
		ClaudeModel:   cfg.ClaudeModel,
		HasAPIKey:     len(cfg.APIKeyEncrypted) > 0 || s.envClaudeKey != "",
	}
	if v.OllamaBaseURL == "" {
		v.OllamaBaseURL = s.envOllamaURL
	}
	_, err := s.providerFrom(cfg)
	v.Configured = err == nil
	return v
}

// ErrInvalidConfig is returned for a malformed settings update.
var ErrInvalidConfig = errors.New("ai: invalid configuration")

// SetConfig validates and persists an admin settings change. The API key, if
// supplied, is sealed before storage.
func (s *Service) SetConfig(ctx context.Context, u ConfigUpdate) error {
	switch u.Provider {
	case ProviderOllama, ProviderClaude, "":
	default:
		return fmt.Errorf("%w: unknown provider %q", ErrInvalidConfig, u.Provider)
	}
	if u.OllamaBaseURL != "" {
		if err := validateHTTPURL(u.OllamaBaseURL); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
	}

	existing, _ := s.store.GetAIConfig(ctx)
	cfg := db.AIConfig{
		Provider:        u.Provider,
		OllamaBaseURL:   u.OllamaBaseURL,
		OllamaModel:     u.OllamaModel,
		ClaudeModel:     u.ClaudeModel,
		APIKeyEncrypted: existing.APIKeyEncrypted, // preserved unless changed below
	}
	if u.APIKey != nil {
		if *u.APIKey == "" {
			cfg.APIKeyEncrypted = nil // explicit clear
		} else {
			sealed, err := s.cipher.Seal([]byte(*u.APIKey))
			if err != nil {
				return err
			}
			cfg.APIKeyEncrypted = sealed
		}
	}
	return s.store.UpsertAIConfig(ctx, cfg)
}

// Ask answers a question using the configured provider. task selects a system
// preamble; context is supplemental material (logs/inspect/config) and is
// truncated before sending. Returns ErrNotConfigured when no provider is usable.
func (s *Service) Ask(ctx context.Context, task, prompt, contextText string) (AskResponse, string, error) {
	cfg, _ := s.store.GetAIConfig(ctx)
	provider, err := s.providerFrom(cfg)
	if err != nil {
		return AskResponse{}, "", err
	}
	resp, err := provider.Ask(ctx, AskRequest{
		System: buildSystem(task, contextText),
		Prompt: prompt,
	})
	return resp, provider.Kind(), err
}

// providerFrom builds the active provider from stored config, falling back to
// environment defaults. Resolution: an explicit cfg.Provider wins; otherwise we
// infer (Claude if a key exists, else Ollama if a URL exists).
func (s *Service) providerFrom(cfg db.AIConfig) (Provider, error) {
	provider := cfg.Provider
	if provider == "" {
		switch {
		case len(cfg.APIKeyEncrypted) > 0 || s.envClaudeKey != "":
			provider = ProviderClaude
		case cfg.OllamaBaseURL != "" || s.envOllamaURL != "":
			provider = ProviderOllama
		default:
			return nil, ErrNotConfigured
		}
	}

	switch provider {
	case ProviderOllama:
		base := cfg.OllamaBaseURL
		if base == "" {
			base = s.envOllamaURL
		}
		if base == "" {
			return nil, ErrNotConfigured
		}
		if err := validateHTTPURL(base); err != nil {
			return nil, err
		}
		return NewOllama(base, cfg.OllamaModel, s.http), nil
	case ProviderClaude:
		key, err := s.claudeKey(cfg)
		if err != nil || key == "" {
			return nil, ErrNotConfigured
		}
		return NewClaude(key, cfg.ClaudeModel, s.http), nil
	default:
		return nil, ErrNotConfigured
	}
}

// claudeKey decrypts the stored key, falling back to the env key.
func (s *Service) claudeKey(cfg db.AIConfig) (string, error) {
	if len(cfg.APIKeyEncrypted) > 0 {
		pt, err := s.cipher.Open(cfg.APIKeyEncrypted)
		if err != nil {
			return "", err
		}
		return string(pt), nil
	}
	return s.envClaudeKey, nil
}

// validateHTTPURL ensures a base URL is a well-formed http(s) URL with a host.
// The Ollama endpoint is admin-configured (an operator/viewer cannot set it),
// so this is a sanity check, not an SSRF allowlist.
func validateHTTPURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("url must be http or https")
	}
	if u.Host == "" {
		return errors.New("url must include a host")
	}
	return nil
}
