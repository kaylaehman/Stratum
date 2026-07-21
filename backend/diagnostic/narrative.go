package diagnostic

import (
	"fmt"

	"github.com/KAE-Labs/stratum/backend/docker"
	"github.com/KAE-Labs/stratum/backend/permissions"
)

// Step status values.
const (
	StatusOK   = "ok"
	StatusWarn = "warn"
	StatusBad  = "bad"
)

// Step is one typed fact in the diagnostic card (frontend renders it).
type Step struct {
	Kind   string `json:"kind"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

// Inputs is everything the narrative/fixes need (assembled by the orchestrator).
type Inputs struct {
	HostPath    string
	FileUID     int
	FileGID     int
	FileMode    string
	HostOwner   string // resolved name or ""
	HostGroup   string
	RunUID      int
	RunGID      int
	RunName     string // container-side name for the run uid, or ""
	IsRoot      bool
	UsernsRemap bool
	Exposure    docker.Exposure
	ACL         permissions.ACLResult
	Effective   EffectiveAccess
}

// Narrative builds the ordered, typed explanation steps.
func Narrative(in Inputs) []Step {
	var steps []Step

	steps = append(steps, Step{
		Kind:   "file_owner",
		Label:  "File ownership & permissions",
		Detail: fmt.Sprintf("%s is owned by UID %d (%s), GID %d (%s), mode %s", in.HostPath, in.FileUID, orUnknown(in.HostOwner), in.FileGID, orUnknown(in.HostGroup), in.FileMode),
		Status: StatusOK,
	})

	runStatus := StatusOK
	runDetail := fmt.Sprintf("container runs as UID %d (%s), GID %d", in.RunUID, orUnknown(in.RunName), in.RunGID)
	if in.IsRoot {
		runDetail = "container runs as root (UID 0)"
		runStatus = StatusWarn
	}
	steps = append(steps, Step{Kind: "run_identity", Label: "Container process identity", Detail: runDetail, Status: runStatus})

	if in.UsernsRemap {
		steps = append(steps, Step{
			Kind: "userns", Label: "User-namespace remapping active",
			Detail: "this container uses userns-remap, so its in-container UID differs from the host UID — the comparison below is low-confidence",
			Status: StatusWarn,
		})
	}

	// Bind-mount exposure (a top root cause when absent).
	switch {
	case !in.Exposure.Exposed:
		steps = append(steps, Step{
			Kind: "bind_mount", Label: "Bind mount", Status: StatusBad,
			Detail: "this host path is NOT mounted into the container — the container cannot see this file at all",
		})
		return steps // short-circuit: permission discussion is moot
	case in.Exposure.IsNamedVolume:
		steps = append(steps, Step{
			Kind: "bind_mount", Label: "Named volume", Status: StatusWarn,
			Detail: fmt.Sprintf("the container mounts named volume %q at %s, not a host bind — host-side permission changes won't apply directly", in.Exposure.VolumeName, in.Exposure.ViaDest),
		})
	default:
		mode := "read-write"
		if !in.Exposure.RW {
			mode = "read-only"
		}
		steps = append(steps, Step{
			Kind: "bind_mount", Label: "Bind mount", Status: StatusOK,
			Detail: fmt.Sprintf("exposed at %s (%s) via host %s", in.Exposure.ContainerPath, mode, in.Exposure.ViaSource),
		})
	}

	steps = append(steps, Step{
		Kind: "category", Label: "Permission category",
		Detail: fmt.Sprintf("the container process falls in the %q category for this file", in.Effective.Category),
		Status: StatusOK,
	})

	if in.ACL.Available && len(in.ACL.Entries) > 0 {
		steps = append(steps, Step{
			Kind: "acl", Label: "POSIX ACLs", Status: StatusOK,
			Detail: fmt.Sprintf("ACLs are present and were reconciled (decided by: %s)", in.Effective.DecidedBy),
		})
	}

	result := StatusBad
	if in.Effective.Read || in.Effective.Write {
		result = StatusOK
	}
	steps = append(steps, Step{
		Kind: "result", Label: "Effective access", Status: result,
		Detail: accessSummary(in.Effective),
	})
	return steps
}

func accessSummary(ea EffectiveAccess) string {
	parts := []string{}
	if ea.Read {
		parts = append(parts, "read")
	}
	if ea.Write {
		parts = append(parts, "write")
	}
	if ea.Exec {
		parts = append(parts, "execute")
	}
	if len(parts) == 0 {
		return "no access (read/write/execute all denied)"
	}
	s := parts[0]
	for i := 1; i < len(parts); i++ {
		s += " + " + parts[i]
	}
	return "granted: " + s
}

func orUnknown(s string) string {
	if s == "" {
		return "name unresolved"
	}
	return s
}
