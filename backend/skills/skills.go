// Package skills loads and indexes the container-troubleshooting skill library
// (the YAML files under assets/skills/). Each skill describes how to recognise a
// container by image/port and a set of common issues with step-by-step
// check/fix procedures. The library is read-only reference data: it is loaded
// once at startup from a configurable directory and served to the AI assistant
// (as context) and a read-only API.
package skills

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// ContainerMatch is how a skill recognises the container it applies to.
type ContainerMatch struct {
	ImagePatterns []string `yaml:"image_patterns" json:"image_patterns"`
	PortHints     []int    `yaml:"port_hints" json:"port_hints"`
}

// Step is a single check or fix action within a common issue.
type Step struct {
	ID              string `yaml:"id" json:"id"`
	Description     string `yaml:"description" json:"description"`
	Type            string `yaml:"type" json:"type"` // check|fix|inform
	Command         string `yaml:"command" json:"command"`
	ExpectedOutput  string `yaml:"expected_output" json:"expected_output,omitempty"`
	RequiresApprove bool   `yaml:"requires_approval" json:"requires_approval"`
	OnFail          string `yaml:"on_fail" json:"on_fail,omitempty"`
	OnSuccess       string `yaml:"on_success" json:"on_success,omitempty"`
}

// TriggerCondition pairs a log pattern (regex-ish substring) with an issue.
type TriggerCondition struct {
	LogPattern string `yaml:"log_pattern" json:"log_pattern"`
}

// CommonIssue is one recognised problem the skill can diagnose/fix.
type CommonIssue struct {
	ID                string             `yaml:"id" json:"id"`
	Name              string             `yaml:"name" json:"name"`
	Symptoms          []string           `yaml:"symptoms" json:"symptoms"`
	TriggerConditions []TriggerCondition `yaml:"trigger_conditions" json:"trigger_conditions,omitempty"`
	Steps             []Step             `yaml:"steps" json:"steps"`
}

// Source distinguishes the read-only shipped library from user-authored skills.
const (
	SourceBuiltin = "builtin"
	SourceCustom  = "custom"
)

// Skill is one parsed skill YAML file.
type Skill struct {
	ID             string         `yaml:"id" json:"id"`
	Name           string         `yaml:"name" json:"name"`
	Version        string         `yaml:"version" json:"version"`
	Category       string         `yaml:"category" json:"category"`
	Description    string         `yaml:"description" json:"description"`
	DocsURL        string         `yaml:"docs_url" json:"docs_url"`
	ContainerMatch ContainerMatch `yaml:"container_match" json:"container_match"`
	CommonIssues   []CommonIssue  `yaml:"common_issues" json:"common_issues"`

	// Source is "builtin" (shipped, read-only) or "custom" (user-authored,
	// editable). It is set by the loader, not parsed from the YAML.
	Source string `yaml:"-" json:"source"`
}

// Parse decodes a single skill from YAML and validates the minimum fields a
// usable skill needs (an id and a name). The returned skill's Source is left
// empty for the caller to set. It is the validation entry point for
// user-authored skills.
func Parse(data []byte) (Skill, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return Skill{}, fmt.Errorf("skills: empty document")
	}
	var s Skill
	if err := yaml.Unmarshal(data, &s); err != nil {
		return Skill{}, fmt.Errorf("skills: invalid YAML: %w", err)
	}
	s.ID = strings.TrimSpace(s.ID)
	if s.ID == "" {
		return Skill{}, fmt.Errorf("skills: missing required field: id")
	}
	if strings.TrimSpace(s.Name) == "" {
		return Skill{}, fmt.Errorf("skills: missing required field: name")
	}
	s.Source = ""
	return s, nil
}

// Marshal renders a skill back to YAML (used to seed the editor when cloning a
// built-in skill into a new custom one). The synthetic Source field is excluded
// via its yaml:"-" tag.
func Marshal(s Skill) ([]byte, error) {
	return yaml.Marshal(s)
}

