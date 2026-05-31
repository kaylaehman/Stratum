// Package stacks implements live Compose stack management: read the running
// stack's compose YAML from disk, merge runtime env vars (including secrets
// injected at deploy time), and redeploy via `docker compose up -d` over SSH.
//
// This is NOT the Template library (Feature 14) — Templates are reusable
// blueprints. Stacks are the LIVE running Compose projects on a node.
package stacks

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/fs"
)

// composeFilenames is the ordered list of filenames the service tries when
// locating the compose file for a project under a given directory.
var composeFilenames = []string{
	"docker-compose.yml",
	"docker-compose.yaml",
	"compose.yml",
	"compose.yaml",
}

// EnvVar is one key/value pair that forms the runtime environment for a stack.
// SecretID, when non-empty, means the value is stored encrypted in the secrets
// vault; the plaintext is never carried inside this struct at rest.
type EnvVar struct {
	Key      string `json:"key"`
	Value    string `json:"value,omitempty"`     // plaintext — only present after Reveal
	SecretID string `json:"secret_id,omitempty"` // set when backed by the secrets vault
	Masked   bool   `json:"masked"`              // true when backed by a secret
}

// StackEnvRow is one persisted env-var record for a (node, project) pair.
// ValueEncrypted stores a plaintext value sealed by AES-256-GCM; it is nil
// when the var is backed by a secret group entry (SecretID is set instead).
type StackEnvRow struct {
	ID             string
	NodeID         string
	ProjectName    string
	Key            string
	ValueEncrypted []byte // nil when SecretID != ""
	SecretID       string // non-empty when backed by secrets vault
}

// Service reads and redeploys live Compose stacks.
type Service struct {
	store  db.Store
	files  *fs.Service
	cipher *crypto.Cipher
}

// New wires the service dependencies.
func New(store db.Store, files *fs.Service, cipher *crypto.Cipher) *Service {
	return &Service{store: store, files: files, cipher: cipher}
}

// FindCompose discovers the compose file path for a project on a node. It
// searches composeDirs under /opt, /srv, /home, /var, /mnt for a directory
// whose last path segment matches the project name, then probes the standard
// filenames. Returns "" when nothing is found (degraded gracefully).
func (s *Service) FindCompose(ctx context.Context, nodeID, projectName string) (string, error) {
	// SECURITY: the project name comes from a URL param. Sanitize before it is
	// ever joined into a filesystem path so a value like "../../etc" cannot
	// escape the search roots (path.Join would otherwise resolve the traversal).
	// Legitimate compose project names are already restricted to [a-z0-9_-].
	projectName = sanitizeProject(projectName)
	searchRoots := []string{"/opt", "/srv", "/home", "/var/lib", "/mnt"}
	for _, root := range searchRoots {
		dir := path.Join(root, projectName)
		for _, fname := range composeFilenames {
			candidate := path.Join(dir, fname)
			if _, err := s.files.StatEntry(ctx, nodeID, candidate); err == nil {
				return candidate, nil
			}
		}
	}
	// Also try /opt/stratum-stacks/<project> (the template deploy default).
	for _, fname := range composeFilenames {
		candidate := path.Join("/opt/stratum-stacks", projectName, fname)
		if _, err := s.files.StatEntry(ctx, nodeID, candidate); err == nil {
			return candidate, nil
		}
	}
	return "", nil
}

// ReadCompose returns the raw YAML of the compose file at composePath.
func (s *Service) ReadCompose(ctx context.Context, nodeID, composePath string) ([]byte, error) {
	content, tooLarge, _, err := s.files.Preview(ctx, nodeID, composePath)
	if err != nil {
		return nil, fmt.Errorf("read compose: %w", err)
	}
	if tooLarge {
		return nil, fmt.Errorf("compose file exceeds preview limit")
	}
	return content, nil
}

// Deploy writes the updated compose YAML and runs `docker compose up -d`.
// envVars is the merged runtime environment: each key may be a raw value or a
// secret reference. Secrets are decrypted here and passed as --env KEY=VALUE
// arguments — they are never written to disk. Returns the command output.
//
// Snapshot responsibility: the caller (API handler) must snapshot all
// containers in the project before calling Deploy.
func (s *Service) Deploy(
	ctx context.Context,
	nodeID, composePath string,
	composeYAML []byte,
	envVars []EnvVar,
) (string, error) {
	// Write the updated compose YAML to the node via SFTP.
	if err := s.files.Write(ctx, nodeID, composePath, composeYAML, nil); err != nil {
		return "", fmt.Errorf("write compose: %w", err)
	}

	// Build inline --env args from resolved env vars. Secrets are decrypted
	// here; the plaintext never leaves this function (not stored, not logged).
	args, err := s.buildEnvArgs(ctx, envVars)
	if err != nil {
		return "", fmt.Errorf("resolve env: %w", err)
	}

	// Run: docker compose -f <path> [--env KEY=VALUE ...] up -d
	cmdArgs := make([]string, 0, 4+len(args))
	cmdArgs = append(cmdArgs, "compose", "-f", composePath)
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = append(cmdArgs, "up", "-d")

	out, err := s.files.Exec(ctx, nodeID, "docker", cmdArgs...)
	if err != nil {
		return out, fmt.Errorf("compose up: %w", err)
	}
	return out, nil
}

// ErrInvalidAction is returned by Lifecycle for an action outside the allowed set.
var ErrInvalidAction = fmt.Errorf("stacks: invalid lifecycle action")

// lifecycleActions is the allowed set for Lifecycle; anything else is rejected.
var lifecycleActions = map[string]bool{"stop": true, "start": true, "restart": true}

