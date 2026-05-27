package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/api"
	"github.com/kaylaehman/stratum/backend/auth"
	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/server"

	"log/slog"
	"os"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "test.db"))
	sqldb, err := appdb.Open("sqlite://" + path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := appdb.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	store := sqlite.New(sqldb)

	jwt := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	h := &api.Handlers{
		Store:         store,
		Activity:      activity.NewStore(store),
		JWT:           jwt,
		Hub:           hub.New(),
		Logger:        slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		StartedAt:     time.Now(),
		SecureCookies: false,
	}
	router := server.NewRouter(&server.Deps{Handlers: h, JWT: jwt, Store: store})
	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		store.Close()
	})
	return srv
}

func TestFoundationE2E(t *testing.T) {
	srv := newTestServer(t)
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// 1. setup status: needs setup
	if got := getBool(t, client, srv.URL+"/api/setup/status", "needs_setup"); !got {
		t.Fatal("expected needs_setup=true initially")
	}

	// 2. create admin
	postJSON(t, client, srv.URL+"/api/setup/admin", map[string]string{
		"username": "kayla", "password": "supersecret",
	}, http.StatusCreated)

	// 3. setup status: no longer needs setup
	if got := getBool(t, client, srv.URL+"/api/setup/status", "needs_setup"); got {
		t.Fatal("expected needs_setup=false after admin created")
	}

	// 4. second admin attempt is forbidden
	postJSON(t, client, srv.URL+"/api/setup/admin", map[string]string{
		"username": "evil", "password": "supersecret",
	}, http.StatusForbidden)

	// 5. login
	var loginResp struct {
		AccessToken string `json:"access_token"`
	}
	postJSONInto(t, client, srv.URL+"/api/auth/login", map[string]string{
		"username": "kayla", "password": "supersecret",
	}, http.StatusOK, &loginResp)
	if loginResp.AccessToken == "" {
		t.Fatal("no access token returned")
	}

	// 6. /api/me with bearer
	meReq, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	meResp, err := client.Do(meReq)
	if err != nil {
		t.Fatal(err)
	}
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("/api/me status = %d", meResp.StatusCode)
	}
	var me struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	json.NewDecoder(meResp.Body).Decode(&me)
	meResp.Body.Close()
	if me.Username != "kayla" || me.Role != "admin" {
		t.Fatalf("me = %+v", me)
	}

	// 7. WebSocket ping/pong (auth-gated)
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + loginResp.AccessToken}},
	})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageText, []byte("ping")); err != nil {
		t.Fatalf("ws write: %v", err)
	}
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if string(data) != "pong" {
		t.Errorf("ws got %q, want pong", data)
	}
	c.Close(websocket.StatusNormalClosure, "")

	// 8. logout (bearer + refresh cookie via jar)
	logoutReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginResp.AccessToken)
	logoutResp, err := client.Do(logoutReq)
	if err != nil {
		t.Fatal(err)
	}
	if logoutResp.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResp.StatusCode)
	}
	logoutResp.Body.Close()

	// 9. /api/me without token -> 401
	noTokResp, err := client.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatal(err)
	}
	if noTokResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/me without token status = %d, want 401", noTokResp.StatusCode)
	}
	noTokResp.Body.Close()
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
		DB     bool   `json:"db"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Status != "ok" || !body.DB {
		t.Fatalf("health body = %+v", body)
	}
}

// --- helpers ---

func getBool(t *testing.T, c *http.Client, url, key string) bool {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	m := map[string]bool{}
	json.NewDecoder(resp.Body).Decode(&m)
	return m[key]
}

func postJSON(t *testing.T, c *http.Client, url string, body any, wantStatus int) {
	t.Helper()
	postJSONInto(t, c, url, body, wantStatus, nil)
}

func postJSONInto(t *testing.T, c *http.Client, url string, body any, wantStatus int, into any) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := c.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("POST %s status = %d, want %d", url, resp.StatusCode, wantStatus)
	}
	if into != nil {
		json.NewDecoder(resp.Body).Decode(into)
	}
}
