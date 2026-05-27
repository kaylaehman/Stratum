package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

// ---------------------------------------------------------------------------
// mapSummary tests
// ---------------------------------------------------------------------------

func TestMapSummary_StripLeadingSlash(t *testing.T) {
	s := container.Summary{
		ID:    "abc123",
		Names: []string{"/mycontainer"},
		Image: "nginx:latest",
		State: "running",
	}
	info := mapSummary(s)
	if info.Name != "mycontainer" {
		t.Errorf("expected Name %q, got %q", "mycontainer", info.Name)
	}
}

func TestMapSummary_NoLeadingSlash(t *testing.T) {
	s := container.Summary{
		Names: []string{"noprefix"},
	}
	info := mapSummary(s)
	if info.Name != "noprefix" {
		t.Errorf("expected Name %q, got %q", "noprefix", info.Name)
	}
}

func TestMapSummary_EmptyNames(t *testing.T) {
	s := container.Summary{
		ID:    "deadbeef",
		Names: []string{},
	}
	info := mapSummary(s)
	if info.Name != "" {
		t.Errorf("expected empty Name for empty Names slice, got %q", info.Name)
	}
	if info.ID != "deadbeef" {
		t.Errorf("expected ID %q, got %q", "deadbeef", info.ID)
	}
}

func TestMapSummary_ComposeProjectLabel(t *testing.T) {
	s := container.Summary{
		Names:  []string{"/web"},
		Labels: map[string]string{"com.docker.compose.project": "mystack"},
	}
	info := mapSummary(s)
	if info.ComposeProject != "mystack" {
		t.Errorf("expected ComposeProject %q, got %q", "mystack", info.ComposeProject)
	}
}

func TestMapSummary_NoComposeLabel(t *testing.T) {
	s := container.Summary{
		Names:  []string{"/standalone"},
		Labels: map[string]string{"other": "value"},
	}
	info := mapSummary(s)
	if info.ComposeProject != "" {
		t.Errorf("expected empty ComposeProject, got %q", info.ComposeProject)
	}
}

func TestMapSummary_NilLabels(t *testing.T) {
	s := container.Summary{
		Names:  []string{"/niltest"},
		Labels: nil,
	}
	info := mapSummary(s)
	if info.ComposeProject != "" {
		t.Errorf("expected empty ComposeProject for nil Labels, got %q", info.ComposeProject)
	}
}

func TestMapSummary_ImageIDPassthrough(t *testing.T) {
	const digest = "sha256:abcdef1234567890"
	s := container.Summary{
		Names:   []string{"/img"},
		ImageID: digest,
	}
	info := mapSummary(s)
	if info.ImageID != digest {
		t.Errorf("expected ImageID %q, got %q", digest, info.ImageID)
	}
}

func TestMapSummary_StatePassthrough(t *testing.T) {
	for _, state := range []string{"running", "exited", "paused", "restarting", "dead", "created"} {
		s := container.Summary{Names: []string{"/c"}, State: state}
		info := mapSummary(s)
		if info.State != state {
			t.Errorf("expected State %q, got %q", state, info.State)
		}
	}
}

// ---------------------------------------------------------------------------
// mapEventAction tests
// ---------------------------------------------------------------------------

func TestMapEventAction_NormalAction(t *testing.T) {
	for _, action := range []string{"start", "die", "create", "destroy", "stop"} {
		got := mapEventAction(action)
		if got != action {
			t.Errorf("mapEventAction(%q) = %q, want %q", action, got, action)
		}
	}
}

func TestMapEventAction_HealthStatusWithSuffix(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"health_status: healthy", "health_status"},
		{"health_status: unhealthy", "health_status"},
		{"health_status:healthy", "health_status"},
	}
	for _, tc := range cases {
		got := mapEventAction(tc.raw)
		if got != tc.want {
			t.Errorf("mapEventAction(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestMapEventAction_EmptyString(t *testing.T) {
	got := mapEventAction("")
	if got != "" {
		t.Errorf("mapEventAction(\"\") = %q, want %q", got, "")
	}
}
