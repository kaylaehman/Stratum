package auth

import "testing"

func TestAtLeast(t *testing.T) {
	cases := []struct {
		role, min string
		want      bool
	}{
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleOperator, true},
		{RoleAdmin, RoleViewer, true},
		{RoleOperator, RoleAdmin, false},
		{RoleOperator, RoleOperator, true},
		{RoleOperator, RoleViewer, true},
		{RoleViewer, RoleOperator, false},
		{RoleViewer, RoleViewer, true},
		// Unknown/empty roles are fail-closed.
		{"", RoleViewer, false},
		{"superuser", RoleViewer, false},
		{RoleViewer, "", false},
	}
	for _, c := range cases {
		if got := AtLeast(c.role, c.min); got != c.want {
			t.Errorf("AtLeast(%q, %q) = %v, want %v", c.role, c.min, got, c.want)
		}
	}
}

func TestRoleValid(t *testing.T) {
	for _, r := range []string{RoleViewer, RoleOperator, RoleAdmin} {
		if !RoleValid(r) {
			t.Errorf("RoleValid(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"", "root", "Admin", "VIEWER"} {
		if RoleValid(r) {
			t.Errorf("RoleValid(%q) = true, want false", r)
		}
	}
}
