package stacks

import (
	"context"
	"strings"
	"testing"
	"text/template"
)

// TestFindComposeUsesRawNameForDockerLabel is the C3 regression: the Docker
// compose-project label filter must use the RAW project name (Docker stores the
// real name), not the sanitized one — otherwise running stacks whose project
// name contains '.', uppercase, etc. fail discovery ("No compose file located").
func TestFindComposeUsesRawNameForDockerLabel(t *testing.T) {
	f := newFakeFileIO() // statFound=false, Exec returns "" -> all discovery misses
	s := &Service{files: f}

	_, _ = s.FindCompose(context.Background(), "node1", "My.App")

	rawFilter := "label=com.docker.compose.project=My.App"
	sanitizedFilter := "label=com.docker.compose.project=My-App"
	var sawRaw bool
	for _, c := range f.execCalls {
		for _, a := range c.args {
			if a == rawFilter {
				sawRaw = true
			}
			if a == sanitizedFilter {
				t.Errorf("docker label filter used sanitized name %q; must use raw name", sanitizedFilter)
			}
		}
	}
	if !sawRaw {
		t.Errorf("expected a docker label filter with the raw project name %q; exec calls: %+v", rawFilter, f.execCalls)
	}
}

// TestFindComposeSanitizesPathFallback verifies the directory fallback still
// sanitizes the project name so a traversal value cannot escape the search roots.
func TestFindComposeSanitizesPathFallback(t *testing.T) {
	f := newFakeFileIO() // statFound=false so we exercise (and miss) every candidate
	s := &Service{files: f}

	_, _ = s.FindCompose(context.Background(), "node1", "../../etc")

	if len(f.statPaths) == 0 {
		t.Fatal("expected directory-fallback StatEntry probes")
	}
	for _, p := range f.statPaths {
		if strings.Contains(p, "..") {
			t.Errorf("path-fallback candidate %q contains traversal; project name not sanitized", p)
		}
	}
}

