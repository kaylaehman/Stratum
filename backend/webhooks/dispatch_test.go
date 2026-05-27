package webhooks_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/webhooks"
)

// fakeStore returns a fixed webhook list for Notify fan-out tests.
type fakeStore struct {
	db.Store
	hooks []db.WebhookConfig
}

func (f *fakeStore) ListWebhooks(context.Context) ([]db.WebhookConfig, error) { return f.hooks, nil }

func TestNotifyFanOutAndFilter(t *testing.T) {
	var mu sync.Mutex
	var received []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(b))
		mu.Unlock()
	}))
	defer srv.Close()

	store := &fakeStore{hooks: []db.WebhookConfig{
		{ID: "a", Name: "subscribed", URL: srv.URL, Provider: "slack", Triggers: []string{"port.new"}, Enabled: true},
		{ID: "b", Name: "wrong-trigger", URL: srv.URL, Provider: "slack", Triggers: []string{"container.crash"}, Enabled: true},
		{ID: "c", Name: "disabled", URL: srv.URL, Provider: "slack", Triggers: []string{"port.new"}, Enabled: false},
	}}
	d := webhooks.New(store)
	d.DisableURLValidationForTest() // deliver to the local httptest server
	d.Notify(context.Background(), "port.new", webhooks.Message{Title: "New port", Text: "x"})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 delivery (only the subscribed+enabled hook), got %d", len(received))
	}
	if !strings.Contains(received[0], "New port") {
		t.Errorf("payload missing title: %s", received[0])
	}
}

func TestTestDeliversAndReportsError(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()

	d := webhooks.New(&fakeStore{})
	d.DisableURLValidationForTest() // deliver to local httptest servers
	if err := d.Test(context.Background(), db.WebhookConfig{URL: ok.URL, Provider: "slack"}, webhooks.Message{Text: "hi"}); err != nil {
		t.Errorf("test to healthy endpoint should succeed: %v", err)
	}
	if err := d.Test(context.Background(), db.WebhookConfig{URL: bad.URL, Provider: "slack"}, webhooks.Message{Text: "hi"}); err == nil {
		t.Error("test to 500 endpoint should return an error")
	}
}
