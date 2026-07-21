package diagnostic

import (
	"fmt"

	appssh "github.com/KAE-Labs/stratum/backend/ssh"
)

// SuggestedFix is a concrete, copy-pasteable remediation. Commands are
// suggest-only in the MVP (never auto-run); the path is shell-quoted so the
// displayed command is safe to copy-paste.
type SuggestedFix struct {
	Command   string `json:"command"`
	Rationale string `json:"rationale"`
	Warning   string `json:"warning,omitempty"`
}

// Fixes produces ordered, least-destructive-first remediations from the
// reconciled access. It never suggests opening an already-accessible file.
func Fixes(in Inputs) []SuggestedFix {
	q := appssh.QuoteArg(in.HostPath)

	// Already accessible (incl. via ACL) -> nothing to do.
	if in.Effective.Read || in.Effective.Write {
		// Only flag a read-only-mount + (separately) inability to write.
		if in.Exposure.Exposed && !in.Exposure.RW {
			return []SuggestedFix{{
				Command:   "# edit your docker-compose volume: change \":ro\" to \":rw\" for " + in.Exposure.ViaSource,
				Rationale: "the file is readable, but the bind mount is read-only — writes require remounting read-write",
			}}
		}
		return nil
	}

	// Not exposed -> add the bind mount (no chmod will help).
	if !in.Exposure.Exposed {
		return []SuggestedFix{{
			Command:   fmt.Sprintf("# in docker-compose.yml, add under volumes:\n#   - %s:%s", in.HostPath, in.HostPath),
			Rationale: "the file is not mounted into the container; no permission change can help until it is exposed via a bind mount",
		}}
	}
	if in.Exposure.IsNamedVolume {
		return []SuggestedFix{{
			Command:   "# manage the file via the named volume " + in.Exposure.VolumeName,
			Rationale: "this path is a named volume, not a host bind — change ownership from inside a container that mounts it, or recreate the volume",
		}}
	}

	// Denied. Offer surgical (setfacl) first, then chmod (with warning), then
	// chown (most invasive, last).
	var fixes []SuggestedFix
	switch in.Effective.Category {
	case "group": // Reconcile emits Category "group" for owning- and named-group ACL matches
		fixes = append(fixes,
			SuggestedFix{
				Command:   fmt.Sprintf("setfacl -m g:%d:r %s", in.RunGID, q),
				Rationale: "grant the container's group read access via a surgical POSIX ACL (no broad permission change)",
			},
			SuggestedFix{
				Command:   "chmod g+r " + q,
				Rationale: "grant the owning group read access",
				Warning:   "widens access for every member of the file's group on the host",
			},
		)
	default: // other / owner-mismatch
		fixes = append(fixes,
			SuggestedFix{
				Command:   fmt.Sprintf("setfacl -m u:%d:r %s", in.RunUID, q),
				Rationale: "grant the container's run UID read access via a surgical POSIX ACL — the least-invasive fix",
			},
			SuggestedFix{
				Command:   "chmod o+r " + q,
				Rationale: "grant read access to all (other) users",
				Warning:   "makes the file world-readable on the host",
			},
			SuggestedFix{
				Command:   fmt.Sprintf("chown %d %s", in.RunUID, q),
				Rationale: "change host-side ownership to the container's run UID",
				Warning:   "changes host ownership and may break host services that rely on the current owner — use as a last resort",
			},
		)
	}
	return fixes
}
