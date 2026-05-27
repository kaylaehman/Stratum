package permissions

import (
	"strings"
	"testing"
)

func TestMismatchClassification(t *testing.T) {
	host := map[int]string{0: "root", 1000: "kayla", 33: "www-data"}
	ctr := map[int]string{0: "root", 1000: "node", 999: "app"}
	rows := Mismatch(host, ctr)

	byID := map[int]Row{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	if byID[0].Class != ClassMatch {
		t.Errorf("uid 0 should be match, got %s", byID[0].Class)
	}
	if byID[1000].Class != ClassMismatch { // same uid, different name — the dangerous case
		t.Errorf("uid 1000 should be mismatch, got %s", byID[1000].Class)
	}
	if byID[33].Class != ClassUnresolvable || byID[33].OnContainer {
		t.Errorf("uid 33 host-only should be unresolvable, got %+v", byID[33])
	}
	if byID[999].Class != ClassUnresolvable || byID[999].OnHost {
		t.Errorf("uid 999 container-only should be unresolvable, got %+v", byID[999])
	}
}

func TestEffectiveIdentity(t *testing.T) {
	passwd := []PasswdEntry{{Name: "root", UID: 0, GID: 0}, {Name: "node", UID: 1000, GID: 1000}, {Name: "app", UID: 999, GID: 999}}
	group := []GroupEntry{
		{Name: "root", GID: 0},
		{Name: "node", GID: 1000},
		{Name: "media", GID: 44, Members: []string{"node", "app"}},
		{Name: "docker", GID: 998, Members: []string{"node"}},
	}

	cases := []struct {
		configUser string
		wantUID    int
		wantGID    int
		wantRoot   bool
		wantSupp   []int
	}{
		{"", 0, 0, true, nil},
		{"node", 1000, 1000, false, []int{44, 998}},
		{"1000", 1000, 1000, false, []int{44, 998}},
		{"1000:1000", 1000, 1000, false, []int{44, 998}},
		{"app:media", 999, 44, false, []int{44}}, // app is in media(44) and that's the primary => supp excludes primary... see note
		{"unknownuser", 0, 0, true, nil},          // unknown name -> uid stays 0 (root-ish); acceptable
	}
	for _, c := range cases {
		id := EffectiveIdentity(c.configUser, passwd, group)
		if id.UID != c.wantUID || id.GID != c.wantGID || id.IsRoot != c.wantRoot {
			t.Errorf("EffectiveIdentity(%q) = uid %d gid %d root %v; want %d %d %v",
				c.configUser, id.UID, id.GID, id.IsRoot, c.wantUID, c.wantGID, c.wantRoot)
		}
	}
}

func TestFileAnalysisDACTruthTable(t *testing.T) {
	hostUIDs := map[int]string{1000: "kayla"}
	hostGIDs := map[int]string{1000: "kayla", 44: "media"}
	ctrUIDs := map[int]string{999: "node"}

	// File owned by uid 1000, gid 44, mode 0640 (owner rw, group r, other none).
	file := FileFacts{UID: 1000, GID: 44, ModeOctal: "0640"}

	t.Run("owner_match_rw", func(t *testing.T) {
		v := FileAnalysis(file, Identity{UID: 1000, GID: 1000}, hostUIDs, hostGIDs, ctrUIDs)
		if v.Category != "owner" || !v.ReadGranted || !v.WriteGranted || v.ExecGranted {
			t.Errorf("owner 0640 = %+v; want owner rw, no exec", v)
		}
	})

	t.Run("group_match_read_only_NOT_false_denial", func(t *testing.T) {
		// process gid != file gid, but file gid (44) is in supplementary groups.
		v := FileAnalysis(file, Identity{UID: 999, GID: 999, SupplementaryGIDs: []int{44}}, hostUIDs, hostGIDs, ctrUIDs)
		if v.Category != "group" || !v.ReadGranted {
			t.Errorf("group-member on 0640 should READ (regression: false denial), got %+v", v)
		}
		if v.WriteGranted {
			t.Error("group on 0640 must NOT write (group has only r)")
		}
	})

	t.Run("other_denied", func(t *testing.T) {
		v := FileAnalysis(file, Identity{UID: 999, GID: 999}, hostUIDs, hostGIDs, ctrUIDs)
		if v.Category != "other" || v.ReadGranted || v.WriteGranted {
			t.Errorf("other on 0640 must be denied, got %+v", v)
		}
	})

	t.Run("root_override", func(t *testing.T) {
		v := FileAnalysis(file, Identity{UID: 0, IsRoot: true}, hostUIDs, hostGIDs, ctrUIDs)
		if !v.RootOverride || !v.ReadGranted || !v.WriteGranted {
			t.Errorf("root must be granted rw regardless of bits, got %+v", v)
		}
		if v.ExecGranted {
			t.Error("root exec on a 0640 file (no x bits) should be false")
		}
	})

	t.Run("root_exec_when_any_x_bit", func(t *testing.T) {
		exe := FileFacts{UID: 1000, GID: 44, ModeOctal: "0750"}
		v := FileAnalysis(exe, Identity{UID: 0, IsRoot: true}, hostUIDs, hostGIDs, ctrUIDs)
		if !v.ExecGranted {
			t.Error("root should exec a file with an x bit")
		}
	})

	t.Run("other_with_world_read", func(t *testing.T) {
		f := FileFacts{UID: 1000, GID: 44, ModeOctal: "0644"}
		v := FileAnalysis(f, Identity{UID: 999, GID: 999}, hostUIDs, hostGIDs, ctrUIDs)
		if v.Category != "other" || !v.ReadGranted || v.WriteGranted {
			t.Errorf("other on 0644 should read, not write, got %+v", v)
		}
	})
}

func TestParsePasswdGroupFull(t *testing.T) {
	pw := ParsePasswdFull(strings.NewReader("root:x:0:0:root:/root:/bin/sh\nnode:x:1000:1000::/home/node:/bin/sh\n# comment\nbad\n"))
	if len(pw) != 2 || pw[1].Name != "node" || pw[1].UID != 1000 || pw[1].GID != 1000 {
		t.Fatalf("ParsePasswdFull = %+v", pw)
	}
	gr := ParseGroupFull(strings.NewReader("media:x:44:node,app\nempty:x:50:\n"))
	if len(gr) != 2 || gr[0].GID != 44 || len(gr[0].Members) != 2 || gr[1].Members != nil {
		t.Fatalf("ParseGroupFull = %+v", gr)
	}
}
