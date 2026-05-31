package skills

// TestValidateCatalog loads every *.yaml under assets/skills/ (the shipped
// catalog) and runs Validate against the full set plus per-file id-vs-filename
// checks.  The test fails on the first batch of violations and prints them all,
// so contributors can fix everything in one pass.
//
// To run locally:
//
//	cd backend && GOWORK=off go test ./skills/... -run TestValidateCatalog -v

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// catalogDir is relative to the package source directory (backend/skills/).
const catalogDir = "../../assets/skills"

func TestValidateCatalog(t *testing.T) {
	absDir, err := filepath.Abs(catalogDir)
	if err != nil {
		t.Fatalf("resolve catalog dir: %v", err)
	}

	info, err := os.Stat(absDir)
	if os.IsNotExist(err) {
		t.Skipf("catalog directory not found at %s; skipping", absDir)
	}
	if err != nil {
		t.Fatalf("stat catalog dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", absDir)
	}

	// Load all skills, collecting parse errors and per-file id-vs-filename errors.
	var catalog []Skill
	var allErrs []error

	walkErr := filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("read %s: %w", path, err))
			return nil
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			return nil // skip empty files
		}
		var sk Skill
		if err := yaml.Unmarshal(data, &sk); err != nil {
			allErrs = append(allErrs, fmt.Errorf("parse %s: %w", path, err))
			return nil
		}
		// id-vs-filename check.
		allErrs = append(allErrs, idMatchesFilename(sk, path)...)
		catalog = append(catalog, sk)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk catalog: %v", walkErr)
	}

	// Full catalog-level validation (uniqueness, categories, steps, etc.).
	allErrs = append(allErrs, Validate(catalog)...)

	if len(allErrs) == 0 {
		t.Logf("catalog OK: %d skills validated with 0 violations", len(catalog))
		return
	}

	t.Errorf("%d violation(s) found in catalog (%d skills):", len(allErrs), len(catalog))
	for _, e := range allErrs {
		t.Errorf("  - %v", e)
	}
}

// idMatchesFilename returns an error if skill.ID does not match the filename stem.
func idMatchesFilename(sk Skill, path string) []error {
	base := filepath.Base(path)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if sk.ID != stem {
		return []error{fmt.Errorf("file %s: id %q does not match filename stem %q", base, sk.ID, stem)}
	}
	return nil
}
