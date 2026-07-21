package templates

import (
	"strings"
	"testing"

	"github.com/KAE-Labs/stratum/backend/db"
)

func TestRenderSubstitutesAndDefaults(t *testing.T) {
	compose := "image: nginx:{{TAG}}\nports:\n  - {{ PORT }}:80\nname: {{NAME}}"
	vars := []db.TemplateVar{
		{Name: "TAG", Default: "latest"},
		{Name: "PORT", Default: "8080"},
		{Name: "NAME", Default: ""}, // no default -> must be supplied
	}
	out, unresolved := Render(compose, vars, map[string]string{"PORT": "9000", "NAME": "web"})
	if strings.Contains(out, "{{") {
		t.Fatalf("unresolved token left: %q", out)
	}
	if !strings.Contains(out, "nginx:latest") { // default used
		t.Error("TAG default not applied")
	}
	if !strings.Contains(out, "9000:80") { // value overrides default
		t.Error("PORT value not applied")
	}
	if !strings.Contains(out, "name: web") {
		t.Error("NAME value not applied")
	}
	if len(unresolved) != 0 {
		t.Errorf("expected none unresolved, got %v", unresolved)
	}
}

func TestRenderReportsUnresolved(t *testing.T) {
	compose := "a: {{FOO}}\nb: {{BAR}}\nc: {{FOO}}"
	out, unresolved := Render(compose, nil, nil)
	// Unresolved tokens are de-duped, sorted, and left in place.
	if len(unresolved) != 2 || unresolved[0] != "BAR" || unresolved[1] != "FOO" {
		t.Fatalf("unresolved = %v, want [BAR FOO]", unresolved)
	}
	if !strings.Contains(out, "{{FOO}}") {
		t.Error("unresolved token should remain in output")
	}
}
