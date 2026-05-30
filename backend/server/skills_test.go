package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
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

const testCustomSkillYAML = `id: my-app
name: My App
version: "1.0"
category: other
description: A test app.
docs_url: ""
container_match:
  image_patterns:
    - myorg/myapp
  port_hints:
    - 9000
common_issues:
  - id: wont-start
    name: Will not start
    symptoms:
      - container exits immediately
    steps:
      - id: check-logs
        description: Inspect the logs.
        type: check
        command: docker logs {container_name}
        requires_approval: false
`

func doJSON(t *testing.T, c *http.Client, method, url, token string, body any) (*http.Response, []byte) {
	t.Helper()
	resp, err := c.Do(authReq(t, method, url, token, body))
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestCustomSkillLifecycle(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}

	// Create.
	resp, body := doJSON(t, c, http.MethodPost, srv.URL+"/api/skills", token, map[string]string{"yaml": testCustomSkillYAML})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201 (body %s)", resp.StatusCode, body)
	}
	var created struct {
		ID       string `json:"id"`
		Source   string `json:"source"`
		Editable bool   `json:"editable"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID != "my-app" || created.Source != "custom" || !created.Editable {
		t.Fatalf("create projection = %+v", created)
	}

	// Duplicate id is rejected.
	if resp, _ := doJSON(t, c, http.MethodPost, srv.URL+"/api/skills", token, map[string]string{"yaml": testCustomSkillYAML}); resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate create = %d, want 409", resp.StatusCode)
	}

	// It now appears in the list as custom and matches its image.
	resp, body = doJSON(t, c, http.MethodGet, srv.URL+"/api/skills", token, nil)
	var list struct {
		Skills []struct {
			ID     string `json:"id"`
			Source string `json:"source"`
		} `json:"skills"`
	}
	_ = json.Unmarshal(body, &list)
	var sawCustom bool
	for _, s := range list.Skills {
		if s.ID == "my-app" && s.Source == "custom" {
			sawCustom = true
		}
	}
	if !sawCustom {
		t.Fatal("custom skill not present in list with source=custom")
	}

	// Raw fetch returns the stored YAML verbatim.
	resp, body = doJSON(t, c, http.MethodGet, srv.URL+"/api/skills/my-app/raw", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw status = %d", resp.StatusCode)
	}
	var raw struct {
		YAML     string `json:"yaml"`
		Editable bool   `json:"editable"`
	}
	_ = json.Unmarshal(body, &raw)
	if raw.YAML != testCustomSkillYAML || !raw.Editable {
		t.Fatalf("raw yaml mismatch or not editable: editable=%v", raw.Editable)
	}

	// Update (same id, changed description).
	updated := strings.Replace(testCustomSkillYAML, "description: A test app.", "description: An updated test app.", 1)
	if resp, b := doJSON(t, c, http.MethodPut, srv.URL+"/api/skills/my-app", token, map[string]string{"yaml": updated}); resp.StatusCode != http.StatusOK {
		t.Fatalf("update = %d, want 200 (body %s)", resp.StatusCode, b)
	}

	// Editing under a different id in the path is a mismatch.
	if resp, _ := doJSON(t, c, http.MethodPut, srv.URL+"/api/skills/other-id", token, map[string]string{"yaml": updated}); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("update unknown id = %d, want 404", resp.StatusCode)
	}

	// Delete, then it is gone.
	if resp, _ := doJSON(t, c, http.MethodDelete, srv.URL+"/api/skills/my-app", token, nil); resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", resp.StatusCode)
	}
	if resp, _ := doJSON(t, c, http.MethodGet, srv.URL+"/api/skills/my-app", token, nil); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", resp.StatusCode)
	}
}

func TestCustomSkillInvalidYAML(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Missing required id.
	resp, _ := doJSON(t, c, http.MethodPost, srv.URL+"/api/skills", token, map[string]string{"yaml": "name: No ID\n"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid create = %d, want 400", resp.StatusCode)
	}
}

func TestBuiltinSkillIsReadOnly(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// flowise is shipped (built-in); edits and deletes must be refused.
	if resp, _ := doJSON(t, c, http.MethodPut, srv.URL+"/api/skills/flowise", token, map[string]string{"yaml": testCustomSkillYAML}); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("edit builtin = %d, want 403", resp.StatusCode)
	}
	if resp, _ := doJSON(t, c, http.MethodDelete, srv.URL+"/api/skills/flowise", token, nil); resp.StatusCode != http.StatusForbidden {
		t.Fatalf("delete builtin = %d, want 403", resp.StatusCode)
	}
}

func TestGenerateSkillGating(t *testing.T) {
	srv, token := newNodeTestServer(t)
	c := &http.Client{}
	// Neither image nor container → 400.
	if resp, _ := doJSON(t, c, http.MethodPost, srv.URL+"/api/skills/generate", token, map[string]string{}); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("generate without target = %d, want 400", resp.StatusCode)
	}
	// Image given but AI not configured in the test harness → 400 ai_not_configured.
	resp, body := doJSON(t, c, http.MethodPost, srv.URL+"/api/skills/generate", token, map[string]string{"image": "myorg/myapp:latest"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("generate unconfigured = %d, want 400 (body %s)", resp.StatusCode, body)
	}
}
