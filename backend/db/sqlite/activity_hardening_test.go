package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
)

// newStoreWithDB returns both the Store and the raw *sql.DB so a test can attempt
// the UPDATE/DELETE the append-only triggers must reject.
func newStoreWithDB(t *testing.T) (*sqlite.Store, *sql.DB) {
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
	return st, sqldb
}

func TestActivityLogAppendOnlyTriggers(t *testing.T) {
	ctx := context.Background()
	st, sqldb := newStoreWithDB(t)

	if err := st.AppendActivity(ctx, appdb.ActivityEntry{ID: "a1", Action: "fs.write", Result: "success"}); err != nil {
		t.Fatalf("AppendActivity: %v", err)
	}

	if _, err := sqldb.ExecContext(ctx, `UPDATE activity_log SET action = 'tampered'`); err == nil {
		t.Error("UPDATE on activity_log should be rejected by the append-only trigger")
	} else if !strings.Contains(err.Error(), "append-only") {
		t.Errorf("UPDATE error = %v, want append-only abort", err)
	}

	if _, err := sqldb.ExecContext(ctx, `DELETE FROM activity_log`); err == nil {
		t.Error("DELETE on activity_log should be rejected by the append-only trigger")
	} else if !strings.Contains(err.Error(), "append-only") {
		t.Errorf("DELETE error = %v, want append-only abort", err)
	}

	// The row must still be intact and readable.
	rows, err := st.ListActivity(ctx, appdb.ActivityFilter{})
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(rows) != 1 || rows[0].Action != "fs.write" {
		t.Errorf("audit row was mutated/lost: %+v", rows)
	}
}

func TestQueryActivityLogFiltersAndKeyset(t *testing.T) {
	ctx := context.Background()
	st, _ := newStoreWithDB(t)

	uid := "u1"
	other := "u2"
	if err := st.CreateUser(ctx, appdb.User{ID: uid, Username: "k", PasswordHash: "h", Role: "admin"}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateUser(ctx, appdb.User{ID: other, Username: "o", PasswordHash: "h", Role: "viewer"}); err != nil {
		t.Fatal(err)
	}

	tFile := "file"
	tNode := "node"
	mk := func(id, user, action, ttype, target, detail, result string) appdb.ActivityEntry {
		e := appdb.ActivityEntry{ID: id, UserID: &user, Action: action, Result: result}
		if ttype != "" {
			e.TargetType = &ttype
		}
		if target != "" {
			e.TargetID = &target
		}
		if detail != "" {
			e.DetailJSON = &detail
		}
		return e
	}
	// Insert in a known order; rowid follows insertion order.
	seed := []appdb.ActivityEntry{
		mk("a1", uid, "auth.login", "", "", "", "success"),
		mk("a2", uid, "fs.write", tFile, "/etc/conf", `{"path":"/etc/conf"}`, "success"),
		mk("a3", other, "fs.delete", tFile, "/data/100%backup", `{"path":"/data/100%backup"}`, "error"),
		mk("a4", uid, "node.create", tNode, "n1", "", "success"),
		mk("a5", uid, "fs.rename", tFile, "/etc/hosts", "", "success"),
	}
	for _, e := range seed {
		if err := st.AppendActivity(ctx, e); err != nil {
			t.Fatalf("append %s: %v", e.ID, err)
		}
	}

	// Default order is newest-first by rowid.
	all, err := st.QueryActivityLog(ctx, appdb.ActivityQuery{})
	if err != nil {
		t.Fatalf("QueryActivityLog: %v", err)
	}
	if len(all) != 5 || all[0].ID != "a5" || all[4].ID != "a1" {
		t.Fatalf("default order wrong: got %d rows, first=%s last=%s", len(all), all[0].ID, all[len(all)-1].ID)
	}
	if all[0].RowID <= all[4].RowID {
		t.Errorf("rowid should descend: %d then %d", all[0].RowID, all[4].RowID)
	}

	// Exact action.
	act := "fs.write"
	if r, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{Action: &act}); len(r) != 1 || r[0].ID != "a2" {
		t.Errorf("action=fs.write got %+v", r)
	}

	// Action prefix fs. matches write/delete/rename only.
	pre := "fs."
	if r, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{ActionPrefix: &pre}); len(r) != 3 {
		t.Errorf("action prefix fs. = %d rows, want 3", len(r))
	}

	// Target type.
	if r, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{TargetType: &tNode}); len(r) != 1 || r[0].ID != "a4" {
		t.Errorf("target_type=node got %+v", r)
	}

	// Result.
	res := "error"
	if r, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{Result: &res}); len(r) != 1 || r[0].ID != "a3" {
		t.Errorf("result=error got %+v", r)
	}

	// User.
	if r, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{UserID: &other}); len(r) != 1 || r[0].ID != "a3" {
		t.Errorf("user=u2 got %+v", r)
	}

	// q substring with a literal % must match only the row whose path contains it
	// (escaped LIKE — the % is not a wildcard).
	if r, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{Q: "100%backup"}); len(r) != 1 || r[0].ID != "a3" {
		t.Errorf("q=100%%backup got %+v (LIKE escaping broken?)", r)
	}
	// A bare % must be treated literally, not as a wildcard: it matches only the
	// one row that actually contains a "%" (a3), not all 5 rows.
	if r, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{Q: "%"}); len(r) != 1 || r[0].ID != "a3" {
		t.Errorf("q=%% should match only the literal-%% row a3, got %+v", r)
	}

	// Keyset: page size 2, walk the cursor.
	page1, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{Limit: 2})
	if len(page1) != 2 || page1[0].ID != "a5" || page1[1].ID != "a4" {
		t.Fatalf("page1 = %+v", page1)
	}
	cursor := page1[1].RowID
	page2, _ := st.QueryActivityLog(ctx, appdb.ActivityQuery{Limit: 2, CursorRowID: &cursor})
	if len(page2) != 2 || page2[0].ID != "a3" || page2[1].ID != "a2" {
		t.Fatalf("page2 = %+v", page2)
	}
}

func TestQueryActivityLogDateRange(t *testing.T) {
	ctx := context.Background()
	st, _ := newStoreWithDB(t)

	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		e := appdb.ActivityEntry{
			ID:        "d" + string(rune('1'+i)),
			Action:    "fs.write",
			Result:    "success",
			CreatedAt: base.AddDate(0, 0, i), // May 1, 2, 3
		}
		if err := st.AppendActivity(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	from := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 2, 23, 59, 59, 0, time.UTC)
	r, err := st.QueryActivityLog(ctx, appdb.ActivityQuery{From: &from, To: &to})
	if err != nil {
		t.Fatal(err)
	}
	if len(r) != 1 {
		t.Errorf("date range May 2 = %d rows, want 1: %+v", len(r), r)
	}
}
