package stacks_test

import (
	"context"
	"errors"
	"testing"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/stacks"
)

// --- fakes ---

type fakeStore struct {
	envVars map[string]appdb.StackEnvRow // key: nodeID+"/"+project+"/"+key
	secrets map[string]appdb.SecretRow   // key: id
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		envVars: make(map[string]appdb.StackEnvRow),
		secrets: make(map[string]appdb.SecretRow),
	}
}

func envKey(nodeID, project, key string) string { return nodeID + "/" + project + "/" + key }

func (f *fakeStore) UpsertStackEnvVar(_ context.Context, r appdb.StackEnvRow) error {
	f.envVars[envKey(r.NodeID, r.ProjectName, r.Key)] = r
	return nil
}

func (f *fakeStore) ListStackEnvVars(_ context.Context, nodeID, projectName string) ([]appdb.StackEnvRow, error) {
	var out []appdb.StackEnvRow
	for _, r := range f.envVars {
		if r.NodeID == nodeID && r.ProjectName == projectName {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) DeleteStackEnvVar(_ context.Context, nodeID, projectName, key string) error {
	k := envKey(nodeID, projectName, key)
	if _, ok := f.envVars[k]; !ok {
		return appdb.ErrNotFound
	}
	delete(f.envVars, k)
	return nil
}

func (f *fakeStore) GetSecret(_ context.Context, id string) (appdb.SecretRow, error) {
	r, ok := f.secrets[id]
	if !ok {
		return appdb.SecretRow{}, appdb.ErrNotFound
	}
	return r, nil
}

// --- tests ---

func TestSanitizeProject(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"my-stack", "my-stack"},
		{"my stack", "my-stack"},
		{"../evil", "---evil"}, // each non-allowed char maps to '-'
		{"proj_01", "proj_01"},
		{"proj.v2", "proj-v2"}, // dot is not allowed (prevents .. sequences)
	}
	for _, tc := range cases {
		if got := stacks.SanitizeProject(tc.in); got != tc.want {
			t.Errorf("SanitizeProject(%q) = %q; want %q", tc.in, got, tc.want)
		}
	}
}

func TestEnvVarRoundTrip(t *testing.T) {
	fs := newFakeStore()
	_ = fs.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "proj", Key: "DB_URL", Value: "postgres://localhost",
	})

	rows, err := fs.ListStackEnvVars(context.Background(), "n1", "proj")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Key != "DB_URL" {
		t.Errorf("key: got %q, want DB_URL", rows[0].Key)
	}

	if err := fs.DeleteStackEnvVar(context.Background(), "n1", "proj", "DB_URL"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	rows, _ = fs.ListStackEnvVars(context.Background(), "n1", "proj")
	if len(rows) != 0 {
		t.Errorf("expected 0 rows after delete, got %d", len(rows))
	}
}

func TestDeleteEnvVarNotFound(t *testing.T) {
	fs := newFakeStore()
	err := fs.DeleteStackEnvVar(context.Background(), "n1", "proj", "MISSING")
	if !errors.Is(err, appdb.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSecretRefResolvesNotFound(t *testing.T) {
	fs := newFakeStore()
	// No matching secret in the store; GetSecret returns ErrNotFound.
	_, err := fs.GetSecret(context.Background(), "nonexistent")
	if !errors.Is(err, appdb.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSecretRefResolves(t *testing.T) {
	fs := newFakeStore()
	fs.secrets["sec-1"] = appdb.SecretRow{
		ID: "sec-1", GroupID: "g1", Key: "API_KEY",
		ValueEncrypted: []byte("sealed"), // not decrypted in this unit — see service_test
	}
	row, err := fs.GetSecret(context.Background(), "sec-1")
	if err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if row.Key != "API_KEY" {
		t.Errorf("key: got %q, want API_KEY", row.Key)
	}
}

func TestUpsertOverwrites(t *testing.T) {
	fs := newFakeStore()
	_ = fs.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "p", Key: "K", Value: "v1",
	})
	_ = fs.UpsertStackEnvVar(context.Background(), appdb.StackEnvRow{
		NodeID: "n1", ProjectName: "p", Key: "K", Value: "v2",
	})
	rows, _ := fs.ListStackEnvVars(context.Background(), "n1", "p")
	if len(rows) != 1 {
		t.Fatalf("expected 1 row after upsert, got %d", len(rows))
	}
	if rows[0].Value != "v2" {
		t.Errorf("value after upsert: got %q, want v2", rows[0].Value)
	}
}
