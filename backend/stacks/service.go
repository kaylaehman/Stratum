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
	"time"

	"github.com/KAE-Labs/stratum/backend/crypto"
	"github.com/KAE-Labs/stratum/backend/db"
	"github.com/KAE-Labs/stratum/backend/fs"
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

// fileIO is the narrow interface over fs.Service that Service requires.
// *fs.Service satisfies this interface. The interface is unexported so it
// stays an implementation detail; callers use New (production) or
// newTestService (tests, via the test helper in service_test.go).
type fileIO interface {
	Exec(ctx context.Context, nodeID, cmd string, args ...string) (string, error)
	Write(ctx context.Context, nodeID, p string, content []byte, ifUnmodifiedSince *time.Time) error
	StatEntry(ctx context.Context, nodeID, p string) (fs.Entry, error)
	Preview(ctx context.Context, nodeID, p string) (content []byte, tooLarge bool, modTime time.Time, err error)
}

// Service reads and redeploys live Compose stacks.
type Service struct {
	store  db.Store
	files  fileIO
	cipher *crypto.Cipher
}

// New wires the service dependencies with the production fs.Service.
func New(store db.Store, files *fs.Service, cipher *crypto.Cipher) *Service {
	return &Service{store: store, files: files, cipher: cipher}
}

// FindCompose discovers the compose file path for a project on a node.
//
// It first asks Docker where the project's compose file actually is — every
// Compose v2 container is stamped with a `com.docker.compose.project.config_files`
// label holding the absolute path Compose was invoked with. This is the only
// reliable source for stacks deployed to non-standard locations (Portainer, git
// stacks, custom data dirs), which the old directory-guessing missed. When the
// stack was deployed from INSIDE a container (Portainer runs compose from its own
// container, so the label reads e.g. `/data/compose/<id>/docker-compose.yml`),
// the label path won't exist on the host — we then translate it through the
// deploying container's bind/volume mounts back to a host path.
//
// If the label lookup yields nothing reachable, it falls back to probing the
// standard directories (/opt, /srv, /home, /var/lib, /mnt, /opt/stratum-stacks)
// for a dir matching the project name. Returns "" when nothing is found
// (degraded gracefully).
func (s *Service) FindCompose(ctx context.Context, nodeID, projectName string) (string, error) {
	// Use the RAW project name for Docker label matching: Docker stores the real,
	// unsanitized compose project name, so sanitizing here would miss any project
	// whose name contains '.', uppercase, or other characters (common for imported
	// or manually-deployed stacks) — the cause of "No compose file could be
	// located" for running stacks. SSH args are shell-quoted (injection-safe), so
	// the raw value is safe to pass to docker, and findComposeByLabel never joins
	// it into a filesystem path.
	rawName := projectName
	// SECURITY: sanitize before joining into a filesystem path so a value like
	// "../../etc" cannot escape the search roots (path.Join would otherwise resolve
	// the traversal). Only the directory fallback below joins paths.
	safeName := sanitizeProject(projectName)

	// 1. Ask Docker for the project's real compose path via the compose label.
	if p, err := s.findComposeByLabel(ctx, nodeID, rawName); err == nil && p != "" {
		return p, nil
	}

	// 2. Fall back to guessing standard directories.
	// /root is included for stacks deployed as root (common on single-user homelabs).
	// /opt/stratum-stacks is the template-deploy default.
	searchRoots := []string{"/opt", "/srv", "/home", "/root", "/var/lib", "/mnt", "/opt/stratum-stacks"}
	for _, root := range searchRoots {
		dir := path.Join(root, safeName)
		for _, fname := range composeFilenames {
			candidate := path.Join(dir, fname)
			if _, err := s.files.StatEntry(ctx, nodeID, candidate); err == nil {
				return candidate, nil
			}
		}
	}
	return "", nil
}

// findComposeByLabel resolves the compose file from the project's running (or
// stopped) containers via the `com.docker.compose.project.config_files` and
// `com.docker.compose.project.working_dir` labels.
//
// BUG FIXED: the previous implementation used
//
//	docker ps --format '{{.Label "com.docker.compose.project.config_files"}}'
//
// Docker's ps --format Go-template context does NOT expose a .Label function
// that accepts a string argument; it only exposes .Labels (a map) for use with
// the index template function. The old template silently produced "<no value>"
// for every container, so the entire label-based discovery path was a no-op.
//
// The fix uses docker inspect with {{index .Labels "key"}} which is the correct
// Go-template accessor for a named label value, and it works on both old and new
// Docker versions.
func (s *Service) findComposeByLabel(ctx context.Context, nodeID, projectName string) (string, error) {
	// Step 1: get the names of containers in this project so we can inspect them.
	names := s.projectContainerNames(ctx, nodeID, projectName)
	if len(names) == 0 {
		return "", nil
	}

	// Step 2: inspect the first container for both compose labels.
	// docker inspect --format '{{index .Labels "key"}}' <name>
	// Using index .Labels avoids the non-existent .Label function bug.
	configFiles, workingDir := s.inspectComposeLabels(ctx, nodeID, names[0])

	// Step 3: try config_files label path directly on host.
	if labelPath := firstConfigFile(configFiles); labelPath != "" {
		if _, err := s.files.StatEntry(ctx, nodeID, labelPath); err == nil {
			return labelPath, nil
		}
		// Portainer-deployed: config_files path is inside the deployer container —
		// translate through its mounts back to a host path.
		if hostPath, ok := s.translateContainerComposePath(ctx, nodeID, projectName, labelPath); ok {
			return hostPath, nil
		}
	}

	// Step 4: try working_dir + standard compose filenames.
	if p := s.findComposeInWorkingDir(ctx, nodeID, workingDir); p != "" {
		return p, nil
	}

	return "", nil
}

