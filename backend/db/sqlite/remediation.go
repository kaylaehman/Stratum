package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	appdb "github.com/KAE-Labs/stratum/backend/db"
)

const proposalColumns = `id, source, title, rationale, node_id, container_id,
	commands_json, risk_level, status, created_by, approved_by,
	stdout, stderr, exit_code, created_at, approved_at, executed_at`

func (s *Store) CreateProposal(ctx context.Context, p appdb.RemediationProposal) error {
	now := time.Now()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	cmds, _ := json.Marshal(p.Commands)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO remediation_proposals
			(id, source, title, rationale, node_id, container_id,
			 commands_json, risk_level, status, created_by, approved_by,
			 stdout, stderr, exit_code, created_at, approved_at, executed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Source, p.Title, p.Rationale,
		nullableEmpty(p.NodeID), nullableEmpty(p.ContainerID),
		string(cmds), p.RiskLevel, p.Status, p.CreatedBy,
		nil, nil, nil, nil,
		tsText(p.CreatedAt), nil, nil)
	if err != nil {
		return fmt.Errorf("sqlite: create proposal: %w", err)
	}
	return nil
}

func (s *Store) GetProposal(ctx context.Context, id string) (appdb.RemediationProposal, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+proposalColumns+` FROM remediation_proposals WHERE id = ?`, id)
	p, err := scanProposalRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return appdb.RemediationProposal{}, appdb.ErrNotFound
	}
	return p, err
}

func (s *Store) ListProposals(ctx context.Context, nodeID string) ([]appdb.RemediationProposal, error) {
	q := `SELECT ` + proposalColumns + ` FROM remediation_proposals`
	var args []any
	if nodeID != "" {
		q += ` WHERE node_id = ?`
		args = append(args, nodeID)
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list proposals: %w", err)
	}
	defer rows.Close()
	var out []appdb.RemediationProposal
	for rows.Next() {
		p, err := scanProposalRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdateProposalStatus(ctx context.Context, id, status, approvedBy string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE remediation_proposals
		 SET status = ?, approved_by = ?, approved_at = ?
		 WHERE id = ?`,
		status, nullableEmpty(approvedBy), tsText(time.Now()), id)
	if err != nil {
		return fmt.Errorf("sqlite: update proposal status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func (s *Store) UpdateProposalExecution(ctx context.Context, id, status, stdout, stderr string, exitCode int) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE remediation_proposals
		 SET status = ?, stdout = ?, stderr = ?, exit_code = ?, executed_at = ?
		 WHERE id = ?`,
		status, stdout, stderr, exitCode, tsText(time.Now()), id)
	if err != nil {
		return fmt.Errorf("sqlite: update proposal execution: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return appdb.ErrNotFound
	}
	return nil
}

func scanProposalRow(row *sql.Row) (appdb.RemediationProposal, error) {
	return scanProposalRows(row)
}

func scanProposalRows(sc rowScanner) (appdb.RemediationProposal, error) {
	var p appdb.RemediationProposal
	var nodeID, containerID, approvedBy, stdout, stderr sql.NullString
	var exitCode sql.NullInt64
	var cmdsJSON, createdAt string
	var approvedAt, executedAt sql.NullString

	err := sc.Scan(
		&p.ID, &p.Source, &p.Title, &p.Rationale,
		&nodeID, &containerID, &cmdsJSON,
		&p.RiskLevel, &p.Status, &p.CreatedBy, &approvedBy,
		&stdout, &stderr, &exitCode,
		&createdAt, &approvedAt, &executedAt,
	)
	if err != nil {
		return appdb.RemediationProposal{}, err
	}

	p.NodeID = nodeID.String
	p.ContainerID = containerID.String
	p.ApprovedBy = approvedBy.String
	p.Stdout = stdout.String
	p.Stderr = stderr.String
	if exitCode.Valid {
		v := int(exitCode.Int64)
		p.ExitCode = &v
	}
	_ = json.Unmarshal([]byte(cmdsJSON), &p.Commands)
	if p.Commands == nil {
		p.Commands = []string{}
	}
	p.CreatedAt, _ = parseTS(createdAt)
	if approvedAt.Valid && approvedAt.String != "" {
		t, _ := parseTS(approvedAt.String)
		p.ApprovedAt = &t
	}
	if executedAt.Valid && executedAt.String != "" {
		t, _ := parseTS(executedAt.String)
		p.ExecutedAt = &t
	}
	return p, nil
}
