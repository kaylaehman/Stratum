package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/KAE-Labs/stratum/backend/skills"
)

// skillSummary is the list-view projection of a skill (no issue/step bodies).
type skillSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	Description   string   `json:"description"`
	DocsURL       string   `json:"docs_url"`
	ImagePatterns []string `json:"image_patterns"`
	PortHints     []int    `json:"port_hints"`
	IssueCount    int      `json:"issue_count"`
	Source        string   `json:"source"`   // "builtin" | "custom"
	Editable      bool     `json:"editable"` // true for user-authored (custom) skills
}

// skillStep is the detail projection of a single step.
type skillStep struct {
	ID              string `json:"id"`
	Description     string `json:"description"`
	Type            string `json:"type"`
	Command         string `json:"command"`
	RequiresApprove bool   `json:"requires_approval"`
}

// skillIssue is the detail projection of a common issue.
type skillIssue struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Symptoms []string    `json:"symptoms"`
	Steps    []skillStep `json:"steps"`
}

// skillDetail is the full-view projection of a skill.
type skillDetail struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Version       string       `json:"version"`
	Category      string       `json:"category"`
	Description   string       `json:"description"`
	DocsURL       string       `json:"docs_url"`
	ImagePatterns []string     `json:"image_patterns"`
	PortHints     []int        `json:"port_hints"`
	CommonIssues  []skillIssue `json:"common_issues"`
	Source        string       `json:"source"`
	Editable      bool         `json:"editable"`
}

func summarize(s skills.Skill) skillSummary {
	return skillSummary{
		ID:            s.ID,
		Name:          s.Name,
		Category:      s.Category,
		Description:   s.Description,
		DocsURL:       s.DocsURL,
		ImagePatterns: nonNilStrings(s.ContainerMatch.ImagePatterns),
		PortHints:     nonNilInts(s.ContainerMatch.PortHints),
		IssueCount:    len(s.CommonIssues),
		Source:        s.Source,
		Editable:      s.Source == skills.SourceCustom,
	}
}

func detail(s skills.Skill) skillDetail {
	issues := make([]skillIssue, 0, len(s.CommonIssues))
	for _, iss := range s.CommonIssues {
		steps := make([]skillStep, 0, len(iss.Steps))
		for _, st := range iss.Steps {
			steps = append(steps, skillStep{
				ID:              st.ID,
				Description:     st.Description,
				Type:            st.Type,
				Command:         st.Command,
				RequiresApprove: st.RequiresApprove,
			})
		}
		issues = append(issues, skillIssue{
			ID:       iss.ID,
			Name:     iss.Name,
			Symptoms: nonNilStrings(iss.Symptoms),
			Steps:    steps,
		})
	}
	return skillDetail{
		ID:            s.ID,
		Name:          s.Name,
		Version:       s.Version,
		Category:      s.Category,
		Description:   s.Description,
		DocsURL:       s.DocsURL,
		ImagePatterns: nonNilStrings(s.ContainerMatch.ImagePatterns),
		PortHints:     nonNilInts(s.ContainerMatch.PortHints),
		CommonIssues:  issues,
		Source:        s.Source,
		Editable:      s.Source == skills.SourceCustom,
	}
}

// ListSkills returns summaries of all loaded skills (authenticated; reference
// data, any role). An empty library yields an empty list, not an error.
func (h *Handlers) ListSkills(w http.ResponseWriter, r *http.Request) {
	var out []skillSummary
	if h.Skills != nil {
		for _, s := range h.Skills.List() {
			out = append(out, summarize(s))
		}
	}
	if out == nil {
		out = []skillSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": out})
}

// GetSkill returns the full skill by id (404 if unknown).
func (h *Handlers) GetSkill(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if h.Skills == nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	s, ok := h.Skills.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	writeJSON(w, http.StatusOK, detail(s))
}

// nonNilStrings returns s, or an empty (non-nil) slice so JSON encodes [] not null.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// nonNilInts returns s, or an empty (non-nil) slice so JSON encodes [] not null.
func nonNilInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return s
}
