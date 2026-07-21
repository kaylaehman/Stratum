package sqlite_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/db/sqlite"
)

// newStore opens a migrated temp-file SQLite store for a test.
func newStore(t *testing.T) *sqlite.Store {
	t.Helper()
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "test.db"))
	sqldb, err := appdb.Open("sqlite://" + path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := appdb.Migrate(sqldb); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	st := sqlite.New(sqldb)
	t.Cleanup(func() { st.Close() })
	return st
}

func TestUserCRUD(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if n, err := st.CountUsers(ctx); err != nil || n != 0 {
		t.Fatalf("CountUsers initial = %d, %v; want 0, nil", n, err)
	}

	u := appdb.User{ID: "u1", Username: "kayla", Email: "k@example.com", PasswordHash: "hash", Role: "admin"}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := st.GetUserByUsername(ctx, "kayla")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got.ID != "u1" || got.Email != "k@example.com" || got.Role != "admin" {
		t.Errorf("got %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt not populated")
	}

	if _, err := st.GetUserByID(ctx, "missing"); err != appdb.ErrNotFound {
		t.Errorf("GetUserByID(missing) err = %v, want ErrNotFound", err)
	}

	if n, _ := st.CountUsers(ctx); n != 1 {
		t.Errorf("CountUsers = %d, want 1", n)
	}

	// Password reset.
	if err := st.UpdatePasswordHash(ctx, "u1", "newhash"); err != nil {
		t.Fatalf("UpdatePasswordHash: %v", err)
	}
	if got, _ := st.GetUserByID(ctx, "u1"); got.PasswordHash != "newhash" {
		t.Errorf("PasswordHash = %q, want newhash", got.PasswordHash)
	}
	if err := st.UpdatePasswordHash(ctx, "missing", "x"); err != appdb.ErrNotFound {
		t.Errorf("UpdatePasswordHash(missing) = %v, want ErrNotFound", err)
	}

	// Profile (username + email) edit.
	if err := st.UpdateUserProfile(ctx, "u1", "kayla2", "k2@example.com"); err != nil {
		t.Fatalf("UpdateUserProfile: %v", err)
	}
	got2, err := st.GetUserByID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUserByID after profile update: %v", err)
	}
	if got2.Username != "kayla2" || got2.Email != "k2@example.com" {
		t.Errorf("after profile update got %+v", got2)
	}
	// Old username no longer resolves; new one does.
	if _, err := st.GetUserByUsername(ctx, "kayla"); err != appdb.ErrNotFound {
		t.Errorf("old username should be gone, got %v", err)
	}
	if _, err := st.GetUserByUsername(ctx, "kayla2"); err != nil {
		t.Errorf("new username should resolve, got %v", err)
	}
	// Clearing email round-trips as empty.
	if err := st.UpdateUserProfile(ctx, "u1", "kayla2", ""); err != nil {
		t.Fatalf("UpdateUserProfile clear email: %v", err)
	}
	if got, _ := st.GetUserByID(ctx, "u1"); got.Email != "" {
		t.Errorf("Email = %q, want empty after clear", got.Email)
	}
	if err := st.UpdateUserProfile(ctx, "missing", "x", ""); err != appdb.ErrNotFound {
		t.Errorf("UpdateUserProfile(missing) = %v, want ErrNotFound", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	if err := st.CreateUser(ctx, appdb.User{ID: "u1", Username: "k", PasswordHash: "h", Role: "admin"}); err != nil {
		t.Fatal(err)
	}

	exp := time.Now().Add(24 * time.Hour)
	sess := appdb.Session{ID: "s1", UserID: "u1", RefreshHash: "abc", UserAgent: "go-test", IP: "127.0.0.1", ExpiresAt: exp}
	if err := st.CreateSession(ctx, sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := st.GetSession(ctx, "s1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.RefreshHash != "abc" || got.UserID != "u1" || got.RevokedAt != nil {
		t.Errorf("got %+v", got)
	}
	if d := got.ExpiresAt.Sub(exp); d > time.Second || d < -time.Second {
		t.Errorf("ExpiresAt round-trip drift = %v", d)
	}

	now := time.Now()
	if err := st.RevokeSession(ctx, "s1", now); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	got, _ = st.GetSession(ctx, "s1")
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set after revoke")
	}
}

func TestActivityAppendAndList(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	uid := "u1"
	if err := st.CreateUser(ctx, appdb.User{ID: uid, Username: "k", PasswordHash: "h", Role: "admin"}); err != nil {
		t.Fatal(err)
	}

	detail := `{"path":"/etc/x"}`
	tt := "file"
	entries := []appdb.ActivityEntry{
		{ID: "a1", UserID: &uid, Action: "auth.login", Result: "success"},
		{ID: "a2", UserID: &uid, Action: "fs.write", TargetType: &tt, DetailJSON: &detail, Result: "success"},
	}
	for _, e := range entries {
		if err := st.AppendActivity(ctx, e); err != nil {
			t.Fatalf("AppendActivity: %v", err)
		}
	}

	all, err := st.ListActivity(ctx, appdb.ActivityFilter{})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("ListActivity len = %d, want 2", len(all))
	}

	act := "fs.write"
	filtered, err := st.ListActivity(ctx, appdb.ActivityFilter{Action: &act})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].ID != "a2" {
		t.Fatalf("filtered = %+v", filtered)
	}
	if filtered[0].DetailJSON == nil || !strings.Contains(*filtered[0].DetailJSON, "/etc/x") {
		t.Errorf("detail not preserved: %+v", filtered[0].DetailJSON)
	}
}
