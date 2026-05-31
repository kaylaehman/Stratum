package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/orchestration"
)

// Orchestration-specific audit action constants.
// The orchestrator must add these to activity/actions.go; see output snippets.
const (
	actionOrchestrationExecute = "orchestration.execute"
	actionNodeDrain             = "node.drain"
)

// planRequest is the JSON body for /api/orchestration/plan and /execute.
type planRequest struct {
	TargetKind string `json:"target_kind"` // "stack" | "node"
	TargetID   string `json:"target_id"`   // node ID when kind==node
	NodeID     string `json:"node_id"`     // required when kind==stack
	Project    string `json:"project"`     // required when kind==stack
	Action     string `json:"action"`      // start | stop | restart
	DryRun     bool   `json:"dry_run"`
}

// OrchestrationPlan handles POST /api/orchestration/plan.
// Operator-gated for start/restart; admin-gated for stop.
// Always dry-run (returns Plan without executing).
func (h *Handlers) OrchestrationPlan(w http.ResponseWriter, r *http.Request) {
	var body planRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	if body.Action == "stop" {
		if !h.requireAdmin(w, r) {
			return
		}
	} else {
		if !h.requireOperator(w, r) {
			return
		}
	}

	req := toServiceRequest(body)
	req.DryRun = true
	plan, err := h.Orchestration.Plan(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// OrchestrationExecute handles POST /api/orchestration/execute.
// Operator-gated for start/restart; admin+step-up for stop.
func (h *Handlers) OrchestrationExecute(w http.ResponseWriter, r *http.Request) {
	var body planRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body")
		return
	}

	if body.Action == "stop" {
		if !h.requireAdmin(w, r) {
			return
		}
		if !h.requireStepUp(w, r) {
			return
		}
	} else {
		if !h.requireOperator(w, r) {
			return
		}
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = actionOrchestrationExecute
		e.Detail = map[string]any{
			"target_kind": body.TargetKind,
			"target_id":   body.TargetID,
			"project":     body.Project,
			"action":      body.Action,
		}
	}

	req := toServiceRequest(body)
	results, err := h.Orchestration.Execute(r.Context(), req)
	if err != nil && results == nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Return partial results even on failure.
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// NodeDrain handles POST /api/nodes/{id}/drain.
// Admin + step-up required. Stops all guests/containers in dependency order.
func (h *Handlers) NodeDrain(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.requireStepUp(w, r) {
		return
	}

	nodeID := chi.URLParam(r, "id")

	var body struct {
		DryRun bool `json:"dry_run"`
	}
	// Body is optional; ignore decode errors (empty body = execute).
	_ = decodeJSON(r, &body)

	req := orchestration.PlanRequest{
		TargetKind: "node",
		TargetID:   nodeID,
		Action:     "drain",
		DryRun:     body.DryRun,
	}

	if body.DryRun {
		plan, err := h.Orchestration.Plan(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, plan)
		return
	}

	if e := activity.FromContext(r.Context()); e != nil {
		e.Action = actionNodeDrain
		e.TargetType = ptr(activity.TargetNode)
		e.TargetID = &nodeID
		e.Detail = map[string]string{"node_id": nodeID}
	}

	results, err := h.Orchestration.Execute(r.Context(), req)
	if err != nil && results == nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func toServiceRequest(b planRequest) orchestration.PlanRequest {
	return orchestration.PlanRequest{
		TargetKind: b.TargetKind,
		TargetID:   b.TargetID,
		NodeID:     b.NodeID,
		Project:    b.Project,
		Action:     b.Action,
	}
}
