package activity_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kaylaehman/stratum/backend/activity"
	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/db/sqlite"
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

func TestAppendMarshalsDetailAndDefaultsResult(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	act := activity.NewStore(st)

	if err := act.Append(ctx, activity.Entry{
		Action: "fs.write",
		Detail: map[string]string{"path": "/etc/hosts"},
		// Result intentionally empty -> defaults to success
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	rows, err := act.List(ctx, appdb.ActivityFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1", len(rows))
	}
	r := rows[0]
	if r.Result != activity.ResultSuccess {
		t.Errorf("Result = %q, want success", r.Result)
	}
	if r.DetailJSON == nil || *r.DetailJSON != `{"path":"/etc/hosts"}` {
		t.Errorf("DetailJSON = %v", r.DetailJSON)
	}
}

func TestAppendRequiresAction(t *testing.T) {
	ctx := context.Background()
	act := activity.NewStore(newStore(t))
	if err := act.Append(ctx, activity.Entry{Action: ""}); err == nil {
		t.Error("Append accepted an empty action")
	}
}

func TestContextEnrichment(t *testing.T) {
	base := context.Background()
	if activity.FromContext(base) != nil {
		t.Error("FromContext on bare context should be nil")
	}
	e := &activity.Entry{Action: "node.create"}
	ctx := activity.NewContext(base, e)
	got := activity.FromContext(ctx)
	if got == nil || got.Action != "node.create" {
		t.Fatalf("FromContext = %+v", got)
	}
	got.TargetType = ptr("node")
	if activity.FromContext(ctx).TargetType == nil {
		t.Error("enrichment via pointer did not persist")
	}
}

func ptr(s string) *string { return &s }
