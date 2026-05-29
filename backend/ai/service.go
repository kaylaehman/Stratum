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
	Provider       string `json:"provider"`
	OllamaBaseURL  string `json:"ollama_base_url"`
	OllamaModel    string `json:"ollama_model"`
	ClaudeModel    string `json:"claude_model"`
	OpenAIModel    string `json:"openai_model"`
	GeminiModel    string `json:"gemini_model"`
	HasAPIKey      bool   `json:"has_api_key"`
	OAuthConnected bool   `json:"oauth_connected"`
	Configured     bool   `json:"configured"`
}

// ConfigUpdate is an admin's settings change. APIKey is optional: when nil the
// stored key is left unchanged; when a non-nil empty string, the key is cleared.
type ConfigUpdate struct {
	Provider      string  `json:"provider"`
	OllamaBaseURL string  `json:"ollama_base_url"`
	OllamaModel   string  `json:"ollama_model"`
	ClaudeModel   string  `json:"claude_model"`
	OpenAIModel   string  `json:"openai_model"`
	GeminiModel   string  `json:"gemini_model"`
	APIKey        *string `json:"api_key"`
}

// Config returns the current non-secret configuration view.
func (s *Service) Config(ctx context.Context) ConfigView {
	cfg, _ := s.store.GetAIConfig(ctx)
	v := ConfigView{
		Provider:       cfg.Provider,
		OllamaBaseURL:  cfg.OllamaBaseURL,
		OllamaModel:    cfg.OllamaModel,
		ClaudeModel:    cfg.ClaudeModel,
		OpenAIModel:    cfg.OpenAIModel,
		GeminiModel:    cfg.GeminiModel,
		HasAPIKey:      len(cfg.APIKeyEncrypted) > 0 || s.envClaudeKey != "",
		OAuthConnected: len(cfg.OAuthAccessEncrypted) > 0,
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
	case ProviderOllama, ProviderClaude, ProviderClaudeOAuth, ProviderOpenAI, ProviderGemini, "":
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
		OpenAIModel:     u.OpenAIModel,
		GeminiModel:     u.GeminiModel,
		APIKeyEncrypted: existing.APIKeyEncrypted, // preserved unless changed below
		// OAuth tokens are managed by the sign-in flow, not this settings save —
		// always carry them over so switching provider/model doesn't drop them.
		OAuthAccessEncrypted:  existing.OAuthAccessEncrypted,
		OAuthRefreshEncrypted: existing.OAuthRefreshEncrypted,
		OAuthExpiresAt:        existing.OAuthExpiresAt,
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
	if cfg.Provider == ProviderClaudeOAuth {
		var err error
		if cfg, err = s.ensureFreshOAuth(ctx, cfg); err != nil {
			return AskResponse{}, "", err
		}
	}
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
	case ProviderClaudeOAuth:
		if len(cfg.OAuthAccessEncrypted) == 0 {
			return nil, ErrNotConfigured
		}
		tok, err := s.cipher.Open(cfg.OAuthAccessEncrypted)
		if err != nil {
			return nil, err
		}
		return NewClaudeOAuth(string(tok), cfg.ClaudeModel, s.http), nil
	case ProviderOpenAI:
		key, err := s.storedAPIKey(cfg)
		if err != nil || key == "" {
			return nil, ErrNotConfigured
		}
		return NewOpenAI(key, cfg.OpenAIModel, s.http), nil
	case ProviderGemini:
		key, err := s.storedAPIKey(cfg)
		if err != nil || key == "" {
			return nil, ErrNotConfigured
		}
		return NewGemini(key, cfg.GeminiModel, s.http), nil
	default:
		return nil, ErrNotConfigured
	}
}

