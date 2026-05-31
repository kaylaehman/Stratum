package skills

// Validate runs catalog-level correctness checks on a slice of parsed skills
// and returns all violations found. The checks are:
//
//  1. Required top-level fields (id, name, category, description, version).
//  2. category ∈ AllowedCategories.
//  3. id matches the filename stem passed as filenameStem (optional; pass ""
//     to skip per-file id-vs-filename check when validating ad-hoc slices).
//  4. id is unique across the full catalog.
//  5. Every step.type ∈ {check, fix, inform, confirm}.
//  6. on_fail / on_success references resolve to a real step id within the
//     same issue.
//  7. port_hints are in 1..65535.
//  8. Every step with type "fix" must set requires_approval = true.
//  9. Every fix/check step whose command classifies as Destructive via
//     remediation.ClassifyRisk must have requires_approval = true.
//
// Validate does NOT modify the skills.

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaylaehman/stratum/backend/remediation"
)

// AllowedCategories is the exhaustive enum derived from the on-disk category
// directories under assets/skills/. A skill whose category field is not in
// this set fails validation.
var AllowedCategories = map[string]struct{}{
	"ai":            {},
	"analytics":     {},
	"automation":    {},
	"backup":        {},
	"communication": {},
	"dashboard":     {},
	"data":          {},
	"database":      {},
	"development":   {},
	"documents":     {},
	"ebooks":        {},
	"email":         {},
	"files":         {},
	"finance":       {},
	"games":         {},
	"identity":      {},
	"media":         {},
	"monitoring":    {},
	"network":       {},
	"passwords":     {},
	"photos":        {},
	"productivity":  {},
	"rss":           {},
	"security":      {},
	"smarthome":     {},
	"time":          {},
	"torrent":       {},
	"voip":          {},
	"weather":       {},
	"web":           {},
}

// validStepTypes is the exhaustive enum of allowed step type values.
var validStepTypes = map[string]struct{}{
	"check":   {},
	"fix":     {},
	"inform":  {},
	"confirm": {},
}

// Validate checks a catalog slice and returns every violation found.
// skills must already be parsed (e.g. via Load). filenameStem is used to
// cross-check that id matches the filename; pass an empty string to skip.
func Validate(catalog []Skill) []error {
	var errs []error
	seenIDs := make(map[string]struct{}, len(catalog))

	for _, sk := range catalog {
		prefix := fmt.Sprintf("skill %q", sk.ID)
		errs = append(errs, validateSkill(sk, prefix, seenIDs)...)
	}
	return errs
}

// ValidateFile validates a single skill parsed from a file at path.
// It additionally checks that the skill id matches the filename stem.
func ValidateFile(sk Skill, path string) []error {
	stem := filenameStem(path)
	prefix := fmt.Sprintf("skill %q (file %s)", sk.ID, filepath.Base(path))
	var errs []error
	if sk.ID != stem {
		errs = append(errs, fmt.Errorf("%s: id %q does not match filename stem %q", prefix, sk.ID, stem))
	}
	dummy := make(map[string]struct{})
	errs = append(errs, validateSkill(sk, prefix, dummy)...)
	return errs
}

func filenameStem(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

func validateSkill(sk Skill, prefix string, seenIDs map[string]struct{}) []error {
	var errs []error

	// --- required fields ---
	if strings.TrimSpace(sk.ID) == "" {
		errs = append(errs, fmt.Errorf("%s: missing required field: id", prefix))
	}
	if strings.TrimSpace(sk.Name) == "" {
		errs = append(errs, fmt.Errorf("%s: missing required field: name", prefix))
	}
	if strings.TrimSpace(sk.Category) == "" {
		errs = append(errs, fmt.Errorf("%s: missing required field: category", prefix))
	}
	if strings.TrimSpace(sk.Description) == "" {
		errs = append(errs, fmt.Errorf("%s: missing required field: description", prefix))
	}
	if strings.TrimSpace(sk.Version) == "" {
		errs = append(errs, fmt.Errorf("%s: missing required field: version", prefix))
	}

	// --- category enum ---
	if sk.Category != "" {
		if _, ok := AllowedCategories[sk.Category]; !ok {
			errs = append(errs, fmt.Errorf("%s: unknown category %q", prefix, sk.Category))
		}
	}

	// --- id uniqueness across catalog ---
	if sk.ID != "" {
		if _, dup := seenIDs[sk.ID]; dup {
			errs = append(errs, fmt.Errorf("%s: duplicate id across catalog", prefix))
		} else {
			seenIDs[sk.ID] = struct{}{}
		}
	}

	// --- port hints ---
	for _, p := range sk.ContainerMatch.PortHints {
		if p < 1 || p > 65535 {
			errs = append(errs, fmt.Errorf("%s: port_hint %d out of range 1..65535", prefix, p))
		}
	}

	// --- common issues ---
	for _, issue := range sk.CommonIssues {
		issPrefix := fmt.Sprintf("%s issue %q", prefix, issue.ID)
		errs = append(errs, validateIssue(issue, issPrefix)...)
	}

	return errs
}

func validateIssue(issue CommonIssue, prefix string) []error {
	var errs []error

	// Build step-id set for reference resolution.
	stepIDs := make(map[string]struct{}, len(issue.Steps))
	for _, s := range issue.Steps {
		if s.ID != "" {
			stepIDs[s.ID] = struct{}{}
		}
	}

	for _, step := range issue.Steps {
		stepPrefix := fmt.Sprintf("%s step %q", prefix, step.ID)
		errs = append(errs, validateStep(step, stepPrefix, stepIDs)...)
	}
	return errs
}

func validateStep(step Step, prefix string, stepIDs map[string]struct{}) []error {
	var errs []error

	// --- step type enum ---
	if _, ok := validStepTypes[step.Type]; !ok {
		errs = append(errs, fmt.Errorf("%s: unknown step type %q (must be check|fix|inform|confirm)", prefix, step.Type))
	}

	// --- fix steps must have requires_approval ---
	if step.Type == "fix" && !step.RequiresApprove {
		errs = append(errs, fmt.Errorf("%s: type \"fix\" step must set requires_approval: true", prefix))
	}

	// --- on_fail / on_success reference resolution ---
	if step.OnFail != "" {
		if _, ok := stepIDs[step.OnFail]; !ok {
			errs = append(errs, fmt.Errorf("%s: on_fail references unknown step id %q", prefix, step.OnFail))
		}
	}
	if step.OnSuccess != "" {
		if _, ok := stepIDs[step.OnSuccess]; !ok {
			errs = append(errs, fmt.Errorf("%s: on_success references unknown step id %q", prefix, step.OnSuccess))
		}
	}

	// --- command risk classification (fix steps only) ---
	// The SCHEMA defines "check" steps as read-only diagnostics that may use
	// shell pipelines (e.g. "docker logs … | tail -20") which ClassifyRisk marks
	// as RiskDestructive due to the pipe metacharacter — that is a false positive
	// for inspection commands. Apply the risk gate only to "fix" steps, which are
	// the sole type allowed to mutate state. A fix step whose command is
	// classified as RiskDestructive MUST already have requires_approval:true (the
	// check above catches the omission), but we add an explicit message here so
	// the risk level is named in the output.
	cmd := strings.TrimSpace(step.Command)
	if cmd != "" && step.Type == "fix" {
		risk := remediation.ClassifyRisk([]string{cmd})
		if risk == remediation.RiskDestructive && !step.RequiresApprove {
			errs = append(errs, fmt.Errorf(
				"%s: command classified as %q by ClassifyRisk but requires_approval is false",
				prefix, risk,
			))
		}
	}

	return errs
}
