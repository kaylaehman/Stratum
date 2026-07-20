package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/capabilities"
	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/stacks"
)

// stackDeployTimeout bounds the compose write + up operation.
const stackDeployTimeout = 10 * time.Minute

// stackEnvVarView is the API representation of one env var. Values are always
// masked in list responses; secret-backed vars carry secret_id for the UI to
// show the group/key picker.
type stackEnvVarView struct {
	Key      string `json:"key"`
	SecretID string `json:"secret_id,omitempty"`
	Masked   bool   `json:"masked"`
}

// createStackBody is the request body for POST /api/nodes/:id/stacks.
type createStackBody struct {
	Project      string          `json:"project"`
	Directory    string          `json:"directory"`    // absolute path on node; defaults to /opt/<project>
	ComposeYAML  string          `json:"compose_yaml"` // required; the full docker-compose.yml content
	EnvVars      []stacks.EnvVar `json:"env_vars"`     // runtime env; secrets referenced by id
	SecretGroups []string        `json:"secret_groups"` // group IDs injected wholesale
}

// CreateStack creates a new compose stack on a node by writing docker-compose.yml
// to the specified directory and running docker compose up -d. Admin-gated and
// audited. The directory must not already contain a compose file.
func (h *Handlers) CreateStack(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	nodeID := chi.URLParam(r, "id")

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Docker {
		writeError(w, http.StatusConflict, "docker_not_available")
		return
	}

	var body createStackBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	if body.ComposeYAML == "" {
		writeError(w, http.StatusBadRequest, "compose_yaml_required")
		return
	}

	// Merge secret groups into the env var list (same pattern as RedeployStack).
	mergedEnv, err := h.mergeSecretGroups(r.Context(), body.EnvVars, body.SecretGroups)
	if err != nil {
		writeError(w, http.StatusBadGateway, "secret_resolution_failed")
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), stackDeployTimeout)
	defer cancel()

	composePath, out, createErr := h.Stacks.CreateStack(
		ctx, nodeID, body.Project, body.Directory,
		[]byte(body.ComposeYAML), mergedEnv,
	)

	auditCreateStack(r, nodeID, body.Project, composePath, createErr)

	if errors.Is(createErr, stacks.ErrProjectExists) {
		writeError(w, http.StatusConflict, "project_already_exists")
		return
	}
	if errors.Is(createErr, stacks.ErrInvalidDirectory) {
		writeError(w, http.StatusBadRequest, "invalid_directory")
		return
	}
	if createErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  "create_failed",
			"output": out,
		})
		return
	}

	// Persist the env-var configuration for this (node, project).
	for _, ev := range body.EnvVars {
		if err := h.Stacks.SetEnvVar(r.Context(), nodeID, body.Project, ev.Key, ev.Value, ev.SecretID); err != nil {
			h.Logger.Warn("stack: persist env var failed", "node", nodeID, "project", body.Project, "key", ev.Key, "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"project":      body.Project,
		"compose_path": composePath,
		"output":       out,
	})
}

func auditCreateStack(r *http.Request, nodeID, project, composePath string, createErr error) {
	e := activity.FromContext(r.Context())
	if e == nil {
		return
	}
	e.Action = activity.ActionStackCreate
	e.TargetType = ptr(activity.TargetStack)
	detail := map[string]string{
		"node_id":      nodeID,
		"project":      project,
		"compose_path": composePath,
	}
	if createErr != nil {
		detail["error"] = createErr.Error()
	}
	e.Detail = detail
}

// GetStackCompose returns the current compose YAML for a named stack on a node.
// Requires docker capability; degrades to 409 Conflict if not available.
func (h *Handlers) GetStackCompose(w http.ResponseWriter, r *http.Request) {
	// Admin-gated: raw compose YAML routinely embeds inline secrets (DB URLs,
	// passwords, API keys). Sibling env/config ops are already admin-gated, and
	// ListStackEnvVars masks values — the raw compose text must not bypass that.
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	project := chi.URLParam(r, "project")

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Docker {
		writeError(w, http.StatusConflict, "docker_not_available")
		return
	}

	composePath, err := h.Stacks.FindCompose(r.Context(), nodeID, project)
	if err != nil {
		writeError(w, http.StatusBadGateway, "find_compose_failed")
		return
	}
	if composePath == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"compose_path": "",
			"compose_yaml": "",
			"found":        false,
		})
		return
	}

	yaml, err := h.Stacks.ReadCompose(r.Context(), nodeID, composePath)
	if err != nil {
		writeError(w, http.StatusBadGateway, "read_compose_failed")
		return
	}

	envVars, err := h.Stacks.ListEnvVars(r.Context(), nodeID, project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	views := makeEnvVarViews(envVars)

	writeJSON(w, http.StatusOK, map[string]any{
		"compose_path": composePath,
		"compose_yaml": string(yaml),
		"env_vars":     views,
		"found":        true,
	})
}

