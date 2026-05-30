package skills

import (
	"path/filepath"
	"testing"
)

func loadTestLib(t *testing.T) *Library {
	t.Helper()
	lib, err := Load(filepath.Join("testdata"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return lib
}

func TestLoadParsesSkills(t *testing.T) {
	lib := loadTestLib(t)
	if got := lib.Len(); got != 2 {
		t.Fatalf("Len = %d, want 2", got)
	}

	fw, ok := lib.Get("flowise")
	if !ok {
		t.Fatal("flowise not loaded")
	}
	if fw.Name != "Flowise LLM Orchestrator" {
		t.Errorf("flowise name = %q", fw.Name)
	}
	if fw.Category != "ai" {
		t.Errorf("flowise category = %q", fw.Category)
	}
	if len(fw.ContainerMatch.PortHints) != 1 || fw.ContainerMatch.PortHints[0] != 3000 {
		t.Errorf("flowise port hints = %v", fw.ContainerMatch.PortHints)
	}
	if len(fw.CommonIssues) != 1 {
		t.Fatalf("flowise issues = %d, want 1", len(fw.CommonIssues))
	}
	iss := fw.CommonIssues[0]
	if len(iss.Steps) != 2 {
		t.Fatalf("issue steps = %d, want 2", len(iss.Steps))
	}
	if iss.Steps[0].Type != "check" {
		t.Errorf("step 0 type = %q", iss.Steps[0].Type)
	}
	if !iss.Steps[1].RequiresApprove {
		t.Errorf("step 1 should require approval")
	}
	if iss.Steps[0].OnFail != "step-2" {
		t.Errorf("step 0 on_fail = %q", iss.Steps[0].OnFail)
	}
}

func TestListStableSorted(t *testing.T) {
	lib := loadTestLib(t)
	list := lib.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d", len(list))
	}
	// ai < media by category.
	if list[0].Category != "ai" || list[1].Category != "media" {
		t.Errorf("unexpected order: %q, %q", list[0].Category, list[1].Category)
	}
}

func TestMatchByImage(t *testing.T) {
	lib := loadTestLib(t)

	// Match: pattern is a substring of the full image ref.
	m := lib.MatchByImage("flowiseai/flowise:latest")
	if len(m) != 1 || m[0].ID != "flowise" {
		t.Fatalf("MatchByImage(flowise) = %v", ids(m))
	}

	// Case-insensitive: uppercase image still matches lowercase pattern.
	m = lib.MatchByImage("FlowiseAI/Flowise:LATEST")
	if len(m) != 1 || m[0].ID != "flowise" {
		t.Fatalf("MatchByImage(uppercase) = %v", ids(m))
	}

	// Second image pattern on a multi-pattern skill matches.
	m = lib.MatchByImage("plexinc/pms-docker:1.40")
	if len(m) != 1 || m[0].ID != "plex" {
		t.Fatalf("MatchByImage(plex alt) = %v", ids(m))
	}

	// No match.
	if m := lib.MatchByImage("nginx:latest"); len(m) != 0 {
		t.Fatalf("MatchByImage(nginx) = %v, want none", ids(m))
	}

	// Empty image yields nothing.
	if m := lib.MatchByImage(""); len(m) != 0 {
		t.Fatalf("MatchByImage(empty) = %v", ids(m))
	}
}

func TestLoadMissingDirGraceful(t *testing.T) {
	lib, err := Load(filepath.Join("testdata", "does-not-exist"))
	if err != nil {
		t.Fatalf("Load(missing): %v", err)
	}
	if lib.Len() != 0 {
		t.Fatalf("Len = %d, want 0", lib.Len())
	}
}

func TestLoadEmptyDirString(t *testing.T) {
	lib, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if lib.Len() != 0 {
		t.Fatalf("Len = %d, want 0", lib.Len())
	}
}

func ids(s []Skill) []string {
	out := make([]string, len(s))
	for i, sk := range s {
		out[i] = sk.ID
	}
	return out
}
