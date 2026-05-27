package chatbot

import (
	"context"
	"log/slog"
	"time"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
)

// Service owns the chat-bot config and (when enabled + configured) the inbound
// poll loop. It is read-only: commands only query inventory.
type Service struct {
	store   db.Store
	cipher  *crypto.Cipher
	logger  *slog.Logger
	enabled func(context.Context) bool // feature-flag gate
}

// New wires the service. enabled reports whether the chat_integration feature
// flag is on (the poller is a no-op when it's off).
func New(store db.Store, cipher *crypto.Cipher, logger *slog.Logger, enabled func(context.Context) bool) *Service {
	return &Service{store: store, cipher: cipher, logger: logger, enabled: enabled}
}

// ConfigView is the non-secret config for the UI.
type ConfigView struct {
	Provider     string  `json:"provider"`
	HasToken     bool    `json:"has_token"`
	AllowedChats []int64 `json:"allowed_chats"`
}

// Config returns the non-secret chat config.
func (s *Service) Config(ctx context.Context) ConfigView {
	cfg, _ := s.store.GetChatConfig(ctx)
	v := ConfigView{Provider: cfg.Provider, AllowedChats: cfg.AllowedChats, HasToken: len(cfg.TokenEncrypted) > 0}
	if v.Provider == "" {
		v.Provider = "telegram"
	}
	if v.AllowedChats == nil {
		v.AllowedChats = []int64{}
	}
	return v
}

// SetConfig stores the chat config. token: nil keeps, "" clears, value seals.
func (s *Service) SetConfig(ctx context.Context, allowedChats []int64, token *string) error {
	existing, _ := s.store.GetChatConfig(ctx)
	cfg := db.ChatConfig{Provider: "telegram", TokenEncrypted: existing.TokenEncrypted, AllowedChats: allowedChats}
	if token != nil {
		if *token == "" {
			cfg.TokenEncrypted = nil
		} else {
			sealed, err := s.cipher.Seal([]byte(*token))
			if err != nil {
				return err
			}
			cfg.TokenEncrypted = sealed
		}
	}
	return s.store.UpsertChatConfig(ctx, cfg)
}

// token decrypts the stored bot token, or "" if unset/unreadable.
func (s *Service) token(ctx context.Context) string {
	cfg, err := s.store.GetChatConfig(ctx)
	if err != nil || len(cfg.TokenEncrypted) == 0 {
		return ""
	}
	if pt, derr := s.cipher.Open(cfg.TokenEncrypted); derr == nil {
		return string(pt)
	}
	return ""
}

// authorized reports whether a chat ID is on the allow-list.
func (s *Service) authorized(ctx context.Context, chatID int64) bool {
	cfg, err := s.store.GetChatConfig(ctx)
	if err != nil {
		return false
	}
	for _, id := range cfg.AllowedChats {
		if id == chatID {
			return true
		}
	}
	return false
}

// Run polls Telegram for commands while the feature is enabled + configured. It
// re-reads config each cycle so toggling the flag or token takes effect without
// a restart. Intended to run in a goroutine; returns when ctx is cancelled.
func (s *Service) Run(ctx context.Context) {
	var client Client
	var activeToken string
	offset := 0
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if s.enabled != nil && !s.enabled(ctx) {
			client = nil
			continue
		}
		tok := s.token(ctx)
		if tok == "" {
			client = nil
			continue
		}
		if client == nil || tok != activeToken {
			client = NewTelegram(tok)
			activeToken = tok
			offset = 0
		}
		offset = s.pollOnce(ctx, client, offset)
	}
}

// pollOnce fetches updates and replies to authorized command messages. Returns
// the next offset.
func (s *Service) pollOnce(ctx context.Context, client Client, offset int) int {
	updates, err := client.GetUpdates(ctx, offset)
	if err != nil {
		return offset
	}
	for _, u := range updates {
		if u.UpdateID >= offset {
			offset = u.UpdateID + 1
		}
		if u.Message == nil || u.Message.Text == "" {
			continue
		}
		chatID := u.Message.Chat.ID
		if !s.authorized(ctx, chatID) {
			_ = client.SendMessage(ctx, chatID, "This chat isn't authorized. Add its chat ID in Stratum → Settings → Chat.")
			continue
		}
		reply := Handle(ctx, &storeProvider{store: s.store}, u.Message.Text)
		_ = client.SendMessage(ctx, chatID, reply)
	}
	return offset
}

// storeProvider adapts db.Store to the command DataProvider.
type storeProvider struct{ store db.Store }

func (p *storeProvider) Nodes(ctx context.Context) ([]NodeBrief, error) {
	nodes, err := p.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]NodeBrief, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, NodeBrief{Name: n.Name, Type: n.Type, Status: n.Status})
	}
	return out, nil
}

func (p *storeProvider) Containers(ctx context.Context) ([]ContainerBrief, error) {
	nodes, err := p.store.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	var out []ContainerBrief
	for _, n := range nodes {
		cs, err := p.store.ListContainersByNode(ctx, n.ID)
		if err != nil {
			continue
		}
		for _, c := range cs {
			out = append(out, ContainerBrief{Name: c.Name, Image: c.Image, Status: c.Status, NodeName: n.Name})
		}
	}
	return out, nil
}
