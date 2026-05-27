package auth

// Roles, lowest to highest privilege (Feature 30). The hierarchy is:
//
//	viewer    — read-only access to inventory, files, logs, metrics.
//	operator  — viewer + non-destructive container lifecycle (start/stop/restart).
//	admin      — full access, including destructive ops, secrets, and user mgmt.
//
// The Auth middleware reloads the user from the DB on every request, so a role
// change takes effect on the user's next call — no token revocation is needed
// to enforce a downgrade.
const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
)

var roleRank = map[string]int{
	RoleViewer:   1,
	RoleOperator: 2,
	RoleAdmin:    3,
}

// RoleRank returns the privilege rank of a role, or 0 for an unknown role.
func RoleRank(role string) int { return roleRank[role] }

// RoleValid reports whether role is one of the known roles.
func RoleValid(role string) bool {
	_, ok := roleRank[role]
	return ok
}

// AtLeast reports whether role meets or exceeds min. An unknown role (rank 0)
// never qualifies, so a malformed/empty role is fail-closed.
func AtLeast(role, min string) bool {
	r, m := roleRank[role], roleRank[min]
	return r != 0 && m != 0 && r >= m
}