// redeployBody is the request body for POST /api/nodes/:id/stacks/:project/deploy.
type redeployBody struct {
	ComposePath  string          `json:"compose_path"`  // must match found path or be allowlisted
	ComposeYAML  string          `json:"compose_yaml"`  // updated YAML; if empty, existing file is used
	EnvVars      []stacks.EnvVar `json:"env_vars"`      // merged env; secrets referenced by id
	SecretGroups []string        `json:"secret_groups"` // group IDs whose keys are injected wholesale
}

// RedeployStack writes the updated compose YAML and runs `docker compose up -d`.
// Pre-redeploy: snapshots all containers in the project. Admin-gated + audited.
// Secrets are injected at runtime via --env; never written to disk.
func (h *Handlers) RedeployStack(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	nodeID := chi.URLParam(r, "id")
	project := chi.URLParam(r, "project")

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Docker {
		writeError(w, http.StatusConflict, "docker_not_available")
		return
	}

	var body redeployBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	// SECURITY: never trust the client-supplied compose_path. Re-resolve the
	// canonical path server-side from the (sanitized) project name and use that
	// for all reads/writes/exec. A client path, if sent, must match exactly.
	composePath, err := h.Stacks.FindCompose(r.Context(), nodeID, project)
	if err != nil {
		writeError(w, http.StatusBadGateway, "find_compose_failed")
		return
	}
	if composePath == "" {
		writeError(w, http.StatusBadRequest, "compose_not_found")
		return
	}
	if body.ComposePath != "" && body.ComposePath != composePath {
		writeError(w, http.StatusBadRequest, "compose_path_not_allowed")
		return
	}

	// Merge any whole-group secret injections into the env vars list.
	mergedEnv, err := h.mergeSecretGroups(r.Context(), body.EnvVars, body.SecretGroups)
	if err != nil {
		writeError(w, http.StatusBadGateway, "secret_resolution_failed")
		return
	}

	// Snapshot all containers in the project before the destructive operation.
	h.snapshotProjectContainers(r.Context(), nodeID, project)

	// Persist the env-var configuration for this (node, project).
	for _, ev := range body.EnvVars {
		if err := h.Stacks.SetEnvVar(r.Context(), nodeID, project, ev.Key, ev.Value, ev.SecretID); err != nil {
			h.Logger.Warn("stack: persist env var failed", "node", nodeID, "project", project, "key", ev.Key, "error", err)
		}
	}

	composeYAML := []byte(body.ComposeYAML)
	if len(composeYAML) == 0 {
		// Re-read the existing YAML when the client didn't provide updated content.
		existing, readErr := h.Stacks.ReadCompose(r.Context(), nodeID, composePath)
		if readErr != nil {
			writeError(w, http.StatusBadGateway, "read_compose_failed")
			return
		}
		composeYAML = existing
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), stackDeployTimeout)
	defer cancel()

	out, deployErr := h.Stacks.Deploy(ctx, nodeID, composePath, composeYAML, mergedEnv)

	// Audit regardless of outcome so failures are traceable.
	auditStack(r, activity.ActionStackDeploy, nodeID, project, deployErr)

	if deployErr != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  "deploy_failed",
			"output": out,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"output": out,
	})
}

// stackLifecycleTimeout bounds a whole-project stop/start/restart.
const stackLifecycleTimeout = 5 * time.Minute

