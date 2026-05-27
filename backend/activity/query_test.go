package activity_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kaylaehman/stratum/backend/activity"
	appdb "github.com/kaylaehman/stratum/backend/db"
)

// fakeStore is a minimal db.Store implementation for cursor/paging unit tests.
type fakeStore struct {
	appdb.Store // embed to satisfy interface; unused methods panic if called
	rows        []appdb.ActivityEntry
}

func (f *fakeStore) QueryActivityLog(_ context.Context, q appdb.ActivityQuery) ([]appdb.ActivityEntry, error) {
	start := 0
	if q.CursorRowID != nil {
		for i, r := range f.rows {
			if r.RowID < *q.CursorRowID {
				start = i
				break
			}
		}
	}
	out := f.rows[start:]
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func makeRows(n int) []appdb.ActivityEntry {
	rows := make([]appdb.ActivityEntry, n)
	for i := range rows {
		rows[i] = appdb.ActivityEntry{
			ID:     string(rune('a' + i)),
			RowID:  int64(n - i), // descending rowid (newest first)
			Action: "fs.write",
			Result: "success",
		}
	}
	return rows
}

func TestEncodeCursorRoundTrip(t *testing.T) {
	for _, id := range []int64{1, 42, 9999, 0, -1} {
		enc := activity.EncodeCursor(id)
		got, err := activity.DecodeCursor(enc)
		if err != nil {
			t.Errorf("DecodeCursor(%q): %v", enc, err)
		}
		if got != id {
			t.Errorf("round-trip %d -> %q -> %d", id, enc, got)
		}
	}
}

func TestDecodeCursorMalformed(t *testing.T) {
	cases := []string{"!!!", "not-base64$$", "dGVzdA==x"} // last is valid base64 but non-int payload
	for _, c := range cases {
		_, err := activity.DecodeCursor(c)
		if !errors.Is(err, activity.ErrBadCursor) {
			t.Errorf("DecodeCursor(%q) err = %v, want ErrBadCursor", c, err)
		}
	}
}

func TestPageFirstPage(t *testing.T) {
	store := &fakeStore{rows: makeRows(5)}
	entries, next, err := activity.Page(context.Background(), store, appdb.ActivityQuery{}, "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if next == "" {
		t.Error("expected a next_cursor when more rows exist")
	}
}

func TestPageLastPage(t *testing.T) {
	store := &fakeStore{rows: makeRows(3)}
	entries, next, err := activity.Page(context.Background(), store, appdb.ActivityQuery{}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if next != "" {
		t.Errorf("expected empty next_cursor on last page, got %q", next)
	}
}

func TestPageCursorAdvances(t *testing.T) {
	rows := makeRows(5) // rowids: 5,4,3,2,1
	store := &fakeStore{rows: rows}

	// Page 1: limit 2
	page1, cur1, err := activity.Page(context.Background(), store, appdb.ActivityQuery{}, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d", len(page1))
	}
	if cur1 == "" {
		t.Fatal("expected cursor after page1")
	}

	// Page 2: use cursor
	page2, cur2, err := activity.Page(context.Background(), store, appdb.ActivityQuery{}, cur1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 2 {
		t.Fatalf("page2 len = %d", len(page2))
	}
	_ = cur2 // may or may not be empty depending on fake implementation
}

func TestPageBadCursorError(t *testing.T) {
	store := &fakeStore{rows: makeRows(3)}
	_, _, err := activity.Page(context.Background(), store, appdb.ActivityQuery{}, "!!!badcursor!!!", 10)
	if !errors.Is(err, activity.ErrBadCursor) {
		t.Errorf("bad cursor err = %v, want ErrBadCursor", err)
	}
}

func TestPageDefaultsAndCapsLimit(t *testing.T) {
	rows := makeRows(10)
	store := &fakeStore{rows: rows}

	// limit=0 should default to 50, but we only have 10 rows
	entries, next, err := activity.Page(context.Background(), store, appdb.ActivityQuery{}, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 10 {
		t.Fatalf("default limit: got %d rows, want 10", len(entries))
	}
	if next != "" {
		t.Errorf("unexpected next cursor %q", next)
	}
}
