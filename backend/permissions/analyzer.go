package permissions

import (
	"bufio"
	"io"
	"sort"
	"strconv"
	"strings"
)

// PasswdEntry is one /etc/passwd line (name:x:uid:gid:...).
type PasswdEntry struct {
	Name string
	UID  int
	GID  int
}

// GroupEntry is one /etc/group line (name:x:gid:member,member,...).
type GroupEntry struct {
	Name    string
	GID     int
	Members []string
}

// ParsePasswdFull parses /etc/passwd into entries (name, uid, primary gid).
func ParsePasswdFull(r io.Reader) []PasswdEntry {
	var out []PasswdEntry
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ":")
		if len(f) < 4 {
			continue
		}
		uid, err1 := strconv.Atoi(f[2])
		gid, err2 := strconv.Atoi(f[3])
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, PasswdEntry{Name: f[0], UID: uid, GID: gid})
	}
	return out
}

// ParseGroupFull parses /etc/group into entries (name, gid, members).
func ParseGroupFull(r io.Reader) []GroupEntry {
	var out []GroupEntry
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, ":")
		if len(f) < 3 {
			continue
		}
		gid, err := strconv.Atoi(f[2])
		if err != nil {
			continue
		}
		var members []string
		if len(f) >= 4 && f[3] != "" {
			members = strings.Split(f[3], ",")
		}
		out = append(out, GroupEntry{Name: f[0], GID: gid, Members: members})
	}
	return out
}

// Row classes for the host-vs-container comparison table.
const (
	ClassMatch        = "match"        // both sides, same name (green)
	ClassMismatch     = "mismatch"     // both sides, different name (amber — the dangerous case)
	ClassUnresolvable = "unresolvable" // one side only / no name (red)
)