// storedAPIKey decrypts the shared API key (used by the OpenAI/Gemini providers).
// Unlike claudeKey it has no environment fallback.
func (s *Service) storedAPIKey(cfg db.AIConfig) (string, error) {
	if len(cfg.APIKeyEncrypted) == 0 {
		return "", nil
	}
	pt, err := s.cipher.Open(cfg.APIKeyEncrypted)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

// --- Claude OAuth (Feature 31) ---

// OAuthStart begins a sign-in: it returns the authorize URL the operator opens
// in a browser, plus the PKCE verifier and state the caller must hand back to
// OAuthExchange. Nothing is persisted yet.
func (s *Service) OAuthStart() (authorizeURL, verifier, state string, err error) {
	p, err := GeneratePKCE()
	if err != nil {
		return "", "", "", err
	}
	state, err = GenerateState()
	if err != nil {
		return "", "", "", err
	}
	return AuthorizeURL(p.Challenge, state), p.Verifier, state, nil
}

// OAuthExchange swaps the pasted authorization code for tokens and persists them
// (sealed), switching the active provider to claude-oauth. The Claude callback
// page shows "code#state"; SplitPastedCode tolerates either form.
func (s *Service) OAuthExchange(ctx context.Context, pastedCode, verifier, state string) error {
	code, embeddedState := SplitPastedCode(pastedCode)
	if embeddedState != "" {
		state = embeddedState
	}
	if code == "" || verifier == "" {
		return fmt.Errorf("%w: missing code or verifier", ErrInvalidConfig)
	}
	ts, err := ExchangeCode(ctx, s.http, code, verifier, state, time.Now())
	if err != nil {
		return err
	}
	cfg, _ := s.store.GetAIConfig(ctx)
	cfg.Provider = ProviderClaudeOAuth
	_, err = s.persistTokens(ctx, cfg, ts)
	return err
}

// SetOAuthToken stores a manually-provided Claude OAuth token (e.g. from
// `claude setup-token`), sealing it and switching the provider to claude-oauth.
// A bare access token is treated as long-lived (zero expiry → no auto-refresh);
// an optional refresh token is stored for future use.
func (s *Service) SetOAuthToken(ctx context.Context, accessToken, refreshToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return fmt.Errorf("%w: empty access token", ErrInvalidConfig)
	}
	cfg, _ := s.store.GetAIConfig(ctx)
	cfg.Provider = ProviderClaudeOAuth
	_, err := s.persistTokens(ctx, cfg, TokenSet{
		AccessToken:  accessToken,
		RefreshToken: strings.TrimSpace(refreshToken),
		// ExpiresAt left zero: tokenExpired() treats it as non-expiring.
	})
	return err
}

// OAuthDisconnect clears the stored OAuth tokens. If claude-oauth was the active
// provider, it is reset to "none" so the assistant doesn't report half-configured.
func (s *Service) OAuthDisconnect(ctx context.Context) error {
	cfg, _ := s.store.GetAIConfig(ctx)
	cfg.OAuthAccessEncrypted = nil
	cfg.OAuthRefreshEncrypted = nil
	cfg.OAuthExpiresAt = time.Time{}
	if cfg.Provider == ProviderClaudeOAuth {
		cfg.Provider = ""
	}
	return s.store.UpsertAIConfig(ctx, cfg)
}

// ensureFreshOAuth refreshes the access token when it's near/after expiry,
// persisting the new tokens. A token with no expiry is treated as long-lived.
func (s *Service) ensureFreshOAuth(ctx context.Context, cfg db.AIConfig) (db.AIConfig, error) {
	if !tokenExpired(cfg.OAuthExpiresAt, time.Now()) {
		return cfg, nil
	}
	if len(cfg.OAuthRefreshEncrypted) == 0 {
		return cfg, ErrNotConfigured // expired and nothing to refresh with
	}
	refresh, err := s.cipher.Open(cfg.OAuthRefreshEncrypted)
	if err != nil {
		return cfg, err
	}
	ts, err := RefreshToken(ctx, s.http, string(refresh), time.Now())
	if err != nil {
		return cfg, err
	}
	return s.persistTokens(ctx, cfg, ts)
}

// persistTokens seals the token set into cfg and writes it. A refresh response
// may omit a new refresh token, in which case the existing one is kept.
func (s *Service) persistTokens(ctx context.Context, cfg db.AIConfig, ts TokenSet) (db.AIConfig, error) {
	access, err := s.cipher.Seal([]byte(ts.AccessToken))
	if err != nil {
		return cfg, err
	}
	cfg.OAuthAccessEncrypted = access
	if ts.RefreshToken != "" {
		refresh, err := s.cipher.Seal([]byte(ts.RefreshToken))
		if err != nil {
			return cfg, err
		}
		cfg.OAuthRefreshEncrypted = refresh
	}
	cfg.OAuthExpiresAt = ts.ExpiresAt
	return cfg, s.store.UpsertAIConfig(ctx, cfg)
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
	// The endpoint path ("/api/chat") is appended by the provider, so the base
	// must be host-only — a base with a path/query would smuggle a different
	// request target (e.g. http://host/x?y= -> http://host/x?y=/api/chat).
	if u.Path != "" && u.Path != "/" {
		return errors.New("url must not include a path")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return errors.New("url must not include a query or fragment")
	}
	return nil
}
