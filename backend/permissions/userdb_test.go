package permissions

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- ParsePasswd tests ---

func TestParsePasswd_Basic(t *testing.T) {
	input := `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
# this is a comment
kayla:x:1000:1000:Kayla,,,:/home/kayla:/bin/bash

malformed_line
:x:bad:0:no name
too:few:fields
`
	got := ParsePasswd(strings.NewReader(input))

	cases := []struct{ id int; name string }{
		{0, "root"},
		{1, "daemon"},
		{1000, "kayla"},
	}
	for _, c := range cases {
		if got[c.id] != c.name {
			t.Errorf("UID %d: want %q, got %q", c.id, c.name, got[c.id])
		}
	}
	// Malformed lines must not appear.
	if _, ok := got[-1]; ok {
		t.Error("negative id should not be present")
	}
	// Only 3 valid entries expected.
	if len(got) != 3 {
		t.Errorf("expected 3 entries, got %d: %v", len(got), got)
	}
}

func TestParsePasswd_Empty(t *testing.T) {
	got := ParsePasswd(strings.NewReader(""))
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestParsePasswd_DuplicateID_LastWins(t *testing.T) {
	input := "first:x:500:500::/home/first:/bin/sh\nsecond:x:500:500::/home/second:/bin/sh\n"
	got := ParsePasswd(strings.NewReader(input))
	if got[500] != "second" {
		t.Errorf("last-wins expected %q, got %q", "second", got[500])
	}
}

// --- ParseGroup tests ---

func TestParseGroup_Basic(t *testing.T) {
	input := `root:x:0:
sudo:x:27:kayla
kayla:x:1000:
# comment line

bad
`
	got := ParseGroup(strings.NewReader(input))

	cases := []struct{ id int; name string }{
		{0, "root"},
		{27, "sudo"},
		{1000, "kayla"},
	}
	for _, c := range cases {
		if got[c.id] != c.name {
			t.Errorf("GID %d: want %q, got %q", c.id, c.name, got[c.id])
		}
	}
	if len(got) != 3 {
		t.Errorf("expected 3 entries, got %d: %v", len(got), got)
	}
}

// --- UserDB tests ---

const passwdFixture = "root:x:0:0:root:/root:/bin/bash\nkayla:x:1000:1000::/home/kayla:/bin/bash\n"
const groupFixture = "root:x:0:\nkayla:x:1000:\ndocker:x:999:\n"

func makeFetcher(passwdData, groupData string, passwdErr, groupErr error, calls *atomic.Int64) Fetcher {
	return func(_ context.Context, _ string, path string) ([]byte, error) {
		calls.Add(1)
		switch path {
		case "/etc/passwd":
			if passwdErr != nil {
				return nil, passwdErr
			}
			return []byte(passwdData), nil
		case "/etc/group":
			if groupErr != nil {
				return nil, groupErr
			}
			return []byte(groupData), nil
		}
		return nil, errors.New("unexpected path: " + path)
	}
}

func TestUserDB_Resolve_CorrectMaps(t *testing.T) {
	var calls atomic.Int64
	db := NewUserDB(makeFetcher(passwdFixture, groupFixture, nil, nil, &calls), 5*time.Minute)

	maps, err := db.Resolve(context.Background(), "node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if maps.UIDToName[0] != "root" {
		t.Errorf("UID 0: want root, got %q", maps.UIDToName[0])
	}
	if maps.UIDToName[1000] != "kayla" {
		t.Errorf("UID 1000: want kayla, got %q", maps.UIDToName[1000])
	}
	if maps.GIDToName[999] != "docker" {
		t.Errorf("GID 999: want docker, got %q", maps.GIDToName[999])
	}
}

func TestUserDB_Resolve_CacheHit(t *testing.T) {
	var calls atomic.Int64
	db := NewUserDB(makeFetcher(passwdFixture, groupFixture, nil, nil, &calls), 5*time.Minute)

	_, err := db.Resolve(context.Background(), "node1")
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}
	first := calls.Load()

	_, err = db.Resolve(context.Background(), "node1")
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}
	second := calls.Load()

	if second != first {
		t.Errorf("cache miss on second resolve: fetcher called %d times total, expected %d", second, first)
	}
}