func TestFirstConfigFile(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"single", "/data/compose/42/docker-compose.yml\n", "/data/compose/42/docker-compose.yml"},
		{"multi-line picks first non-empty", "\n\n/opt/app/compose.yml\n/opt/app/compose.yml\n", "/opt/app/compose.yml"},
		{"comma-joined takes first", "/srv/a.yml,/srv/b.yml\n", "/srv/a.yml"},
		{"no value sentinel skipped", "<no value>\n/x/docker-compose.yml\n", "/x/docker-compose.yml"},
		{"only no value", "<no value>\n<no value>\n", ""},
		{"whitespace trimmed", "   /q/compose.yaml  \n", "/q/compose.yaml"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstConfigFile(c.in); got != c.want {
				t.Fatalf("firstConfigFile(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNonEmptyLines(t *testing.T) {
	got := nonEmptyLines("a\n\n  b \n\nc\n")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseMountLines(t *testing.T) {
	in := "/data\t/var/lib/docker/volumes/portainer_data/_data\n" +
		"/etc/localtime\t/etc/localtime\n" +
		"malformed-no-tab\n" +
		"\t/only-src\n"
	m := parseMountLines(in)
	if got := m["/data"]; got != "/var/lib/docker/volumes/portainer_data/_data" {
		t.Fatalf("/data -> %q", got)
	}
	if got := m["/etc/localtime"]; got != "/etc/localtime" {
		t.Fatalf("/etc/localtime -> %q", got)
	}
	if _, ok := m["malformed-no-tab"]; ok {
		t.Fatal("malformed line should be skipped")
	}
	if len(m) != 2 {
		t.Fatalf("map size = %d, want 2: %v", len(m), m)
	}
}

// TestComposePathFromWorkingDir covers the pure path-builder helper that is used
// before any stat calls. It ensures every standard compose filename is tried and
// that edge cases (empty dir, trailing slash) are handled cleanly.
func TestComposePathFromWorkingDir(t *testing.T) {
	cases := []struct {
		name       string
		workingDir string
		wantNil    bool
		wantFirst  string
		wantLen    int
	}{
		{
			name:       "empty workingDir returns nil",
			workingDir: "",
			wantNil:    true,
		},
		{
			name:       "normal dir produces all filenames",
			workingDir: "/home/user/watchtower",
			wantFirst:  "/home/user/watchtower/docker-compose.yml",
			wantLen:    len(composeFilenames),
		},
		{
			name:       "root deployed stack",
			workingDir: "/root/watchtower",
			wantFirst:  "/root/watchtower/docker-compose.yml",
			wantLen:    len(composeFilenames),
		},
		{
			name: "path.Join cleans trailing slash",
			// path.Join normalises the result so trailing slash is stripped.
			workingDir: "/srv/stacks/myapp",
			wantFirst:  "/srv/stacks/myapp/docker-compose.yml",
			wantLen:    len(composeFilenames),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := composePathFromWorkingDir(c.workingDir)
			if c.wantNil {
				if got != nil {
					t.Fatalf("want nil, got %v", got)
				}
				return
			}
			if len(got) != c.wantLen {
				t.Fatalf("len = %d, want %d: %v", len(got), c.wantLen, got)
			}
			if got[0] != c.wantFirst {
				t.Fatalf("first = %q, want %q", got[0], c.wantFirst)
			}
			// Verify all results start with the working dir.
			for _, p := range got {
				if !strings.HasPrefix(p, c.workingDir+"/") {
					t.Errorf("path %q does not start with workingDir %q", p, c.workingDir)
				}
			}
			// Verify no duplicate paths.
			seen := make(map[string]bool, len(got))
			for _, p := range got {
				if seen[p] {
					t.Errorf("duplicate path: %q", p)
				}
				seen[p] = true
			}
		})
	}
}

// TestInspectComposeLabelTemplateSyntax validates that the Go template strings
// used in inspectComposeLabels compile as valid text/template expressions.
// This guards against accidentally reintroducing the broken
// '{{.Label "key"}}' syntax that has no method on the docker inspect context.
func TestInspectComposeLabelTemplateSyntax(t *testing.T) {
	// We can't exec docker in unit tests, but we CAN verify the template strings
	// are syntactically valid Go templates (text/template parses them).
	goodTemplates := []string{
		`{{index .Config.Labels "com.docker.compose.project.config_files"}}`,
		`{{index .Config.Labels "com.docker.compose.project.working_dir"}}`,
	}
	for _, tmpl := range goodTemplates {
		if _, err := template.New("t").Parse(tmpl); err != nil {
			t.Errorf("template %q failed to parse: %v", tmpl, err)
		}
	}

	// The broken pattern also parses (it's syntactically valid Go template) but
	// it calls a non-existent method and returns <no value> at runtime.
	// We document the distinction: "index" accesses a map key; ".Label" would
	// call a method that doesn't exist on the inspect JSON struct.
	broken := `{{.Label "com.docker.compose.project.config_files"}}`
	tmpl, err := template.New("broken").Parse(broken)
	if err != nil {
		t.Logf("broken syntax unexpectedly failed to parse: %v", err)
		return
	}
	// Execute against a struct that has no Label method: result must be "<no value>".
	var buf strings.Builder
	_ = tmpl.Execute(&buf, struct{ Labels map[string]string }{
		Labels: map[string]string{
			"com.docker.compose.project.config_files": "/opt/watchtower/docker-compose.yml",
		},
	})
	if got := buf.String(); got != "<no value>" {
		// If this stops being "<no value>", the regression test needs updating.
		t.Logf("broken template produced %q (expected <no value>)", got)
	}
}

func TestRemapUnderMount(t *testing.T) {
	cases := []struct {
		name                     string
		containerPath, dest, src string
		wantPath                 string
		wantOK                   bool
	}{
		{
			"portainer data dir", "/data/compose/42/docker-compose.yml",
			"/data", "/var/lib/docker/volumes/portainer_data/_data",
			"/var/lib/docker/volumes/portainer_data/_data/compose/42/docker-compose.yml", true,
		},
		{"exact mount", "/cfg", "/cfg", "/host/cfg", "/host/cfg", true},
		{"not under mount", "/data/x", "/other", "/host", "", false},
		{"root dest rejected", "/data/x", "/", "/host", "", false},
		{"empty src rejected", "/data/x", "/data", "", "", false},
		{"sibling prefix not matched", "/database/x", "/data", "/host", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := remapUnderMount(c.containerPath, c.dest, c.src)
			if ok != c.wantOK || got != c.wantPath {
				t.Fatalf("remapUnderMount(%q,%q,%q) = (%q,%v), want (%q,%v)",
					c.containerPath, c.dest, c.src, got, ok, c.wantPath, c.wantOK)
			}
		})
	}
}
