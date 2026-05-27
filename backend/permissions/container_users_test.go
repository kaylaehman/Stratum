package permissions

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResolveContainer(t *testing.T) {
	passwd := "root:x:0:0::/root:/bin/sh\nnode:x:1000:1000::/home/node:/bin/sh\n"
	group := "root:x:0:\nmedia:x:44:node\n"
	calls := 0
	fetch := func(_ context.Context, _, _, path string) ([]byte, error) {
		calls++
		switch path {
		case "/etc/passwd":
			return []byte(passwd), nil
		case "/etc/group":
			return []byte(group), nil
		}
		return nil, errors.New("unexpected")
	}
	c := NewContainerCache(fetch, time.Minute)

	cu, err := c.ResolveContainer(context.Background(), "n1", "ctr1")
	if err != nil {
		t.Fatalf("ResolveContainer: %v", err)
	}
	if cu.UIDToName[1000] != "node" || cu.GIDToName[44] != "media" {
		t.Fatalf("parsed maps wrong: %+v", cu)
	}
	if len(cu.Passwd) != 2 || len(cu.Group) != 2 {
		t.Errorf("entries: passwd %d group %d", len(cu.Passwd), len(cu.Group))
	}

	// Cached: a second resolve must not re-fetch.
	before := calls
	if _, err := c.ResolveContainer(context.Background(), "n1", "ctr1"); err != nil {
		t.Fatal(err)
	}
	if calls != before {
		t.Errorf("expected cache hit, but fetcher was called again (%d -> %d)", before, calls)
	}

	// Invalidate -> refetch.
	c.Invalidate("ctr1")
	if _, err := c.ResolveContainer(context.Background(), "n1", "ctr1"); err != nil {
		t.Fatal(err)
	}
	if calls == before {
		t.Error("expected refetch after Invalidate")
	}
}

func TestResolveContainerDistroless(t *testing.T) {
	// Distroless: both files missing (fetch errors) -> empty tables, no error.
	fetch := func(_ context.Context, _, _, _ string) ([]byte, error) {
		return nil, errors.New("file not found in container")
	}
	c := NewContainerCache(fetch, time.Minute)
	cu, err := c.ResolveContainer(context.Background(), "n1", "distroless")
	if err != nil {
		t.Fatalf("distroless should not error, got %v", err)
	}
	if len(cu.UIDToName) != 0 || len(cu.GIDToName) != 0 {
		t.Errorf("distroless should yield empty maps, got %+v", cu)
	}
}
