package orchestration

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/depgraph"
)

// Service is the top-level orchestration service wired into the API layer.
type Service struct {
	planner  *Planner
	executor *Executor
	store    StoreReader
	logger   *slog.Logger
}

// NewService constructs the orchestration Service.
func NewService(
	store StoreReader,
	conn ConnManager,
	graphs GraphProvider,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		planner:  NewPlanner(store, graphs),
		executor: NewExecutor(store, conn),
		store:    store,
		logger:   logger,
	}
}

// PlanRequest describes what the caller wants to plan or execute.
type PlanRequest struct {
	TargetKind string // "stack" | "node"
	TargetID   string // nodeID for "node"; for "stack" use NodeID + Project
	NodeID     string // required when TargetKind == "stack"
	Project    string // required when TargetKind == "stack"
	Action     string // "start" | "stop" | "restart" | "drain"
	DryRun     bool
}

// Plan builds (but does not execute) the ordered plan.
func (s *Service) Plan(ctx context.Context, req PlanRequest) (Plan, error) {
	if err := validateRequest(req); err != nil {
		return Plan{}, err
	}
	plan, err := s.buildPlan(ctx, req)
	if err != nil {
		return Plan{}, err
	}
	s.logCycles(plan)
	return plan, nil
}

// Execute builds the plan and runs it. Returns the step results.
// On the first failing step, execution stops and the partial results are returned.
func (s *Service) Execute(ctx context.Context, req PlanRequest) ([]StepResult, error) {
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	plan, err := s.buildPlan(ctx, req)
	if err != nil {
		return nil, err
	}
	s.logCycles(plan)
	if req.DryRun {
		return nil, fmt.Errorf("orchestration: dry_run set on Execute call")
	}
	return s.executor.Execute(ctx, plan)
}

func (s *Service) buildPlan(ctx context.Context, req PlanRequest) (Plan, error) {
	switch req.TargetKind {
	case "stack":
		return s.planner.PlanStack(ctx, req.NodeID, req.Project, req.Action)
	case "node":
		action := req.Action
		if action == "drain" {
			action = "drain"
		}
		return s.planner.PlanNode(ctx, req.TargetID, action)
	default:
		return Plan{}, fmt.Errorf("orchestration: unknown target_kind %q", req.TargetKind)
	}
}

func validateRequest(req PlanRequest) error {
	switch req.TargetKind {
	case "stack":
		if req.NodeID == "" || req.Project == "" {
			return fmt.Errorf("orchestration: stack target requires node_id and project")
		}
	case "node":
		if req.TargetID == "" {
			return fmt.Errorf("orchestration: node target requires target_id")
		}
	default:
		return fmt.Errorf("orchestration: target_kind must be \"stack\" or \"node\"")
	}
	switch req.Action {
	case "start", "stop", "restart", "drain":
	default:
		return fmt.Errorf("orchestration: action must be start/stop/restart/drain")
	}
	return nil
}

func (s *Service) logCycles(plan Plan) {
	for _, cycle := range plan.Cycles {
		s.logger.Warn("orchestration: dependency cycle detected",
			"target", plan.Target,
			"cycle", cycle,
		)
	}
}

// Compile-time check: db.Store satisfies StoreReader (methods used are a subset).
var _ StoreReader = (db.Store)(nil)

// Compile-time check: *depgraph.Service satisfies GraphProvider.
var _ GraphProvider = (*depgraph.Service)(nil)
