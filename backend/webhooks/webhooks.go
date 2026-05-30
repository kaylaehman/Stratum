// Package webhooks dispatches notification messages to Slack/Discord webhooks
// (Feature 26). It formats a provider-appropriate JSON payload, rate-limits per
// (webhook, trigger) pair, and fans out to every enabled webhook subscribed to
// a trigger. Event sources call Notify; the settings UI calls Test.
//
// Trigger definitions live in triggers.go and registry.go. External packages
// extend the trigger set by calling Register (see registry.go for the API).
// The ListWebhooks handler reads Registered() so every registered trigger
// surfaces in the settings UI automatically.
package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
)

// ErrInvalidURL is returned when a webhook URL fails SSRF validation.
var ErrInvalidURL = errors.New("webhooks: url must be an https provider webhook endpoint")

// allowedHosts pins each provider to its real webhook host. Because these are
// specifically Slack/Discord incoming webhooks, the URL must target the
// provider's known host — this allowlist removes the SSRF surface entirely (no
// pointing the server at metadata endpoints, loopback, or RFC1918 hosts), which
// is more robust than DNS/IP filtering (no rebinding TOCTOU).
var allowedHosts = map[string][]string{
	"slack":   {"hooks.slack.com"},
	"discord": {"discord.com", "discordapp.com"},
}

// ValidateURL enforces https + a provider host allowlist on a webhook URL.
func ValidateURL(provider, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return ErrInvalidURL
	}
	hosts, ok := allowedHosts[provider]
	if !ok {
		return ErrInvalidURL
	}
	for _, h := range hosts {
		if u.Hostname() == h {
			return nil
		}
	}
	return ErrInvalidURL
}

// rateWindow caps each (webhook,trigger) to one message per window.
const rateWindow = 5 * time.Minute

// Message is a provider-agnostic notification.
type Message struct {
	Title string
	Text  string
}

// Dispatcher sends messages to configured webhooks.
type Dispatcher struct {
	store  db.Store
	client *http.Client

	mu       sync.Mutex
	lastSent map[string]time.Time // key: webhookID + "|" + trigger

	// skipValidate disables the post-time SSRF re-validation. Test-only (set via
	// the export_test seam) so delivery can be exercised against a local server.
	skipValidate bool
}

// New builds a Dispatcher. The HTTP client refuses redirects so a 3xx from an
// allowlisted host can't bounce the request to an internal target.
func New(store db.Store) *Dispatcher {
	return &Dispatcher{
		store: store,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		lastSent: map[string]time.Time{},
	}
}

// Notify sends msg to every enabled webhook subscribed to trigger, honoring the
// per-(webhook,trigger) rate limit. Best-effort: errors are swallowed (callers
// are event sources that must not fail on a webhook outage).
func (d *Dispatcher) Notify(ctx context.Context, trigger string, msg Message) {
	hooks, err := d.store.ListWebhooks(ctx)
	if err != nil {
		return
	}
	for _, h := range hooks {
		if !h.Enabled || !contains(h.Triggers, trigger) {
			continue
		}
		if !d.allow(h.ID, trigger) {
			continue
		}
		_ = d.post(ctx, h, msg)
	}
}

// Test sends msg to one webhook immediately, bypassing the rate limit, and
// returns any delivery error (so the UI can show success/failure).
func (d *Dispatcher) Test(ctx context.Context, h db.WebhookConfig, msg Message) error {
	return d.post(ctx, h, msg)
}

// allow returns true if this (webhook,trigger) hasn't fired within the window,
// recording the send time when it returns true.
func (d *Dispatcher) allow(webhookID, trigger string) bool {
	key := webhookID + "|" + trigger
	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := d.lastSent[key]; ok && time.Since(last) < rateWindow {
		return false
	}
	d.lastSent[key] = time.Now()
	return true
}

func (d *Dispatcher) post(ctx context.Context, h db.WebhookConfig, msg Message) error {
	// Defense in depth: re-validate at send time so a row that somehow bypassed
	// config-time validation can never drive a server-side request to an
	// arbitrary host.
	if !d.skipValidate {
		if err := ValidateURL(h.Provider, h.URL); err != nil {
			return err
		}
	}
	body, err := payloadFor(h.Provider, msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhooks: %s returned %d", h.Provider, resp.StatusCode)
	}
	return nil
}

// payloadFor renders the provider-specific JSON body. Slack uses {"text":...};
// Discord uses {"content":...}. Both get a bold title + text.
func payloadFor(provider string, msg Message) ([]byte, error) {
	text := msg.Text
	if msg.Title != "" {
		text = "*" + msg.Title + "*\n" + msg.Text
	}
	switch provider {
	case "discord":
		// Discord uses **bold**; swap the single-asterisk title marker.
		dtext := msg.Text
		if msg.Title != "" {
			dtext = "**" + msg.Title + "**\n" + msg.Text
		}
		return json.Marshal(map[string]string{"content": dtext})
	default: // slack
		return json.Marshal(map[string]string{"text": text})
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// ValidProvider reports whether p is a supported provider.
func ValidProvider(p string) bool { return p == "slack" || p == "discord" }