// StackLifecycle runs a whole-project compose lifecycle action (stop/start/
// restart) on a node. Operator-gated (matches container start/stop/restart) +
// audited. Docker capability required; degrades to 409 on SSH-only nodes.
func (h *Handlers) StackLifecycle(w http.ResponseWriter, r *http.Request) {
	if !h.requireOperator(w, r) {
		return
	}

	nodeID := chi.URLParam(r, "id")
	project := chi.URLParam(r, "project")

	var body struct {
		Action string `json:"action"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}
	switch body.Action {
	case "stop", "start", "restart":
	default:
		writeError(w, http.StatusBadRequest, "invalid_action")
		return
	}

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "node_not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	caps, _ := capabilities.Parse([]byte(node.CapabilitiesJSON))
	if !caps.Docker {
		writeError(w, http.StatusConflict, "docker_not_available")
		return
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), stackLifecycleTimeout)
	defer cancel()

	out, lifecycleErr := h.Stacks.Lifecycle(ctx, nodeID, project, body.Action)

	auditStack(r, stackLifecycleAction(body.Action), nodeID, project, lifecycleErr)

	if lifecycleErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":  "stack_lifecycle_failed",
			"output": out,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"action":  body.Action,
		"project": project,
		"output":  out,
	})
}

// stackLifecycleAction maps a lifecycle action to its audit action constant.
func stackLifecycleAction(action string) string {
	switch action {
	case "stop":
		return activity.ActionStackStop
	case "start":
		return activity.ActionStackStart
	case "restart":
		return activity.ActionStackRestart
	default:
		return activity.ActionStackDeploy // unreachable: action is validated upstream
	}
}

// ListStackEnvVars returns the env var keys (never values) for a (node, project).
func (h *Handlers) ListStackEnvVars(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "id")
	project := chi.URLParam(r, "project")

	envVars, err := h.Stacks.ListEnvVars(r.Context(), nodeID, project)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"env_vars": makeEnvVarViews(envVars)})
}

// SetStackEnvVar upserts one env var for a (node, project). Admin-gated.
// If secret_id is provided, the var is backed by the vault and value is ignored.
func (h *Handlers) SetStackEnvVar(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	project := chi.URLParam(r, "project")

	var body struct {
		Key      string `json:"key"`
		Value    string `json:"value"`
		SecretID string `json:"secret_id"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Key == "" {
		writeError(w, http.StatusBadRequest, "key_required")
		return
	}
	if err := h.Stacks.SetEnvVar(r.Context(), nodeID, project, body.Key, body.Value, body.SecretID); err != nil {
		writeError(w, http.StatusInternalServerError, "set_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteStackEnvVar removes one env var. Admin-gated.
func (h *Handlers) DeleteStackEnvVar(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	nodeID := chi.URLParam(r, "id")
	project := chi.URLParam(r, "project")
	key := chi.URLParam(r, "key")

	if err := h.Stacks.DeleteEnvVar(r.Context(), nodeID, project, key); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- helpers ---

// mergeSecretGroups injects all secrets from the named groups into the env var
// slice. Group secrets are appended; if a key already exists in envVars it is
// not overwritten (explicit wins over group).
func (h *Handlers) mergeSecretGroups(ctx context.Context, envVars []stacks.EnvVar, groupIDs []string) ([]stacks.EnvVar, error) {
	if len(groupIDs) == 0 {
		return envVars, nil
	}
	existing := make(map[string]struct{}, len(envVars))
	for _, ev := range envVars {
		existing[ev.Key] = struct{}{}
	}

	merged := make([]stacks.EnvVar, len(envVars))
	copy(merged, envVars)

	for _, gid := range groupIDs {
		keys, err := h.Store.ListSecretKeysByGroup(ctx, gid)
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			if _, dup := existing[k.Key]; dup {
				continue
			}
			merged = append(merged, stacks.EnvVar{Key: k.Key, SecretID: k.ID, Masked: true})
			existing[k.Key] = struct{}{}
		}
	}
	return merged, nil
}

// snapshotProjectContainers takes a best-effort pre-redeploy snapshot of every
// container in the compose project. Failures are logged and not fatal; the
// deploy proceeds regardless so a compose error doesn't prevent rollback setup.
func (h *Handlers) snapshotProjectContainers(ctx context.Context, nodeID, project string) {
	ctrs, err := h.Store.ListContainersByNode(ctx, nodeID)
	if err != nil {
		return
	}
	for _, ctr := range ctrs {
		if ctr.ComposeProject != project {
			continue
		}
		snapCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		if _, err := h.Recreate.Snapshot(snapCtx, ctr.ID, "pre-stack-deploy"); err != nil {
			h.Logger.Warn("stack: pre-deploy snapshot failed",
				"container", ctr.Name, "node", nodeID, "error", err)
		}
		cancel()
	}
}

func makeEnvVarViews(vars []stacks.EnvVar) []stackEnvVarView {
	out := make([]stackEnvVarView, 0, len(vars))
	for _, v := range vars {
		out = append(out, stackEnvVarView{
			Key:      v.Key,
			SecretID: v.SecretID,
			Masked:   v.Masked,
		})
	}
	return out
}

func auditStack(r *http.Request, action, nodeID, project string, deployErr error) {
	e := activity.FromContext(r.Context())
	if e == nil {
		return
	}
	e.Action = action
	e.TargetType = ptr(activity.TargetStack)
	detail := map[string]string{"node_id": nodeID, "project": project}
	if deployErr != nil {
		detail["error"] = deployErr.Error()
	}
	e.Detail = detail
}