// Lifecycle runs a whole-project compose lifecycle action (stop/start/restart)
// against the named project on a node. The project name is sanitized before use.
// It prefers `docker compose -p <project> <action>`, adding `-f <path>` when the
// compose file can be located on disk (so the action targets the on-disk project
// definition). Returns the combined command output.
//
// TODO: when no compose file is found and `docker compose -p` cannot resolve the
// project (e.g. it was started without compose), fall back to acting on the set
// of containers carrying the com.docker.compose.project label. Not implemented
// here because the common case (stacks deployed via this platform) always has a
// compose file or a labelled compose project the daemon can resolve by name.
func (s *Service) Lifecycle(ctx context.Context, nodeID, project, action string) (string, error) {
	if !lifecycleActions[action] {
		return "", ErrInvalidAction
	}
	project = sanitizeProject(project)

	cmdArgs := []string{"compose", "-p", project}
	// Best-effort: if we can find the compose file, target it explicitly so the
	// action operates on the on-disk project definition regardless of cwd.
	if composePath, err := s.FindCompose(ctx, nodeID, project); err == nil && composePath != "" {
		cmdArgs = append(cmdArgs, "-f", composePath)
	}
	cmdArgs = append(cmdArgs, action)

	out, err := s.files.Exec(ctx, nodeID, "docker", cmdArgs...)
	if err != nil {
		return out, fmt.Errorf("compose %s: %w", action, err)
	}
	return out, nil
}

// buildEnvArgs resolves each EnvVar to a `--env KEY=VALUE` pair.
// Secret-backed vars are decrypted inline; plaintext vars are used directly.
// The resulting slice is: ["--env","K1=V1","--env","K2=V2", ...].
func (s *Service) buildEnvArgs(ctx context.Context, vars []EnvVar) ([]string, error) {
	if len(vars) == 0 {
		return nil, nil
	}
	// Sort for deterministic command lines (aids debugging without leaking order).
	sorted := make([]EnvVar, len(vars))
	copy(sorted, vars)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })

	args := make([]string, 0, len(sorted)*2)
	for _, v := range sorted {
		val, err := s.resolveValue(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("resolve %q: %w", v.Key, err)
		}
		args = append(args, "--env", v.Key+"="+val)
	}
	return args, nil
}

// resolveValue returns the plaintext value for an EnvVar. Secret-backed vars
// fetch and decrypt from the vault; plain vars return their Value field.
func (s *Service) resolveValue(ctx context.Context, v EnvVar) (string, error) {
	if v.SecretID != "" {
		row, err := s.store.GetSecret(ctx, v.SecretID)
		if err != nil {
			return "", err
		}
		pt, err := s.cipher.Open(row.ValueEncrypted)
		if err != nil {
			return "", err
		}
		return string(pt), nil
	}
	return v.Value, nil
}

// SetEnvVar stores (or removes) one env var for a (node, project) pair.
// If secretID is non-empty, the var is backed by the secrets vault and value
// is ignored. If value is empty and secretID is empty, the var is removed.
func (s *Service) SetEnvVar(ctx context.Context, nodeID, projectName, key, value, secretID string) error {
	return s.store.UpsertStackEnvVar(ctx, db.StackEnvRow{
		NodeID:      nodeID,
		ProjectName: projectName,
		Key:         key,
		Value:       value,
		SecretID:    secretID,
	})
}

// DeleteEnvVar removes one env var for a (node, project) pair.
func (s *Service) DeleteEnvVar(ctx context.Context, nodeID, projectName, key string) error {
	return s.store.DeleteStackEnvVar(ctx, nodeID, projectName, key)
}

// ListEnvVars returns the env vars for a (node, project) pair. Values are
// never returned in plaintext here; callers get Masked=true for secret-backed
// vars. A dedicated reveal path (with audit) exists for plaintext access.
func (s *Service) ListEnvVars(ctx context.Context, nodeID, projectName string) ([]EnvVar, error) {
	rows, err := s.store.ListStackEnvVars(ctx, nodeID, projectName)
	if err != nil {
		return nil, err
	}
	out := make([]EnvVar, 0, len(rows))
	for _, r := range rows {
		ev := EnvVar{Key: r.Key, SecretID: r.SecretID, Masked: r.SecretID != "" || len(r.Value) > 0}
		out = append(out, ev)
	}
	return out, nil
}

// RevealEnvVar decrypts and returns the plaintext value for one env var.
// Secret-backed vars decrypt from the vault; plain vars decrypt from the row.
// This must only be called after admin + audit checks.
func (s *Service) RevealEnvVar(ctx context.Context, nodeID, projectName, key string) (string, error) {
	rows, err := s.store.ListStackEnvVars(ctx, nodeID, projectName)
	if err != nil {
		return "", err
	}
	for _, r := range rows {
		if r.Key != key {
			continue
		}
		if r.SecretID != "" {
			row, err := s.store.GetSecret(ctx, r.SecretID)
			if err != nil {
				return "", err
			}
			pt, err := s.cipher.Open(row.ValueEncrypted)
			if err != nil {
				return "", err
			}
			return string(pt), nil
		}
		if len(r.Value) > 0 {
			return r.Value, nil
		}
		return "", nil
	}
	return "", db.ErrNotFound
}

// sanitizeProject removes characters unsafe in a file path segment. A dot is
// treated as unsafe to prevent sequences like `..` that could escape a root.
func sanitizeProject(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, name)
}

// SanitizeProject exposes the sanitizer for use in the API layer.
func SanitizeProject(name string) string { return sanitizeProject(name) }
