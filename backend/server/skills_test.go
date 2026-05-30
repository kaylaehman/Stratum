package server_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type skillSummaryResp struct {
	Skills []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Category      string `json:"category"`
		Description   string `json:"description"`
		DocsURL       string `json:"docs_url"`
		ImagePatterns []string `json:"image_patterns"`
		PortHints     []int    `json:"port_hints"`
		IssueCount    int      `json:"issue_count"`
	} `json:"skills"`
}

type skillDetailResp struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Category      string   `json:"category"`
	ImagePatterns []string `json:"image_patterns"`
	PortHints     []int    `json:"port_hints"`
	CommonIssues  []struct {
		ID       string   `json:"id"`
		Name     string   `json:"name"`
		Symptoms []string `json:"symptoms"`
		Steps    []struct {
			ID              string `json:"id"`
			Description     string `json:"description"`
			Type            string `json:"type"`
			Command         string `json:"command"`
			RequiresApprove bool   `json:"requires_approval"`
		} `json:"steps"`
	} `json:"common_issues"`
}

func TestListSkills(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/skills", token, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body skillSummaryResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Skills) == 0 {
		t.Fatal("expected skills loaded from assets/skills, got none")
	}
	// Find flowise and sanity-check the summary projection.
	var found bool
	for _, s := range body.Skills {
		if s.ID == "flowise" {
			found = true
			if s.Category != "ai" {
				t.Errorf("flowise category = %q", s.Category)
			}
			if s.IssueCount == 0 {
				t.Errorf("flowise issue_count = 0")
			}
			if len(s.ImagePatterns) == 0 {
				t.Errorf("flowise image_patterns empty")
			}
		}
	}
	if !found {
		t.Error("flowise not present in skills list")
	}
}

func TestGetSkillDetail(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/skills/flowise", token, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body skillDetailResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ID != "flowise" {
		t.Fatalf("id = %q", body.ID)
	}
	if len(body.CommonIssues) == 0 {
		t.Fatal("expected common_issues in detail")
	}
	if len(body.CommonIssues[0].Steps) == 0 {
		t.Fatal("expected steps in first issue")
	}
}

func TestGetSkillNotFound(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Do(authReq(t, http.MethodGet, srv.URL+"/api/skills/does-not-exist", token, nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSkillsRequireAuth(t *testing.T) {
	srv, _ := newNodeTestServer(t)
	c := &http.Client{}
	resp, err := c.Get(srv.URL + "/api/skills")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated /api/skills = %d, want 401", resp.StatusCode)
	}
}