// Row is one UID (or GID) compared across host and container.
type Row struct {
	ID            int    `json:"id"`
	HostName      string `json:"host_name,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
	OnHost        bool   `json:"on_host"`
	OnContainer   bool   `json:"on_container"`
	Class         string `json:"class"`
}

// Mismatch compares two id->name maps (works for both UID and GID tables).
// Classification is numeric-driven; names are display-only.
func Mismatch(host, container map[int]string) []Row {
	ids := map[int]bool{}
	for id := range host {
		ids[id] = true
	}
	for id := range container {
		ids[id] = true
	}
	rows := make([]Row, 0, len(ids))
	for id := range ids {
		hn, hok := host[id]
		cn, cok := container[id]
		row := Row{ID: id, HostName: hn, ContainerName: cn, OnHost: hok, OnContainer: cok}
		switch {
		case hok && cok && hn == cn:
			row.Class = ClassMatch
		case hok && cok:
			row.Class = ClassMismatch
		default:
			row.Class = ClassUnresolvable
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

// Identity is a container's effective run identity.
type Identity struct {
	UID               int
	GID               int
	SupplementaryGIDs []int
	IsRoot            bool
}

// EffectiveIdentity resolves Config.User against the container's passwd/group.
// Forms: "" -> root(0,0); "name"; "uid"; "uid:gid"; "name:group". Supplementary
// GIDs are every group whose member list contains the resolved username.
func EffectiveIdentity(configUser string, passwd []PasswdEntry, group []GroupEntry) Identity {
	id := Identity{}
	userPart, groupPart, hasGroup := strings.Cut(configUser, ":")
	userPart = strings.TrimSpace(userPart)

	username := ""
	switch {
	case userPart == "":
		id.UID = 0 // empty Config.User => root
	default:
		if uid, err := strconv.Atoi(userPart); err == nil {
			id.UID = uid
			if e, ok := findByUID(passwd, uid); ok {
				id.GID = e.GID
				username = e.Name
			}
		} else {
			// a username
			if e, ok := findByName(passwd, userPart); ok {
				id.UID = e.UID
				id.GID = e.GID
				username = e.Name
			}
		}
	}
	if userPart == "" {
		if e, ok := findByUID(passwd, 0); ok {
			username = e.Name
		}
	}

	if hasGroup {
		groupPart = strings.TrimSpace(groupPart)
		if gid, err := strconv.Atoi(groupPart); err == nil {
			id.GID = gid
		} else if g, ok := findGroupByName(group, groupPart); ok {
			id.GID = g.GID
		}
	}

	id.IsRoot = id.UID == 0

	// Supplementary groups: every group whose members include the username.
	if username != "" {
		for _, g := range group {
			for _, m := range g.Members {
				if strings.TrimSpace(m) == username && g.GID != id.GID {
					id.SupplementaryGIDs = append(id.SupplementaryGIDs, g.GID)
					break
				}
			}
		}
	}
	sort.Ints(id.SupplementaryGIDs)
	return id
}

func findByUID(p []PasswdEntry, uid int) (PasswdEntry, bool) {
	for _, e := range p {
		if e.UID == uid {
			return e, true
		}
	}
	return PasswdEntry{}, false
}
func findByName(p []PasswdEntry, name string) (PasswdEntry, bool) {
	for _, e := range p {
		if e.Name == name {
			return e, true
		}
	}
	return PasswdEntry{}, false
}
func findGroupByName(g []GroupEntry, name string) (GroupEntry, bool) {
	for _, e := range g {
		if e.Name == name {
			return e, true
		}
	}
	return GroupEntry{}, false
}

// FileFacts are the host file's ownership/mode (from the SP3 fs.Entry).
type FileFacts struct {
	UID       int
	GID       int
	ModeOctal string // e.g. "0644", "4755"
}

// Verdict is the full structured access decision for a file vs a container's
// effective identity. SP5 turns this into prose + suggested fix.
type Verdict struct {
	FileUID            int    `json:"file_uid"`
	FileGID            int    `json:"file_gid"`
	FileModeOctal      string `json:"file_mode_octal"`
	HostOwnerName      string `json:"host_owner_name,omitempty"`
	HostGroupName      string `json:"host_group_name,omitempty"`
	ContainerOwnerName string `json:"container_owner_name,omitempty"`
	EffUID             int    `json:"eff_uid"`
	EffGID             int    `json:"eff_gid"`
	SupplementaryGIDs  []int  `json:"supplementary_gids,omitempty"`
	ProcessIsRoot      bool   `json:"process_is_root"`
	RootOverride       bool   `json:"root_override"`
	Category           string `json:"category"` // owner | owner(root) | group | other
	ReadGranted        bool   `json:"read_granted"`
	WriteGranted       bool   `json:"write_granted"`
	ExecGranted        bool   `json:"exec_granted"`
	Reason             string `json:"reason"`
}

// FileAnalysis computes the actual UNIX DAC access decision (not a uid==owner
// shortcut): owner -> group(+supplementary) -> other category, the matching rwx
// bits, with a root (UID 0) override.
func FileAnalysis(file FileFacts, id Identity, hostUIDs, hostGIDs, ctrUIDs map[int]string) Verdict {
	perm := parseOctalPerm(file.ModeOctal)
	v := Verdict{
		FileUID: file.UID, FileGID: file.GID, FileModeOctal: file.ModeOctal,
		HostOwnerName: hostUIDs[file.UID], HostGroupName: hostGIDs[file.GID],
		ContainerOwnerName: ctrUIDs[file.UID],
		EffUID:             id.UID, EffGID: id.GID, SupplementaryGIDs: id.SupplementaryGIDs,
		ProcessIsRoot: id.IsRoot,
	}

	var bits int
	switch {
	case id.UID == 0:
		v.RootOverride = true
		v.Category = "owner(root)"
	case id.UID == file.UID:
		v.Category = "owner"
		bits = (perm >> 6) & 7
	case id.GID == file.GID || containsInt(id.SupplementaryGIDs, file.GID):
		v.Category = "group"
		bits = (perm >> 3) & 7
	default:
		v.Category = "other"
		bits = perm & 7
	}

	if v.RootOverride {
		v.ReadGranted = true
		v.WriteGranted = true
		v.ExecGranted = anyExecBit(perm) // root needs at least one x bit to execute
	} else {
		v.ReadGranted = bits&4 != 0
		v.WriteGranted = bits&2 != 0
		v.ExecGranted = bits&1 != 0
	}
	v.Reason = buildReason(v)
	return v
}

func parseOctalPerm(octal string) int {
	n, err := strconv.ParseInt(octal, 8, 32)
	if err != nil {
		return 0
	}
	return int(n) & 0o777 // permission bits only (drop setuid/setgid/sticky)
}

func anyExecBit(perm int) bool { return perm&0o111 != 0 }

func containsInt(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func buildReason(v Verdict) string {
	if v.RootOverride {
		return "container process runs as root (UID 0): read+write granted regardless of permission bits"
	}
	access := []string{}
	if v.ReadGranted {
		access = append(access, "read")
	}
	if v.WriteGranted {
		access = append(access, "write")
	}
	if v.ExecGranted {
		access = append(access, "exec")
	}
	granted := "no access"
	if len(access) > 0 {
		granted = strings.Join(access, "+")
	}
	return "container process falls in the '" + v.Category + "' permission category: " + granted
}
