package chatbot

import (
	"context"
	"strings"
	"testing"

	"github.com/KAE-Labs/stratum/backend/db"
)

// fakeStore supplies just the methods the poller touches.
type fakeStore struct {
	db.Store
	cfg db.ChatConfig
}

func (f *fakeStore) GetChatConfig(context.Context) (db.ChatConfig, error)        { return f.cfg, nil }
func (f *fakeStore) ListNodes(context.Context) ([]db.Node, error)               { return nil, nil }
func (f *fakeStore) ListContainersByNode(context.Context, string) ([]db.Container, error) {
	return nil, nil
}

// fakeClient records outbound messages and serves canned updates once.
type fakeClient struct {
	updates []Update
	served  bool
	sent    map[int64][]string
}

func (c *fakeClient) GetUpdates(context.Context, int) ([]Update, error) {
	if c.served {
		return nil, nil
	}
	c.served = true
	return c.updates, nil
}
func (c *fakeClient) SendMessage(_ context.Context, chatID int64, text string) error {
	if c.sent == nil {
		c.sent = map[int64][]string{}
	}
	c.sent[chatID] = append(c.sent[chatID], text)
	return nil
}

func msg(updateID int, chatID int64, text string) Update {
	u := Update{UpdateID: updateID}
	u.Message = &struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			Username string `json:"username"`
		} `json:"from"`
	}{}
	u.Message.Text = text
	u.Message.Chat.ID = chatID
	return u
}

// fakeAI is a stub *ai.Service replacement — we test via dispatch directly
// rather than constructing a real ai.Service (which needs a DB).
// We test the nil-aiSvc path (no provider configured) via dispatch.
func TestDispatchFreeTextNoAI(t *testing.T) {
	s := &Service{
		store:  &fakeStore{cfg: db.ChatConfig{AllowedChats: []int64{1}}},
		logger: nil, // logger is only used by replyAI on error; nil is safe here
		aiSvc:  nil, // no AI configured
	}
	client := &fakeClient{}
	s.dispatch(context.Background(), client, 1, "what is docker?")
	if len(client.sent[1]) != 1 {
		t.Fatalf("expected 1 reply, got %v", client.sent[1])
	}
	if !strings.Contains(client.sent[1][0], "not configured") {
		t.Errorf("expected 'not configured' message, got %q", client.sent[1][0])
	}
}

func TestDispatchAskCommandNoAI(t *testing.T) {
	s := &Service{
		store:  &fakeStore{cfg: db.ChatConfig{AllowedChats: []int64{1}}},
		logger: nil,
		aiSvc:  nil,
	}
	client := &fakeClient{}
	s.dispatch(context.Background(), client, 1, "/ask what is nginx?")
	if len(client.sent[1]) != 1 {
		t.Fatalf("expected 1 reply, got %v", client.sent[1])
	}
	if !strings.Contains(client.sent[1][0], "not configured") {
		t.Errorf("expected 'not configured' message, got %q", client.sent[1][0])
	}
}

func TestDispatchAskNoQuestion(t *testing.T) {
	s := &Service{
		store:  &fakeStore{cfg: db.ChatConfig{AllowedChats: []int64{1}}},
		logger: nil,
		aiSvc:  nil,
	}
	client := &fakeClient{}
	// /ask with no question falls through to Handle which returns usage hint.
	s.dispatch(context.Background(), client, 1, "/ask")
	if len(client.sent[1]) != 1 {
		t.Fatalf("expected 1 reply, got %v", client.sent[1])
	}
	if !strings.Contains(client.sent[1][0], "Usage") {
		t.Errorf("expected usage hint, got %q", client.sent[1][0])
	}
}

func TestPollOnceAuthorization(t *testing.T) {
	s := &Service{store: &fakeStore{cfg: db.ChatConfig{AllowedChats: []int64{100}}}}
	client := &fakeClient{updates: []Update{
		msg(1, 100, "/nodes"),  // authorized
		msg(2, 999, "/nodes"),  // NOT authorized
	}}
	next := s.pollOnce(context.Background(), client, 0)
	if next != 3 {
		t.Errorf("next offset = %d, want 3", next)
	}
	if len(client.sent[100]) != 1 || strings.Contains(client.sent[100][0], "isn't authorized") {
		t.Errorf("authorized chat reply = %v", client.sent[100])
	}
	if len(client.sent[999]) != 1 || !strings.Contains(client.sent[999][0], "isn't authorized") {
		t.Errorf("unauthorized chat should be rejected, got %v", client.sent[999])
	}
}
