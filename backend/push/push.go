// Package push implements Web Push (RFC 8030) notifications for Stratum.
// It manages VAPID keypair lifecycle and dispatches push messages to stored
// browser subscriptions. All sends are fire-and-forget; expired endpoints are
// silently removed (410 Gone).
package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"

	"github.com/kaylaehman/stratum/backend/db"
)

// Store is the narrow persistence interface required by this package.
type Store interface {
	GetVAPID(ctx context.Context) (db.PushVAPID, error)
	UpsertVAPID(ctx context.Context, v db.PushVAPID) error
	ListPushSubscriptions(ctx context.Context) ([]db.PushSubscription, error)
	ListPushSubscriptionsByUser(ctx context.Context, userID string) ([]db.PushSubscription, error)
	UpsertPushSubscription(ctx context.Context, sub db.PushSubscription) error
	DeletePushSubscription(ctx context.Context, endpoint string) error
	DeletePushSubscriptionByUser(ctx context.Context, userID, endpoint string) error
}

// allowedPushHostSuffixes restricts the subscription endpoint to the real
// browser push services. The endpoint is a URL the SERVER later POSTs to, so an
// unvalidated value is an SSRF vector (point it at an internal host). A strict
// host allowlist closes that — only the known FCM/Mozilla/Windows/Apple push
// gateways are accepted, which inherently excludes internal IPs/hostnames.
var allowedPushHostSuffixes = []string{
	"fcm.googleapis.com",          // Chrome / Chromium (exact host)
	".push.services.mozilla.com",  // Firefox
	".notify.windows.com",         // Edge / Windows (WNS)
	".push.apple.com",             // Safari / Apple (incl. web.push.apple.com)
}

// validateEndpoint rejects any subscription endpoint that is not an https URL on
// a recognized web-push service host (anti-SSRF).
func validateEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("push: invalid endpoint url: %w", err)
	}
	if u.Scheme != "https" {
		return errors.New("push: endpoint must be https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return errors.New("push: endpoint missing host")
	}
	for _, suf := range allowedPushHostSuffixes {
		if strings.HasPrefix(suf, ".") {
			if host == strings.TrimPrefix(suf, ".") || strings.HasSuffix(host, suf) {
				return nil
			}
		} else if host == suf {
			return nil
		}
	}
	return fmt.Errorf("push: endpoint host %q is not a recognized web-push service", host)
}

// Subscription is the JSON shape POSTed by the browser (Web Push subscription object).
type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// Payload is the JSON body sent in each push notification.
type Payload struct {
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	Icon    string   `json:"icon,omitempty"`
	Tag     string   `json:"tag,omitempty"`
	URL     string   `json:"url,omitempty"`
	Actions []Action `json:"actions,omitempty"`
	// ResourceID and ResourceType are included so the SW quick-action handler
	// can call the correct REST endpoint without a DB lookup.
	ResourceID   string `json:"resource_id,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

// Action mirrors the Web Notification action schema.
type Action struct {
	Action string `json:"action"`
	Title  string `json:"title"`
}

// Service manages VAPID keys and push dispatch.
type Service struct {
	store      Store
	logger     *slog.Logger
	subject    string // mailto: or https: VAPID subject
	privateKey string // cached after first init
	publicKey  string // cached after first init
}

// New creates a Service, loading or generating the VAPID keypair.
func New(ctx context.Context, store Store, subject string, logger *slog.Logger) (*Service, error) {
	s := &Service{store: store, logger: logger, subject: subject}
	if err := s.initVAPID(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// PublicKey returns the URL-safe base64 VAPID public key for the frontend.
func (s *Service) PublicKey() string { return s.publicKey }

// Subscribe saves or replaces a push subscription for a user.
func (s *Service) Subscribe(ctx context.Context, userID string, sub Subscription) error {
	if sub.Endpoint == "" || sub.Keys.P256DH == "" || sub.Keys.Auth == "" {
		return errors.New("push: missing required subscription fields")
	}
	if err := validateEndpoint(sub.Endpoint); err != nil {
		return err
	}
	return s.store.UpsertPushSubscription(ctx, db.PushSubscription{
		ID:        uuid.NewString(),
		UserID:    userID,
		Endpoint:  sub.Endpoint,
		P256DH:    sub.Keys.P256DH,
		Auth:      sub.Keys.Auth,
		CreatedAt: time.Now(),
	})
}

// Unsubscribe removes a push subscription, scoped to the owning user so one user
// cannot delete another user's subscription by guessing its endpoint (IDOR).
func (s *Service) Unsubscribe(ctx context.Context, userID, endpoint string) error {
	if endpoint == "" {
		return errors.New("push: endpoint required")
	}
	return s.store.DeletePushSubscriptionByUser(ctx, userID, endpoint)
}

// SendToAll dispatches a push to every stored subscription.
// Expired endpoints (HTTP 410) are removed silently.
func (s *Service) SendToAll(ctx context.Context, p Payload) error {
	subs, err := s.store.ListPushSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("push: list subscriptions: %w", err)
	}
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("push: marshal payload: %w", err)
	}
	for _, sub := range subs {
		s.sendOne(ctx, sub, body)
	}
	return nil
}

// SendToUser dispatches a push to all subscriptions for a single user.
func (s *Service) SendToUser(ctx context.Context, userID string, p Payload) error {
	subs, err := s.store.ListPushSubscriptionsByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("push: list user subscriptions: %w", err)
	}
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("push: marshal payload: %w", err)
	}
	for _, sub := range subs {
		s.sendOne(ctx, sub, body)
	}
	return nil
}

func (s *Service) sendOne(ctx context.Context, sub db.PushSubscription, body []byte) {
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256DH,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
		Subscriber:      s.subject,
		TTL:             3600,
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		s.logger.Warn("push: send failed", "endpoint", sub.Endpoint, "err", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusGone {
		// Endpoint is no longer valid — remove it.
		if delErr := s.store.DeletePushSubscription(ctx, sub.Endpoint); delErr != nil {
			s.logger.Warn("push: remove expired subscription", "err", delErr)
		}
	} else if resp.StatusCode >= 400 {
		s.logger.Warn("push: unexpected status", "status", resp.StatusCode, "endpoint", sub.Endpoint)
	}
}

// initVAPID loads the VAPID keypair from storage, generating one if absent.
func (s *Service) initVAPID(ctx context.Context) error {
	v, err := s.store.GetVAPID(ctx)
	if err == nil {
		s.privateKey = v.PrivateKey
		s.publicKey = v.PublicKey
		return nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("push: load vapid: %w", err)
	}
	// Generate a fresh VAPID keypair.
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return fmt.Errorf("push: generate vapid keys: %w", err)
	}
	if err := s.store.UpsertVAPID(ctx, db.PushVAPID{PrivateKey: priv, PublicKey: pub}); err != nil {
		return fmt.Errorf("push: persist vapid: %w", err)
	}
	s.privateKey = priv
	s.publicKey = pub
	s.logger.Info("push: generated new VAPID keypair")
	return nil
}
