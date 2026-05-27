package secrets

import (
	"context"
	"testing"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
)

func TestParseEnv(t *testing.T) {
	in := `
# a comment
FOO=bar
export BAZ = qux
QUOTED="hello world"
SINGLE='single'
EMPTY=
notakeyline
=novalue
WITH_EQUALS=a=b=c
`
	got := ParseEnv(in)
	want := map[string]string{
		"FOO": "bar", "BAZ": "qux", "QUOTED": "hello world",
		"SINGLE": "single", "EMPTY": "", "WITH_EQUALS": "a=b=c",
	}
	if len(got) != len(want) {
		t.Fatalf("parsed %d keys, want %d: %+v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if _, ok := got["notakeyline"]; ok {
		t.Error("line without '=' should be skipped")
	}
}

// memSecretStore is a minimal db.Store for the seal/reveal round-trip.
type memSecretStore struct {
	db.Store
	rows map[string]db.SecretRow
}

func (m *memSecretStore) UpsertSecret(_ context.Context, s db.SecretRow) error {
	m.rows[s.ID] = s
	return nil
}
func (m *memSecretStore) GetSecret(_ context.Context, id string) (db.SecretRow, error) {
	for _, r := range m.rows {
		if r.ID == id {
			return r, nil
		}
	}
	return db.SecretRow{}, db.ErrNotFound
}

func TestSealRevealRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	cipher, err := crypto.New(key)
	if err != nil {
		t.Fatal(err)
	}
	store := &memSecretStore{rows: map[string]db.SecretRow{}}
	svc := New(store, cipher)

	if err := svc.SetSecret(context.Background(), "g1", "DB_PASSWORD", "s3cr3t"); err != nil {
		t.Fatalf("SetSecret: %v", err)
	}
	// The stored blob must NOT contain the plaintext.
	var id string
	for k, r := range store.rows {
		id = k
		if string(r.ValueEncrypted) == "s3cr3t" {
			t.Fatal("value stored in plaintext")
		}
		_ = r
	}
	gotKey, gotVal, err := svc.Reveal(context.Background(), id)
	if err != nil {
		t.Fatalf("Reveal: %v", err)
	}
	if gotKey != "DB_PASSWORD" || gotVal != "s3cr3t" {
		t.Errorf("reveal = %q/%q, want DB_PASSWORD/s3cr3t", gotKey, gotVal)
	}
}