func TestUserDB_Resolve_InvalidateTriggersFetch(t *testing.T) {
	var calls atomic.Int64
	db := NewUserDB(makeFetcher(passwdFixture, groupFixture, nil, nil, &calls), 5*time.Minute)

	_, _ = db.Resolve(context.Background(), "node1")
	before := calls.Load()

	db.Invalidate("node1")

	_, err := db.Resolve(context.Background(), "node1")
	if err != nil {
		t.Fatalf("after invalidate: %v", err)
	}
	after := calls.Load()

	if after <= before {
		t.Errorf("expected fetcher to be called after Invalidate; calls before=%d after=%d", before, after)
	}
}

func TestUserDB_Resolve_PasswdFailGroupSucceeds(t *testing.T) {
	var calls atomic.Int64
	fetchErr := errors.New("sftp: no such file")
	db := NewUserDB(makeFetcher("", groupFixture, fetchErr, nil, &calls), 5*time.Minute)

	maps, err := db.Resolve(context.Background(), "node1")
	if err != nil {
		t.Fatalf("expected no error when only passwd fails, got: %v", err)
	}
	if len(maps.UIDToName) != 0 {
		t.Errorf("UIDToName should be empty, got %v", maps.UIDToName)
	}
	if len(maps.GIDToName) == 0 {
		t.Error("GIDToName should be populated")
	}
	if maps.GIDToName[1000] != "kayla" {
		t.Errorf("GID 1000: want kayla, got %q", maps.GIDToName[1000])
	}
}

func TestUserDB_Resolve_BothFetchesFail_ReturnsError(t *testing.T) {
	var calls atomic.Int64
	fetchErr := errors.New("node unreachable")
	db := NewUserDB(makeFetcher("", "", fetchErr, fetchErr, &calls), 5*time.Minute)

	_, err := db.Resolve(context.Background(), "node1")
	if err == nil {
		t.Fatal("expected error when both fetches fail, got nil")
	}
}

func TestUserDB_Resolve_TTLExpiry(t *testing.T) {
	var calls atomic.Int64
	ttl := 10 * time.Millisecond
	db := NewUserDB(makeFetcher(passwdFixture, groupFixture, nil, nil, &calls), ttl)

	_, _ = db.Resolve(context.Background(), "node1")
	before := calls.Load()

	time.Sleep(20 * time.Millisecond)

	_, err := db.Resolve(context.Background(), "node1")
	if err != nil {
		t.Fatalf("after TTL expiry: %v", err)
	}
	after := calls.Load()

	if after <= before {
		t.Errorf("expected refetch after TTL expiry; calls before=%d after=%d", before, after)
	}
}

func TestUserDB_Resolve_DifferentNodesIndependent(t *testing.T) {
	var calls atomic.Int64
	db := NewUserDB(makeFetcher(passwdFixture, groupFixture, nil, nil, &calls), 5*time.Minute)

	_, _ = db.Resolve(context.Background(), "nodeA")
	afterA := calls.Load()

	_, _ = db.Resolve(context.Background(), "nodeB")
	afterB := calls.Load()

	if afterB <= afterA {
		t.Errorf("nodeB should trigger its own fetch; calls after A=%d after B=%d", afterA, afterB)
	}

	// Second resolve for nodeA should still be cached.
	_, _ = db.Resolve(context.Background(), "nodeA")
	afterA2 := calls.Load()
	if afterA2 != afterB {
		t.Errorf("nodeA second resolve should be cached; calls after B=%d after A2=%d", afterB, afterA2)
	}
}
