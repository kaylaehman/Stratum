package remediation

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/KAE-Labs/stratum/backend/db"
)

// ExecFn is the SSH/agent execution signature reused from the script-runner
// path (avoids introducing a new dep on the fs package).
type ExecFn func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// execTimeout bounds a single command execution.
const execTimeout = 5 * time.Minute

// ErrNotApproved is returned when execution is attempted on a non-approved
// proposal — the primary guard against auto-execution.
var ErrNotApproved = errors.New("remediation: proposal is not in approved status")

// ErrAlreadyTerminal is returned when a state transition is attempted on a
// proposal that has already reached a terminal state.
var ErrAlreadyTerminal = errors.New("remediation: proposal is already in a terminal state")

// ErrInvalidTransition is returned when the requested status transition is not
// valid from the current status.
var ErrInvalidTransition = errors.New("remediation: invalid status transition")

// ErrSelfApproval is returned when the approver is the same user who created
// the proposal. Separation of duties: a second pair of eyes must approve any
// host command before it can be executed.
var ErrSelfApproval = errors.New("remediation: a proposal cannot be approved by its creator")

// GenerateRequest is the input for creating a new proposal.
type GenerateRequest struct {
	Source      string   // diagnostic | runbook | ai
	Title       string
	Rationale   string
	NodeID      string
	ContainerID string   // optional
	Commands    []string
}

// Service drives the proposal lifecycle.
type Service struct {
	store db.Store
	exec  ExecFn
}

// New wires the service. exec is the SSH/agent execution function from
// backend/fs so we reuse the existing SSH path (never a new dep).
func New(store db.Store, exec ExecFn) *Service {
	return &Service{store: store, exec: exec}
}

// Generate creates a new proposal from the request. The risk level is
// classified automatically; commands are never executed here.
func (s *Service) Generate(ctx context.Context, req GenerateRequest, createdBy string) (db.RemediationProposal, error) {
	if req.Title == "" {
		return db.RemediationProposal{}, fmt.Errorf("remediation: title is required")
	}
	if req.NodeID == "" {
		return db.RemediationProposal{}, fmt.Errorf("remediation: node_id is required")
	}
	if len(req.Commands) == 0 {
		return db.RemediationProposal{}, fmt.Errorf("remediation: at least one command is required")
	}
	if !validSource(req.Source) {
		return db.RemediationProposal{}, fmt.Errorf("remediation: invalid source %q", req.Source)
	}
	risk := ClassifyRisk(req.Commands)
	p := db.RemediationProposal{
		ID:          uuid.NewString(),
		Source:      req.Source,
		Title:       req.Title,
		Rationale:   req.Rationale,
		NodeID:      req.NodeID,
		ContainerID: req.ContainerID,
		Commands:    req.Commands,
		RiskLevel:   risk,
		Status:      StatusProposed,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
	}
	if err := s.store.CreateProposal(ctx, p); err != nil {
		return db.RemediationProposal{}, err
	}
	return p, nil
}

// Approve transitions a proposal from proposed → approved. Only Admin or
// Operator may approve non-destructive proposals; Admin only for destructive
// (enforced additionally at the handler layer).
func (s *Service) Approve(ctx context.Context, id, approvedBy string) (db.RemediationProposal, error) {
	p, err := s.store.GetProposal(ctx, id)
	if err != nil {
		return db.RemediationProposal{}, err
	}
	if isTerminal(p.Status) {
		return db.RemediationProposal{}, ErrAlreadyTerminal
	}
	if p.Status != StatusProposed {
		return db.RemediationProposal{}, ErrInvalidTransition
	}
	// Separation of duties: the creator may not approve their own proposal.
	if approvedBy != "" && approvedBy == p.CreatedBy {
		return db.RemediationProposal{}, ErrSelfApproval
	}
	if err := s.store.UpdateProposalStatus(ctx, id, StatusApproved, approvedBy); err != nil {
		return db.RemediationProposal{}, err
	}
	p.Status = StatusApproved
	p.ApprovedBy = approvedBy
	return p, nil
}

// Reject transitions a proposal from proposed → rejected.
func (s *Service) Reject(ctx context.Context, id string) (db.RemediationProposal, error) {
	p, err := s.store.GetProposal(ctx, id)
	if err != nil {
		return db.RemediationProposal{}, err
	}
	if isTerminal(p.Status) {
		return db.RemediationProposal{}, ErrAlreadyTerminal
	}
	if p.Status != StatusProposed {
		return db.RemediationProposal{}, ErrInvalidTransition
	}
	if err := s.store.UpdateProposalStatus(ctx, id, StatusRejected, ""); err != nil {
		return db.RemediationProposal{}, err
	}
	p.Status = StatusRejected
	return p, nil
}

// Execute runs the commands on the target node via SSH. It REQUIRES the
// proposal to be in StatusApproved — this is the primary no-auto-execute
// guard. The handler also verifies step-up for destructive proposals.
func (s *Service) Execute(ctx context.Context, id string) (db.RemediationProposal, error) {
	p, err := s.store.GetProposal(ctx, id)
	if err != nil {
		return db.RemediationProposal{}, err
	}
	// Primary safety gate: only approved proposals may be executed.
	if p.Status != StatusApproved {
		return db.RemediationProposal{}, ErrNotApproved
	}

	// Bound execution so a hung remediation command cannot run unbounded.
	execCtx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	combined := buildScript(p.Commands)
	b64 := base64.StdEncoding.EncodeToString([]byte(combined))
	cmd := "printf '%s' '" + b64 + "' | base64 -d | sh 2>&1"

	stdout, execErr := s.exec(execCtx, p.NodeID, "sh", "-c", cmd)

	exitCode := 0
	status := StatusExecuted
	stderr := ""
	if execErr != nil {
		exitCode = 1
		status = StatusFailed
		stderr = execErr.Error()
	}

	if dbErr := s.store.UpdateProposalExecution(ctx, id, status, stdout, stderr, exitCode); dbErr != nil {
		return db.RemediationProposal{}, dbErr
	}
	p.Status = status
	p.Stdout = stdout
	p.Stderr = stderr
	p.ExitCode = &exitCode
	return p, nil
}

// Get returns a single proposal.
func (s *Service) Get(ctx context.Context, id string) (db.RemediationProposal, error) {
	return s.store.GetProposal(ctx, id)
}

// List returns proposals; pass empty nodeID for all.
func (s *Service) List(ctx context.Context, nodeID string) ([]db.RemediationProposal, error) {
	return s.store.ListProposals(ctx, nodeID)
}

// buildScript joins commands with newlines so they run as a single script,
// preserving individual exit codes via set -e.
func buildScript(commands []string) string {
	parts := make([]string, 0, len(commands)+1)
	parts = append(parts, "set -e")
	parts = append(parts, commands...)
	return strings.Join(parts, "\n")
}

func validSource(s string) bool {
	return s == SourceDiagnostic || s == SourceRunbook || s == SourceAI
}

func isTerminal(status string) bool {
	return status == StatusRejected || status == StatusExecuted || status == StatusFailed
}