// projectContainerNames returns the names of containers belonging to the
// compose project (running or stopped). It does NOT include Portainer — that
// is only consulted later if a container-path translation is needed.
func (s *Service) projectContainerNames(ctx context.Context, nodeID, projectName string) []string {
	out, err := s.files.Exec(ctx, nodeID, "docker", "ps", "-a",
		"--filter", "label=com.docker.compose.project="+projectName,
		"--format", "{{.Names}}")
	if err != nil {
		return nil
	}
	return nonEmptyLines(out)
}

// inspectComposeLabels returns the raw string values of
// com.docker.compose.project.config_files and
// com.docker.compose.project.working_dir for the named container.
// It uses "docker inspect --format '{{index .Labels "key"}}'" which is the
// correct Go template accessor for a map entry — unlike the broken
// "{{.Label "key"}}" form that does not exist on the ps formatter context.
func (s *Service) inspectComposeLabels(ctx context.Context, nodeID, containerName string) (configFiles, workingDir string) {
	// We run two separate inspect calls with simple {{index .Labels "..."}}
	// templates rather than one complex template so each output is unambiguous.
	if out, err := s.files.Exec(ctx, nodeID, "docker", "inspect",
		"--format", `{{index .Config.Labels "com.docker.compose.project.config_files"}}`,
		"--", containerName); err == nil {
		configFiles = strings.TrimSpace(out)
		if configFiles == "<no value>" {
			configFiles = ""
		}
	}
	if out, err := s.files.Exec(ctx, nodeID, "docker", "inspect",
		"--format", `{{index .Config.Labels "com.docker.compose.project.working_dir"}}`,
		"--", containerName); err == nil {
		workingDir = strings.TrimSpace(out)
		if workingDir == "<no value>" {
			workingDir = ""
		}
	}
	return configFiles, workingDir
}

