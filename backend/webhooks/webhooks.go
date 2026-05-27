// Package webhooks dispatches notification messages to Slack/Discord webhooks
// (Feature 26). It formats a provider-appropriate JSON payload, rate-limits per
// (webhook, trigger) pair, and fans out to every enabled webhook subscribed to
// a trigger. Event sources call Notify; the settings UI calls Test.
package webhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
)

// Trigger keys. Event sources reference these; webhooks subscribe to them.
const (
	TriggerPortNew         = "port.new"
	TriggerContainerCrash  = "container.crash"
	TriggerCVECritical     = "cve.critical"
	TriggerSSHKeyAdded     = "sshkey.added"
	TriggerFileChange      = "file.change"
	TriggerAgentDisconnect = "agent.disconnect"
	TriggerCPUThreshold    = "cpu.threshold"
)

// AllTriggers is the catalog the settings UI offers.
var AllTriggers = []string{
	TriggerPortNew, TriggerContainerCrash, TriggerCVECritical, TriggerSSHKeyAdded,
	TriggerFileChange, TriggerAgentDisconnect, TriggerCPUThreshold,
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
}

// New builds a Dispatcher.
func New(store db.Store) *Dispatcher {
	return &Dispatcher{
		store:    store,
		client:   &http.Client{Timeout: 10 * time.Second},
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
