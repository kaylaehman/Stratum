package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/api"
	"github.com/kaylaehman/stratum/backend/auth"
	"github.com/kaylaehman/stratum/backend/crypto"
	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	"github.com/kaylaehman/stratum/backend/docker"
	"github.com/kaylaehman/stratum/backend/fs"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/logtail"
	"github.com/kaylaehman/stratum/backend/mountindex"
	"github.com/kaylaehman/stratum/backend/nodeconn"
	"github.com/kaylaehman/stratum/backend/nodes"
	"github.com/kaylaehman/stratum/backend/depgraph"
	"github.com/kaylaehman/stratum/backend/permissions"
	"github.com/kaylaehman/stratum/backend/security"
	"github.com/kaylaehman/stratum/backend/server"
	"github.com/kaylaehman/stratum/backend/topology"
	"github.com/kaylaehman/stratum/backend/volumes"
	"github.com/kaylaehman/stratum/backend/webhooks"

	"context"
	"errors"
	"log/slog"
	"os"
)

var errNoDockerInTest = errors.New("no docker in test")

// newNodeTestServer builds a server with the nodes service wired in and returns
// it plus an admin access token (via the setup + login flow).
func newNodeTestServer(t *testing.T) (*httptest.Server, string) {
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

	key := make([]byte, crypto.KeySize)
	for i := range key {
		key[i] = 0x9
	}
	cipher, _ := crypto.New(key)
	jwt := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), 15*time.Minute)
	hb := hub.New()
	noDocker := func(context.Context, string) (*docker.Client, error) { return nil, errNoDockerInTest }
	logsMgr := logtail.NewManager(noDocker, hb, func(context.Context, string, string) (bool, error) { return true, nil })
	ctrUsers := permissions.NewContainerCache(func(context.Context, string, string, string) ([]byte, error) { return nil, nil }, time.Minute)
	mountIdx := mountindex.New(store, noDocker, time.Minute)

	h := &api.Handlers{
		Store:          store,
		Activity:       activity.NewStore(store),
		JWT:            jwt,
		Hub:            hb,
		Nodes:          nodes.NewService(store, cipher),
		Files:          fs.NewService(store, cipher, 0),
		Conn:           nodeconn.NewManager(store, cipher),
		ContainerUsers: ctrUsers,
		Logs:           logsMgr,
		Mounts:         mountIdx,
		Security:       security.NewScanner(store, security.ClientProvider(noDocker), ctrUsers, time.Minute),
		Volumes:        volumes.New(store, volumes.ClientProvider(noDocker), mountIdx, 0),
		Topology:       topology.New(store, topology.ClientProvider(noDocker)),
		DepGraph:       depgraph.New(store, depgraph.ClientProvider(noDocker), mountIdx),
		Webhooks:       webhooks.New(store),
		Logger:         slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
		StartedAt:      time.Now(),
		PreviewLimiter: rate.NewLimiter(rate.Every(time.Millisecond), 100),
	}
	srv := httptest.NewServer(server.NewRouter(&server.Deps{Handlers: h, JWT: jwt, Store: store}))
	t.Cleanup(func() { srv.Close(); store.Close() })

	// Create admin + login.
	c := &http.Client{}
	postJSONInto(t, c, srv.URL+"/api/setup/admin", map[string]string{"username": "admin", "password": "supersecret"}, http.StatusCreated, nil)
	var login struct {
		AccessToken string `json:"access_token"`
	}
	postJSONInto(t, c, srv.URL+"/api/auth/login", map[string]string{"username": "admin", "password": "supersecret"}, http.StatusOK, &login)
	if login.AccessToken == "" {
		t.Fatal("no token")
	}
	return srv, login.AccessToken
}

func authReq(t *testing.T, method, url, token string, body any) *http.Request {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestNodeCreateListGetDelete(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Create a node with no SSH creds (probes instantly, no network).
	var created struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Name string `json:"name"`
	}
	resp, err := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "bare", "host": "10.0.0.50", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_key"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d", resp.StatusCode)
	}
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == "" || created.Type != "ssh" {
		t.Fatalf("created = %+v", created)
	}

	// List
	listResp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes", token, nil))
	var list struct {
		Nodes []map[string]any `json:"nodes"`
	}
	json.NewDecoder(listResp.Body).Decode(&list)
	listResp.Body.Close()
	if len(list.Nodes) != 1 {
		t.Fatalf("list len = %d", len(list.Nodes))
	}

	// Delete
	delResp, _ := c.Do(authReq(t, http.MethodDelete, srv.URL+"/api/nodes/"+created.ID, token, nil))
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d", delResp.StatusCode)
	}
	delResp.Body.Close()

	getResp, _ := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/nodes/"+created.ID, token, nil))
	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete status = %d, want 404", getResp.StatusCode)
	}
	getResp.Body.Close()
}

func TestNodeCreateRequiresHostKeyAndHidesSecret(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// SSH creds but no accepted_host_key -> 400 host_key_required.
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes", token, map[string]any{
		"name": "ssh-host", "host": "127.0.0.1", "ssh_port": 22,
		"credentials": map[string]string{"method": "ssh_password", "ssh_user": "root", "ssh_password": "LEAKME-PASSWORD"},
	}))
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", resp.StatusCode, body)
	}
	if bytes.Contains(body, []byte("LEAKME-PASSWORD")) {
		t.Error("error response leaked the submitted password")
	}
}

func TestProbePreviewInsecureDockerRequiresAck(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, _ := c.Do(authReq(t, http.MethodPost, srv.URL+"/api/nodes/probe-preview", token, map[string]any{
		"host": "10.0.0.5", "docker_endpoint": "tcp://10.0.0.5:2375",
		"credentials": map[string]string{},
	}))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (insecure docker needs ack)", resp.StatusCode)
	}
}
