// Package chatbot implements inbound chat commands (Feature F8). The outbound
// alert path already exists (notification webhooks); this adds a read-only
// Telegram command bot. Mutating commands (restart, etc.) require the F7-style
// approval flow and are a deliberate follow-on — this bot answers status/inventory
// queries only and authorizes by a configured allow-list of chat IDs.
package chatbot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Update is one inbound Telegram update (subset).
type Update struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			Username string `json:"username"`
		} `json:"from"`
	} `json:"message"`
}

// Client is the subset of the Telegram Bot API the poller needs (an interface so
// the service is testable without network).
type Client interface {
	GetUpdates(ctx context.Context, offset int) ([]Update, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// httpClient talks to api.telegram.org with a bot token.
type httpClient struct {
	token string
	http  *http.Client
}

// NewTelegram builds a real Telegram client.
func NewTelegram(token string) Client {
	return &httpClient{token: token, http: &http.Client{Timeout: 65 * time.Second}}
}

func (c *httpClient) base() string { return "https://api.telegram.org/bot" + c.token }

func (c *httpClient) GetUpdates(ctx context.Context, offset int) ([]Update, error) {
	u := fmt.Sprintf("%s/getUpdates?timeout=50&offset=%d", c.base(), offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram: getUpdates status %d", resp.StatusCode)
	}
	var out struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c *httpClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	form := url.Values{}
	form.Set("chat_id", strconv.FormatInt(chatID, 10))
	form.Set("text", text)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/sendMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: sendMessage status %d", resp.StatusCode)
	}
	return nil
}
