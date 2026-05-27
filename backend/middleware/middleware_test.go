package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/auth"
	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
	mw "github.com/kaylaehman/stratum/backend/middleware"
)

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

func TestActivityLogsSuccess(t *testing.T) {
	st := newStore(t)
	act := activity.NewStore(st)
	h := mw.Activity(act)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/x", nil))

	rows, _ := act.List(context.Background(), appdb.ActivityFilter{})
	if len(rows) != 1 || rows[0].Result != activity.ResultSuccess {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestActivityLogsOnPanic(t *testing.T) {
	st := newStore(t)
	act := activity.NewStore(st)

	// Recoverer (outer) wraps Activity (inner), mirroring the production chain.
	h := chimw.Recoverer(mw.Activity(act)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/danger", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	rows, _ := act.List(context.Background(), appdb.ActivityFilter{})
	if len(rows) != 1 || rows[0].Result != activity.ResultError {
		t.Fatalf("expected one error row, got %+v", rows)
	}
}

func TestActivitySuppressed(t *testing.T) {
	st := newStore(t)
	act := activity.NewStore(st)
	h := mw.Activity(act)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if e := activity.FromContext(r.Context()); e != nil {
			e.Suppressed = true
		}
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/upload", nil))

	rows, _ := act.List(context.Background(), appdb.ActivityFilter{})
	if len(rows) != 0 {
		t.Fatalf("suppressed request should write no row, got %+v", rows)
	}
}

type fakeUserStore struct{ user appdb.User }

func (f fakeUserStore) GetUserByID(_ context.Context, id string) (appdb.User, error) {
	if id == f.user.ID {
		return f.user, nil
	}
	return appdb.User{}, appdb.ErrNotFound
}

func TestAuthAcceptsValidToken(t *testing.T) {
	j := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	store := fakeUserStore{user: appdb.User{ID: "u1", Username: "kayla", Role: "admin"}}
	token, _, _, _ := j.Issue("u1")

	var gotUser appdb.User
	var ok bool
	h := mw.Auth(j, store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, ok = mw.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !ok || gotUser.Username != "kayla" {
		t.Fatalf("status=%d ok=%v user=%+v", rec.Code, ok, gotUser)
	}
}

func TestAuthRejectsMissingAndBadToken(t *testing.T) {
	j := auth.NewJWT([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	store := fakeUserStore{user: appdb.User{ID: "u1"}}
	h := mw.Auth(j, store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, tok := range []string{"", "Bearer garbage", "Basic xyz"} {
		req := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		if tok != "" {
			req.Header.Set("Authorization", tok)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: status = %d, want 401", tok, rec.Code)
		}
	}
}
