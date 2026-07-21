package diagnostic

import (
	"testing"

	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/permissions"
)

func TestReconcileNoACLPassthrough(t *testing.T) {
	v := permissions.Verdict{ReadGranted: true, WriteGranted: false, Category: "owner"}
	ea := Reconcile(v, permissions.ACLResult{Available: false})
	if !ea.Read || ea.Write || ea.DecidedBy != "mode" {
		t.Fatalf("no-ACL should pass the mode verdict through, got %+v", ea)
	}
}

func TestReconcileRoot(t *testing.T) {
	v := permissions.Verdict{RootOverride: true, ExecGranted: false}
	ea := Reconcile(v, permissions.ACLResult{Available: true, Entries: []permissions.ACLEntry{{Tag: "other", Perms: "---"}}})
	if !ea.Read || !ea.Write || ea.DecidedBy != "root" {
		t.Fatalf("root must be granted regardless of ACL, got %+v", ea)
	}
}

func TestReconcileNamedUserGrantOverridesDeniedMode(t *testing.T) {
	// Mode said "other ---" (denied), but a named-user ACL grants read.
	v := permissions.Verdict{
		FileUID: 1000, FileGID: 1000, EffUID: 999, EffGID: 999,
		Category: "other", ReadGranted: false,
	}
	acl := permissions.ACLResult{Available: true, Entries: []permissions.ACLEntry{
		{Tag: "user", Perms: "rw-"},      // owner
		{Tag: "user", Qualifier: "999", Perms: "rw-"}, // named user = our run uid
		{Tag: "mask", Perms: "r--"},      // mask caps named user to r
		{Tag: "other", Perms: "---"},
	}}
	ea := Reconcile(v, acl)
	if !ea.Read || ea.Write || ea.DecidedBy != "named_acl" {
		t.Fatalf("named-user ACL should grant read (capped by mask), got %+v", ea)
	}
}

func TestReconcileOwnerUnaffectedByMask(t *testing.T) {
	v := permissions.Verdict{FileUID: 1000, EffUID: 1000, Category: "owner"}
	acl := permissions.ACLResult{Available: true, Entries: []permissions.ACLEntry{
		{Tag: "user", Perms: "rwx"}, // owner full
		{Tag: "mask", Perms: "r--"}, // mask must NOT cap the owning user
		{Tag: "other", Perms: "---"},
	}}
	ea := Reconcile(v, acl)
	if !ea.Read || !ea.Write || !ea.Exec || ea.DecidedBy != "owner" {
		t.Fatalf("owning user is unaffected by mask, got %+v", ea)
	}
}

func TestReconcileGroupCappedByMask(t *testing.T) {
	v := permissions.Verdict{FileUID: 1000, FileGID: 44, EffUID: 999, EffGID: 44, Category: "group"}
	acl := permissions.ACLResult{Available: true, Entries: []permissions.ACLEntry{
		{Tag: "user", Perms: "rw-"},
		{Tag: "group", Perms: "rwx"}, // owning group wants rwx
		{Tag: "mask", Perms: "r--"},  // but mask caps to r
		{Tag: "other", Perms: "---"},
	}}
	ea := Reconcile(v, acl)
	if !ea.Read || ea.Write || ea.Exec {
		t.Fatalf("owning group should be capped to r by mask, got %+v", ea)
	}
}

func TestNarrativeNotExposedShortCircuits(t *testing.T) {
	steps := Narrative(Inputs{
		HostPath: "/home/kayla/secret", FileUID: 1000, RunUID: 999,
		Exposure: docker.Exposure{Exposed: false},
	})
	last := steps[len(steps)-1]
	if last.Kind != "bind_mount" || last.Status != StatusBad {
		t.Fatalf("not-exposed should end on a bad bind_mount step, got %+v", steps)
	}
}

func TestFixesAlreadyGrantedNoFix(t *testing.T) {
	in := Inputs{Exposure: docker.Exposure{Exposed: true, RW: true}, Effective: EffectiveAccess{Read: true}}
	if f := Fixes(in); len(f) != 0 {
		t.Fatalf("already-accessible file should have no fix, got %+v", f)
	}
}

func TestFixesDeniedOtherOrdersLeastDestructiveFirst(t *testing.T) {
	in := Inputs{
		HostPath: "/srv/data/f", RunUID: 999,
		Exposure:  docker.Exposure{Exposed: true, RW: true},
		Effective: EffectiveAccess{Read: false, Category: "other"},
	}
	fixes := Fixes(in)
	if len(fixes) != 3 {
		t.Fatalf("expected 3 fixes, got %+v", fixes)
	}
	if fixes[0].Command[:7] != "setfacl" {
		t.Errorf("first fix should be setfacl (surgical), got %q", fixes[0].Command)
	}
	if fixes[2].Command[:5] != "chown" || fixes[2].Warning == "" {
		t.Errorf("chown should be last and carry a warning, got %+v", fixes[2])
	}
}

func TestFixesNotExposedSuggestsMount(t *testing.T) {
	in := Inputs{HostPath: "/x", Exposure: docker.Exposure{Exposed: false}, Effective: EffectiveAccess{Read: false}}
	fixes := Fixes(in)
	if len(fixes) != 1 || fixes[0].Command == "" {
		t.Fatalf("not-exposed should suggest adding a mount, got %+v", fixes)
	}
}
