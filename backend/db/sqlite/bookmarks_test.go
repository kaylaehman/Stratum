package sqlite_test

import (
	"context"
	"testing"

	appdb "github.com/kaylaehman/stratum/backend/db"
)

func TestBookmarksCRUDAndScoping(t *testing.T) {
	ctx := context.Background()
	st, _ := newStoreWithDB(t)

	for _, u := range []string{"u1", "u2"} {
		if err := st.CreateUser(ctx, appdb.User{ID: u, Username: u, PasswordHash: "h", Role: "admin"}); err != nil {
			t.Fatal(err)
		}
	}

	mk := func(id, user, label string) appdb.Bookmark {
		return appdb.Bookmark{ID: id, UserID: user, Label: label, ResourceType: "container", ResourceRef: "c-" + id}
	}
	for _, b := range []appdb.Bookmark{mk("b1", "u1", "Plex"), mk("b2", "u1", "DB"), mk("b3", "u2", "Other")} {
		if err := st.CreateBookmark(ctx, b); err != nil {
			t.Fatalf("create %s: %v", b.ID, err)
		}
	}

	u1, err := st.ListBookmarksByUser(ctx, "u1")
	if err != nil || len(u1) != 2 {
		t.Fatalf("u1 bookmarks = %d (%v), want 2", len(u1), err)
	}

	// Reorder u1: b2 before b1.
	if err := st.SetBookmarkOrder(ctx, "u1", []string{"b2", "b1"}); err != nil {
		t.Fatal(err)
	}
	u1, _ = st.ListBookmarksByUser(ctx, "u1")
	if u1[0].ID != "b2" || u1[1].ID != "b1" {
		t.Errorf("order after reorder = [%s,%s], want [b2,b1]", u1[0].ID, u1[1].ID)
	}

	// IDOR: u2 cannot delete u1's bookmark.
	if err := st.DeleteBookmark(ctx, "b1", "u2"); err != appdb.ErrNotFound {
		t.Errorf("cross-user delete err = %v, want ErrNotFound", err)
	}
	// Owner can delete.
	if err := st.DeleteBookmark(ctx, "b1", "u1"); err != nil {
		t.Errorf("owner delete: %v", err)
	}
	u1, _ = st.ListBookmarksByUser(ctx, "u1")
	if len(u1) != 1 {
		t.Errorf("after delete u1 has %d, want 1", len(u1))
	}
	// u2's bookmark untouched.
	if u2, _ := st.ListBookmarksByUser(ctx, "u2"); len(u2) != 1 {
		t.Errorf("u2 should still have 1 bookmark")
	}
}
