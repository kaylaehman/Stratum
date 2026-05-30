package chatbot

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kaylaehman/stratum/backend/ai"
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
	aiSvc   *ai.Service                // optional; nil → "configure AI in Settings"
}

// New wires the service. enabled reports whether the chat_integration feature
// flag is on (the poller is a no-op when it's off). aiSvc may be nil when AI
// is not wired; the bot will respond with a friendly prompt to configure it.
func New(store db.Store, cipher *crypto.Cipher, logger *slog.Logger, enabled func(context.Context) bool, aiSvc *ai.Service) *Service {
	return &Service{store: store, cipher: cipher, logger: logger, enabled: enabled, aiSvc: aiSvc}
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
		s.dispatch(ctx, client, chatID, u.Message.Text)
	}
	return offset
}

// dispatch routes a single authorized message: slash commands go to Handle;
// free-text (and /ask <question>) go to the AI service.
func (s *Service) dispatch(ctx context.Context, client Client, chatID int64, text string) {
	if aiQuestion, isAI := extractAIQuestion(text); isAI {
		s.replyAI(ctx, client, chatID, aiQuestion)
		return
	}
	reply := Handle(ctx, &storeProvider{store: s.store}, text)
	_ = client.SendMessage(ctx, chatID, reply)
}

// extractAIQuestion returns (question, true) when the message should be routed
// to AI: either it is free-text (doesn't start with "/") or it is the /ask
// command. For all other slash commands it returns ("", false).
func extractAIQuestion(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return "", false
	}
	if !strings.HasPrefix(trimmed, "/") {
		return trimmed, true
	}
	// /ask[@botname] <question>
	lower := strings.ToLower(trimmed)
	cmd := strings.Fields(lower)[0]
	// strip optional @botname suffix
	if i := strings.Index(cmd, "@"); i >= 0 {
		cmd = cmd[:i]
	}
	if cmd == "/ask" {
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return "", false // /ask with no question → fall through to Handle
		}
		return strings.Join(fields[1:], " "), true
	}
	return "", false
}

// replyAI calls the AI service and sends one or more messages (Telegram caps at
// 4096 chars per message). When AI is unconfigured or returns an error the user
// gets a friendly plain-text reply — the bot loop never crashes.
func (s *Service) replyAI(ctx context.Context, client Client, chatID int64, question string) {
	if s.aiSvc == nil {
		_ = client.SendMessage(ctx, chatID, "AI is not configured. Open Stratum → Settings → AI Assistant to set up a provider.")
		return
	}
	resp, _, err := s.aiSvc.Ask(ctx, "", question, "")
	if err != nil {
		if errors.Is(err, ai.ErrNotConfigured) {
			_ = client.SendMessage(ctx, chatID, "AI is not configured. Open Stratum → Settings → AI Assistant to set up a provider.")
			return
		}
		s.logger.Warn("chatbot: AI ask failed", "err", err)
		_ = client.SendMessage(ctx, chatID, "Sorry, the AI provider returned an error. Please try again later.")
		return
	}
	for _, chunk := range splitMessage(resp.Answer) {
		_ = client.SendMessage(ctx, chatID, chunk)
	}
}

// telegramMaxLen is Telegram's documented per-message character limit.
const telegramMaxLen = 4096

// splitMessage splits text into chunks of at most telegramMaxLen runes,
// breaking on newlines where possible to keep chunks readable.
func splitMessage(text string) []string {
	if text == "" {
		return []string{"(no response)"}
	}
	if len([]rune(text)) <= telegramMaxLen {
		return []string{text}
	}
	var chunks []string
	for len(text) > 0 {
		if len([]rune(text)) <= telegramMaxLen {
			chunks = append(chunks, text)
			break
		}
		// Find a newline near the limit to break cleanly.
		cut := runeIndex(text, telegramMaxLen)
		if nl := strings.LastIndex(text[:cut], "\n"); nl > telegramMaxLen/2 {
			cut = nl + 1
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}
	return chunks
}

// runeIndex returns the byte index after the n-th rune (or len(s) if shorter).
func runeIndex(s string, n int) int {
	i := 0
	for count := 0; count < n && i < len(s); count++ {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return i
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