// findComposeInWorkingDir probes for a compose file under workingDir using the
// standard filename list. Returns "" if workingDir is empty or nothing exists.
func (s *Service) findComposeInWorkingDir(ctx context.Context, nodeID, workingDir string) string {
	if workingDir == "" {
		return ""
	}
	for _, fname := range composeFilenames {
		candidate := path.Join(workingDir, fname)
		if _, err := s.files.StatEntry(ctx, nodeID, candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// composePathFromWorkingDir is a pure helper (no I/O) that constructs candidate
// compose file paths for a given workingDir. Used in tests and as the
// enumeration step before StatEntry calls.
func composePathFromWorkingDir(workingDir string) []string {
	if workingDir == "" {
		return nil
	}
	out := make([]string, len(composeFilenames))
	for i, fname := range composeFilenames {
		out[i] = path.Join(workingDir, fname)
	}
	return out
}

// translateContainerComposePath maps an in-container compose path (e.g.
// /data/compose/42/docker-compose.yml reported by a Portainer-deployed stack)
// to a host path by inspecting the bind/volume mounts of the containers in the
// project and of any Portainer container, then verifying the candidate exists.
func (s *Service) translateContainerComposePath(ctx context.Context, nodeID, projectName, containerPath string) (string, bool) {
	// Gather mount (destination -> host source) pairs from candidate containers:
	// the project's own containers plus any container named/imaged "portainer".
	// `docker ps -a --format` with .Mounts only gives sources, so use inspect.
	names := s.composeRelatedContainerNames(ctx, nodeID, projectName)
	for _, name := range names {
		// Format: one "<destination>\t<source>" line per mount.
		out, err := s.files.Exec(ctx, nodeID, "docker", "inspect",
			"--format", `{{range .Mounts}}{{.Destination}}{{"\t"}}{{.Source}}{{"\n"}}{{end}}`,
			name)
		if err != nil {
			continue
		}
		for dest, src := range parseMountLines(out) {
			if hostPath, ok := remapUnderMount(containerPath, dest, src); ok {
				if _, err := s.files.StatEntry(ctx, nodeID, hostPath); err == nil {
					return hostPath, true
				}
			}
		}
	}
	return "", false
}

// composeRelatedContainerNames returns container names worth inspecting for a
// mount that exposes the compose file: the project's containers first, then any
// container that looks like Portainer (which is what deployed it).
// Used only by translateContainerComposePath.
func (s *Service) composeRelatedContainerNames(ctx context.Context, nodeID, projectName string) []string {
	// Re-use projectContainerNames for the project slice; append Portainer separately.
	names := s.projectContainerNames(ctx, nodeID, projectName)
	if out, err := s.files.Exec(ctx, nodeID, "docker", "ps", "-a",
		"--filter", "name=portainer",
		"--format", "{{.Names}}"); err == nil {
		names = append(names, nonEmptyLines(out)...)
	}
	return names
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

// firstConfigFile picks the first usable compose path from `docker ps` output
// of the config_files label (one line per container; compose joins multiple
// -f files with commas — we take the first).
func firstConfigFile(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "<no value>" {
			continue
		}
		if i := strings.IndexByte(line, ','); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line != "" {
			return line
		}
	}
	return ""
}

// nonEmptyLines splits command output into trimmed, non-empty lines.
func nonEmptyLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// parseMountLines parses "<destination>\t<source>" lines (from docker inspect)
// into a destination→hostSource map.
func parseMountLines(out string) map[string]string {
	m := make(map[string]string)
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l == "" {
			continue
		}
		parts := strings.SplitN(l, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		dest := strings.TrimSpace(parts[0])
		src := strings.TrimSpace(parts[1])
		if dest != "" && src != "" {
			m[dest] = src
		}
	}
	return m
}

// remapUnderMount translates an in-container path to a host path when it falls
// under a mount whose container destination is `dest` and host source is `src`.
// Returns ("", false) when containerPath is not under dest.
func remapUnderMount(containerPath, dest, src string) (string, bool) {
	containerPath = path.Clean(containerPath)
	dest = path.Clean(dest)
	src = path.Clean(src)
	if dest == "/" || dest == "." || src == "" || src == "." {
		return "", false
	}
	if containerPath == dest {
		return src, true
	}
	prefix := dest + "/"
	if strings.HasPrefix(containerPath, prefix) {
		return path.Join(src, containerPath[len(prefix):]), true
	}
	return "", false
}

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

// ErrProjectExists is returned when a compose file already exists at the target
// directory. CreateStack never silently overwrites an existing stack.
var ErrProjectExists = fmt.Errorf("stacks: a compose file already exists in that directory")

// ErrInvalidDirectory is returned when the supplied directory path fails
// validation (not absolute, contains "..", or is empty).
var ErrInvalidDirectory = fmt.Errorf("stacks: directory must be an absolute path with no '..' segments")

// CreateStack provisions a brand-new compose stack on a node. It:
//  1. Validates project (non-empty, stable under sanitization) and directory
//     (absolute, no "..", defaults to /opt/<project>).
//  2. Creates the directory via `mkdir -p` on the node.
//  3. Rejects the operation if a compose file already exists there.
//  4. Writes docker-compose.yml to the directory.
//  5. Calls Deploy to inject secrets at runtime and bring the stack up.
//
// Returns the full compose file path and the docker-compose output.
// Secrets are never written to disk: they are forwarded to Deploy as envVars.
func (s *Service) CreateStack(
	ctx context.Context,
	nodeID, project, directory string,
	composeYAML []byte,
	envVars []EnvVar,
) (composePath string, output string, err error) {
	// --- validation ---

	if project == "" {
		return "", "", fmt.Errorf("stacks: project name is required")
	}
	if sanitizeProject(project) != project {
		return "", "", fmt.Errorf("stacks: project name %q contains characters not allowed in a stack name (a-z A-Z 0-9 - _)", project)
	}

	directory, err = validateDirectory(project, directory)
	if err != nil {
		return "", "", err
	}

	// --- ensure directory exists ---

	if _, execErr := s.files.Exec(ctx, nodeID, "mkdir", "-p", "--", directory); execErr != nil {
		return "", "", fmt.Errorf("stacks: create directory %q: %w", directory, execErr)
	}

	// --- reject if compose file already exists ---

	composePath = path.Join(directory, "docker-compose.yml")
	if _, statErr := s.files.StatEntry(ctx, nodeID, composePath); statErr == nil {
		return "", "", ErrProjectExists
	}

	// --- write compose YAML ---

	if writeErr := s.files.Write(ctx, nodeID, composePath, composeYAML, nil); writeErr != nil {
		return "", "", fmt.Errorf("stacks: write compose file: %w", writeErr)
	}

	// --- deploy ---

	out, deployErr := s.Deploy(ctx, nodeID, composePath, composeYAML, envVars)
	if deployErr != nil {
		return composePath, out, deployErr
	}
	return composePath, out, nil
}

// validateDirectory checks that dir is an absolute path with no ".." segments.
// If dir is empty it returns the default path /opt/<project>.
// The ".." check is performed on the RAW path before cleaning, because
// path.Clean resolves ".." segments and would silently absorb traversal
// attempts like "/opt/../etc" → "/etc".
func validateDirectory(project, dir string) (string, error) {
	if dir == "" {
		return "/opt/" + project, nil
	}
	// Check raw segments for ".." BEFORE cleaning.
	for _, seg := range strings.Split(dir, "/") {
		if seg == ".." {
			return "", ErrInvalidDirectory
		}
	}
	if !path.IsAbs(dir) {
		return "", ErrInvalidDirectory
	}
	return path.Clean(dir), nil
}
