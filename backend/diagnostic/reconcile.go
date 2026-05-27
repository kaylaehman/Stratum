// Package diagnostic is the "Why is this broken?" orchestration + narrative
// layer. It consumes SP4's mode-bit Verdict, reconciles POSIX ACLs over it
// (ACLs override mode bits), and produces typed steps + suggested fixes.
package diagnostic

import (
	"strconv"

	"github.com/kaylaehman/stratum/backend/permissions"
)

// EffectiveAccess is the FINAL, ACL-reconciled access decision. The narrative
// and fixes consume this, never the raw mode-bit Verdict.
type EffectiveAccess struct {
	Read       bool   `json:"read"`
	Write      bool   `json:"write"`
	Exec       bool   `json:"exec"`
	DecidedBy  string `json:"decided_by"` // root | mode | owner | named_acl | group_acl | other
	Category   string `json:"category"`
	Confidence string `json:"confidence"` // high | low
}

func permBits(perms string) (r, w, x bool) {
	// perms like "rwx", "r--", "rw-".
	if len(perms) >= 3 {
		return perms[0] == 'r', perms[1] == 'w', perms[2] == 'x'
	}
	return false, false, false
}

func andMask(r, w, x, mr, mw, mx bool) (bool, bool, bool) {
	return r && mr, w && mw, x && mx
}

func findEntry(acls []permissions.ACLEntry, tag, qualifier string) (permissions.ACLEntry, bool) {
	for _, e := range acls {
		if e.IsDefault {
			continue // default ACLs apply to NEW children, not access decisions
		}
		if e.Tag == tag && e.Qualifier == qualifier {
			return e, true
		}
	}
	return permissions.ACLEntry{}, false
}

// Reconcile computes the true effective access by applying POSIX ACL semantics
// over SP4's base Verdict. The ACL access-check order is: owner -> named user ->
// owning/named groups (any match grants) -> other; the mask caps named users
// and all groups (but not the owning user or other).
func Reconcile(v permissions.Verdict, acl permissions.ACLResult) EffectiveAccess {
	ea := EffectiveAccess{Category: v.Category, Confidence: "high"}

	// Root override is absolute.
	if v.RootOverride {
		return EffectiveAccess{Read: true, Write: true, Exec: v.ExecGranted, DecidedBy: "root", Category: "owner(root)", Confidence: "high"}
	}
	if v.Category == "unknown" {
		return EffectiveAccess{DecidedBy: "mode", Category: "unknown", Confidence: "low"}
	}

	// No usable ACL -> the mode-bit verdict stands.
	if !acl.Available || len(acl.Entries) == 0 {
		ea.Read, ea.Write, ea.Exec = v.ReadGranted, v.WriteGranted, v.ExecGranted
		ea.DecidedBy = "mode"
		return ea
	}

	// Mask (caps named users + groups). Absent mask => no cap (all true).
	mr, mw, mx := true, true, true
	if m, ok := findEntry(acl.Entries, "mask", ""); ok {
		mr, mw, mx = permBits(m.Perms)
	}

	// 1. Owning user (unaffected by mask).
	if v.EffUID == v.FileUID {
		if e, ok := findEntry(acl.Entries, "user", ""); ok {
			ea.Read, ea.Write, ea.Exec = permBits(e.Perms)
			ea.DecidedBy, ea.Category = "owner", "owner"
			return ea
		}
	}

	// 2. Named user (masked).
	if e, ok := findNamedUser(acl.Entries, v.EffUID); ok {
		r, w, x := permBits(e.Perms)
		ea.Read, ea.Write, ea.Exec = andMask(r, w, x, mr, mw, mx)
		ea.DecidedBy, ea.Category = "named_acl", "named-user-acl"
		return ea
	}

	// 3. Owning + named groups (masked). POSIX: if ANY matching group entry
	// grants the requested permission, it is allowed.
	gids := append([]int{v.EffGID}, v.SupplementaryGIDs...)
	matchedGroup := false
	var gr, gw, gx bool
	apply := func(perms string) {
		r, w, x := permBits(perms)
		amr, amw, amx := andMask(r, w, x, mr, mw, mx)
		gr, gw, gx = gr || amr, gw || amw, gx || amx
		matchedGroup = true
	}
	// Owning group:: applies when one of the process's gids equals the file's gid.
	if containsInt(gids, v.FileGID) {
		if e, ok := findEntry(acl.Entries, "group", ""); ok {
			apply(e.Perms)
		}
	}
	for _, gid := range gids {
		if e, ok := findNamedGroup(acl.Entries, gid); ok {
			apply(e.Perms)
		}
	}
	if matchedGroup {
		ea.Read, ea.Write, ea.Exec = gr, gw, gx
		ea.DecidedBy, ea.Category = "group_acl", "group"
		return ea
	}

	// 4. Other (unaffected by mask).
	if e, ok := findEntry(acl.Entries, "other", ""); ok {
		ea.Read, ea.Write, ea.Exec = permBits(e.Perms)
	}
	ea.DecidedBy, ea.Category = "other", "other"
	return ea
}

func findNamedUser(acls []permissions.ACLEntry, uid int) (permissions.ACLEntry, bool) {
	return findEntry(acls, "user", strconv.Itoa(uid))
}
func findNamedGroup(acls []permissions.ACLEntry, gid int) (permissions.ACLEntry, bool) {
	return findEntry(acls, "group", strconv.Itoa(gid))
}

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