// Library is an in-memory, read-only index of loaded skills.
type Library struct {
	mu     sync.RWMutex
	byID   map[string]Skill
	sorted []Skill // stable: category, then name
}

// New returns an empty library.
func New() *Library {
	return &Library{byID: make(map[string]Skill)}
}

// Load reads every *.yaml under dir (recursive), parses each into a Skill, and
// returns the indexed library. Loading is graceful: a missing or empty dir
// yields an empty library and no error. A file that fails to parse is logged and
// skipped rather than aborting the whole load.
func Load(dir string) (*Library, error) {
	lib := New()
	if dir == "" {
		slog.Debug("skills: no directory configured; skill library empty")
		return lib, nil
	}
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		slog.Info("skills: directory not found; skill library empty", "dir", dir)
		return lib, nil
	}
	if err != nil {
		return nil, fmt.Errorf("skills: stat %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skills: %q is not a directory", dir)
	}

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
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
			slog.Warn("skills: read failed; skipping", "path", path, "error", err)
			return nil
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			return nil
		}
		var s Skill
		if err := yaml.Unmarshal(data, &s); err != nil {
			slog.Warn("skills: parse failed; skipping", "path", path, "error", err)
			return nil
		}
		if s.ID == "" {
			slog.Warn("skills: file has no id; skipping", "path", path)
			return nil
		}
		s.Source = SourceBuiltin
		lib.byID[s.ID] = s
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("skills: walk %q: %w", dir, walkErr)
	}

	lib.reindex()
	slog.Info("skills: loaded library", "count", len(lib.sorted), "dir", dir)
	return lib, nil
}

// reindex rebuilds the stable-sorted slice from byID. Caller need not hold the
// lock during Load (single-threaded), but it's cheap to be safe.
func (l *Library) reindex() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sorted = l.sorted[:0]
	for _, s := range l.byID {
		l.sorted = append(l.sorted, s)
	}
	sort.SliceStable(l.sorted, func(i, j int) bool {
		a, b := l.sorted[i], l.sorted[j]
		if a.Category != b.Category {
			return a.Category < b.Category
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.ID < b.ID
	})
}

// List returns all skills, stable-sorted by category then name.
func (l *Library) List() []Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Skill, len(l.sorted))
	copy(out, l.sorted)
	return out
}

// Len reports how many skills are loaded.
func (l *Library) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.sorted)
}

// Get returns the skill with the given id.
func (l *Library) Get(id string) (Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s, ok := l.byID[id]
	return s, ok
}

// Upsert adds or replaces a skill in the live index and reindexes. Used to merge
// user-authored skills (Source should be SourceCustom) into the library at
// startup and whenever one is created/edited via the API.
func (l *Library) Upsert(s Skill) {
	l.mu.Lock()
	l.byID[s.ID] = s
	l.mu.Unlock()
	l.reindex()
}

// Remove deletes a skill from the live index, returning the removed skill and
// whether it existed.
func (l *Library) Remove(id string) (Skill, bool) {
	l.mu.Lock()
	s, ok := l.byID[id]
	if ok {
		delete(l.byID, id)
	}
	l.mu.Unlock()
	if ok {
		l.reindex()
	}
	return s, ok
}

// MatchByImage returns every skill whose any image_pattern is a (case-insensitive)
// substring of image, e.g. image "flowiseai/flowise:latest" matches the pattern
// "flowiseai/flowise". Results are returned in the library's stable order
// (category, then name) so the output is deterministic.
func (l *Library) MatchByImage(image string) []Skill {
	if strings.TrimSpace(image) == "" {
		return nil
	}
	lower := strings.ToLower(image)
	l.mu.RLock()
	defer l.mu.RUnlock()
	var out []Skill
	for _, s := range l.sorted { // iterate sorted slice => deterministic order
		for _, pat := range s.ContainerMatch.ImagePatterns {
			pat = strings.TrimSpace(pat)
			if pat == "" {
				continue
			}
			if strings.Contains(lower, strings.ToLower(pat)) {
				out = append(out, s)
				break
			}
		}
	}
	return out
}
