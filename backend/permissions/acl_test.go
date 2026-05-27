package permissions

import (
	"context"
	"errors"
	"testing"
)

func TestParseGetfacl(t *testing.T) {
	out := `user::rw-
user:1000:r--
group::r--
group:44:rwx		#effective:r--
mask::r--
other::---
default:user::rwx
`
	entries := ParseGetfacl(out)
	if len(entries) != 7 {
		t.Fatalf("got %d entries, want 7: %+v", len(entries), entries)
	}
	// Named user
	if entries[1].Tag != "user" || entries[1].Qualifier != "1000" || entries[1].Perms != "r--" {
		t.Errorf("named user entry wrong: %+v", entries[1])
	}
	// Effective annotation stripped
	if entries[3].Tag != "group" || entries[3].Qualifier != "44" || entries[3].Perms != "rwx" {
		t.Errorf("effective annotation not stripped: %+v", entries[3])
	}
	// Mask
	if entries[4].Tag != "mask" || entries[4].Perms != "r--" {
		t.Errorf("mask entry wrong: %+v", entries[4])
	}
	// Default
	if !entries[6].IsDefault || entries[6].Tag != "user" {
		t.Errorf("default entry wrong: %+v", entries[6])
	}
}

func TestGetACLAvailable(t *testing.T) {
	exec := func(_ context.Context, _, cmd string, args ...string) (string, error) {
		// Assert the -- terminator is passed before the path.
		if cmd != "getfacl" || len(args) < 3 || args[1] != "--" {
			t.Errorf("unexpected exec call: %s %v", cmd, args)
		}
		return "user::rw-\nother::---\n", nil
	}
	res := GetACL(context.Background(), "n1", "/etc/hosts", exec)
	if !res.Available || len(res.Entries) != 2 {
		t.Fatalf("got %+v", res)
	}
}

func TestGetACLUnavailable(t *testing.T) {
	exec := func(_ context.Context, _, _ string, _ ...string) (string, error) {
		return "", errors.New("getfacl: command not found")
	}
	res := GetACL(context.Background(), "n1", "/etc/hosts", exec)
	if res.Available {
		t.Error("missing getfacl should yield Available:false, not an error")
	}
}
