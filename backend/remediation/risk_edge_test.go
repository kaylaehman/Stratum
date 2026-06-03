package remediation

// risk_edge_test.go — edge-case tests for the risk classifier, the
// low-risk allowlist, RequiresStepUp logic, and audit-action constants.
//
// Covers gaps NOT present in risk_test.go and service_test.go:
// - allowlist boundary cases (commands that look similar but must NOT be low)
// - sensitive-path interaction with allowlisted read-only commands
// - whitespace-padded and case-variant commands
// - riskRank ordering invariants
// - RequiresStepUp exhaustive coverage including unknown levels (fail-closed)
// - all low-risk allowlist entry classes have at least one positive test
// - audit action/target constants present in the activity catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/kaylaehman/stratum/backend/activity"
)

// ---------------------------------------------------------------------------
// Allowlist boundary: commands that look similar to allowlisted ones but must
// NOT be classified low-risk.
// ---------------------------------------------------------------------------

func TestAllowlistBoundary_DockerRun_IsNotLow(t *testing.T) {
	// "docker run" is not on the allowlist (it starts a new container with
	// arbitrary image and config) — must require step-up.
	risk := ClassifyRisk([]string{"docker run --privileged myimage"})
	if risk == RiskLow {
		t.Errorf("docker run should not be RiskLow, got %q", risk)
	}
	if !RequiresStepUp(risk) {
		t.Errorf("docker run (risk %q) should require step-up", risk)
	}
}

func TestAllowlistBoundary_DockerRm_IsNotLow(t *testing.T) {
	// "docker rm" removes a container — not on the allowlist.
	risk := ClassifyRisk([]string{"docker rm myapp"})
	if risk == RiskLow {
		t.Errorf("docker rm should not be RiskLow, got %q", risk)
	}
}

func TestAllowlistBoundary_DockerExec_IsNotLow(t *testing.T) {
	// "docker exec" opens a shell inside a container — arbitrary code exec,
	// not on the allowlist.
	risk := ClassifyRisk([]string{"docker exec -it myapp sh"})
	if risk == RiskLow {
		t.Errorf("docker exec should not be RiskLow, got %q", risk)
	}
}

func TestAllowlistBoundary_SystemctlStop_IsHigh(t *testing.T) {
	// "systemctl stop" is not on the restart allowlist — stopping (not
	// restarting) a service is a one-way operation, must require step-up.
	risk := ClassifyRisk([]string{"systemctl stop nginx"})
	if risk != RiskHigh {
		t.Errorf("systemctl stop: got %q; want RiskHigh", risk)
	}
	if !RequiresStepUp(risk) {
		t.Errorf("systemctl stop (risk %q) must require step-up", risk)
	}
}

func TestAllowlistBoundary_SystemctlDisable_IsHigh(t *testing.T) {
	// "systemctl disable" is not allowlisted.
	risk := ClassifyRisk([]string{"systemctl disable nginx"})
	if risk != RiskHigh {
		t.Errorf("systemctl disable: got %q; want RiskHigh", risk)
	}
}

func TestAllowlistBoundary_DockerPull_IsNotLow(t *testing.T) {
	// "docker pull" is not explicitly on the allowlist (it mutates the local
	// image cache and is not a read-only inspection verb).
	risk := ClassifyRisk([]string{"docker pull myimage:latest"})
	if risk == RiskLow {
		t.Errorf("docker pull should not be RiskLow, got %q", risk)
	}
}

// ---------------------------------------------------------------------------
// Sensitive path: allowlisted read-only commands touching /etc are NOT low.
// ---------------------------------------------------------------------------

func TestSensitivePath_CatEtcPasswd_IsNotLow(t *testing.T) {
	// "cat" is on the allowlist but /etc is sensitive — must be >= high.
	risk := ClassifyRisk([]string{"cat /etc/passwd"})
	if risk == RiskLow {
		t.Errorf("cat /etc/passwd should not be RiskLow (sensitive path), got %q", risk)
	}
	if !RequiresStepUp(risk) {
		t.Errorf("cat /etc/passwd (risk %q) must require step-up", risk)
	}
}

func TestSensitivePath_CatEtcShadow_IsNotLow(t *testing.T) {
	risk := ClassifyRisk([]string{"cat /etc/shadow"})
	if risk == RiskLow {
		t.Errorf("cat /etc/shadow should not be RiskLow (sensitive path), got %q", risk)
	}
}

func TestSensitivePath_LsRootDir_IsNotLow(t *testing.T) {
	// "ls" is allowlisted but listing /root is sensitive.
	risk := ClassifyRisk([]string{"ls /root"})
	if risk == RiskLow {
		t.Errorf("ls /root should not be RiskLow (sensitive path), got %q", risk)
	}
}

func TestSensitivePath_StatBoot_IsNotLow(t *testing.T) {
	// "stat" is allowlisted but /boot is sensitive.
	risk := ClassifyRisk([]string{"stat /boot/vmlinuz"})
	if risk == RiskLow {
		t.Errorf("stat /boot/vmlinuz should not be RiskLow (sensitive path), got %q", risk)
	}
}

func TestSensitivePath_TailVarLog_IsHigh(t *testing.T) {
	// /var/log IS a sensitive path: even an allowlisted read like `tail` is not
	// auto-approvable there (logs can hold secrets/tokens), so it falls through to
	// RiskHigh and requires step-up rather than running autonomously.
	risk := ClassifyRisk([]string{"tail /var/log/syslog"})
	if risk != RiskHigh {
		t.Errorf("tail /var/log/syslog: got %q; want RiskHigh (/var/log is a sensitive path)", risk)
	}
}

func TestSensitivePath_JournalctlNoPath_IsLow(t *testing.T) {
	// journalctl is on the allowlist and does not reference a raw filesystem
	// path — must be low risk.
	risk := ClassifyRisk([]string{"journalctl -u nginx --since '1h ago'"})
	if risk != RiskLow {
		t.Errorf("journalctl: got %q; want RiskLow", risk)
	}
}

// ---------------------------------------------------------------------------
// Whitespace padding and case variation.
// ---------------------------------------------------------------------------

func TestWhitespacePadded_DockerRestart_IsLow(t *testing.T) {
	// Leading/trailing whitespace is trimmed before classification.
	risk := ClassifyRisk([]string{"  docker restart myapp  "})
	if risk != RiskLow {
		t.Errorf("whitespace-padded docker restart: got %q; want RiskLow", risk)
	}
}

func TestCaseVariant_DockerRestart_IsLow(t *testing.T) {
	// The allowlist is compiled case-insensitively.
	risk := ClassifyRisk([]string{"Docker Restart myapp"})
	if risk != RiskLow {
		t.Errorf("mixed-case docker restart: got %q; want RiskLow", risk)
	}
}

func TestCaseVariant_RmRf_IsDestructive(t *testing.T) {
	// Uppercase variant must still classify destructive.
	risk := ClassifyRisk([]string{"RM -RF /important"})
	if risk != RiskDestructive {
		t.Errorf("uppercase RM -RF: got %q; want RiskDestructive", risk)
	}
}

// ---------------------------------------------------------------------------
// Empty / comment commands: never add risk.
// ---------------------------------------------------------------------------

func TestCommentContainingDangerousText_IsLow(t *testing.T) {
	// A comment body containing a dangerous command pattern must not be
	// classified as destructive — the comment check runs before metacharacter
	// and denylist checks.
	risk := ClassifyRisk([]string{"# chmod -R 777 /etc"})
	if risk != RiskLow {
		t.Errorf("comment containing dangerous text: got %q; want RiskLow", risk)
	}
}

func TestMultipleCommands_EmptyDoesNotLowerRisk(t *testing.T) {
	// An empty command in a list should not dilute a high classification.
	risk := ClassifyRisk([]string{"", "rm -rf /var/junk", ""})
	if risk != RiskDestructive {
		t.Errorf("got %q; want RiskDestructive (rm -rf is destructive)", risk)
	}
}

// ---------------------------------------------------------------------------
// riskRank ordering: verify internal ordering invariants.
// ---------------------------------------------------------------------------

func TestRiskRank_Ordering(t *testing.T) {
	cases := []struct {
		lower, higher string
	}{
		{RiskLow, RiskMedium},
		{RiskMedium, RiskHigh},
		{RiskHigh, RiskDestructive},
		{RiskLow, RiskHigh},
		{RiskLow, RiskDestructive},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.lower+"_lt_"+tc.higher, func(t *testing.T) {
			if riskRank(tc.lower) >= riskRank(tc.higher) {
				t.Errorf("riskRank(%q) should be < riskRank(%q)", tc.lower, tc.higher)
			}
		})
	}
}

func TestRiskRank_UnknownIsZero(t *testing.T) {
	if riskRank("bogus") != 0 {
		t.Error("unknown risk level should rank 0")
	}
}

// ---------------------------------------------------------------------------
// RequiresStepUp: fail-closed behaviour — unknown levels must require step-up.
// ---------------------------------------------------------------------------

func TestRequiresStepUp_UnknownLevel_FailsClosed(t *testing.T) {
	// An unrecognised risk level is not RiskLow, so it must require step-up
	// (fail-closed: anything not positively identified as low is gated).
	if !RequiresStepUp("unknown_risk_level") {
		t.Error("unrecognised risk level should require step-up (fail-closed)")
	}
}

func TestRequiresStepUp_EmptyString_FailsClosed(t *testing.T) {
	if !RequiresStepUp("") {
		t.Error("empty risk level should require step-up (fail-closed)")
	}
}

// ---------------------------------------------------------------------------
// Allowlist positive coverage: every entry class has at least one test.
// ---------------------------------------------------------------------------

func TestAllowlist_DockerComposeRestart_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"docker compose restart"})
	if risk != RiskLow {
		t.Errorf("docker compose restart: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_DockerComposeStop_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"docker compose stop"})
	if risk != RiskLow {
		t.Errorf("docker compose stop: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_DockerImageLs_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"docker image ls"})
	if risk != RiskLow {
		t.Errorf("docker image ls: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_DockerComposeLogs_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"docker compose logs --tail 50 app"})
	if risk != RiskLow {
		t.Errorf("docker compose logs: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_ServiceRestart_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"service nginx restart"})
	if risk != RiskLow {
		t.Errorf("service restart: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_SystemctlIsActive_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"systemctl is-active nginx"})
	if risk != RiskLow {
		t.Errorf("systemctl is-active: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_SystemctlListUnits_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"systemctl list-units --type=service"})
	if risk != RiskLow {
		t.Errorf("systemctl list-units: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_Uptime_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"uptime"})
	if risk != RiskLow {
		t.Errorf("uptime: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_GetFacl_NonSensitivePath_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"getfacl /data/config"})
	if risk != RiskLow {
		t.Errorf("getfacl /data/config: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_FindMnt_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"findmnt"})
	if risk != RiskLow {
		t.Errorf("findmnt: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_SSNetstat_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"ss -tlnp"})
	if risk != RiskLow {
		t.Errorf("ss -tlnp: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_DockerPause_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"docker pause myapp"})
	if risk != RiskLow {
		t.Errorf("docker pause: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_DockerUnpause_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"docker unpause myapp"})
	if risk != RiskLow {
		t.Errorf("docker unpause: got %q; want RiskLow", risk)
	}
}

func TestAllowlist_EchoWithArgs_IsLow(t *testing.T) {
	risk := ClassifyRisk([]string{"echo hello world"})
	if risk != RiskLow {
		t.Errorf("echo with args: got %q; want RiskLow", risk)
	}
}

// ---------------------------------------------------------------------------
// Audit action constants: ensure all 5 remediation actions are defined in
// the activity catalog (integration check — no HTTP server needed).
// ---------------------------------------------------------------------------

func TestActivityCatalog_RemediationActionsPresent(t *testing.T) {
	cases := []struct {
		action string
	}{
		{activity.ActionRemediationGenerated},
		{activity.ActionRemediationApproved},
		{activity.ActionRemediationRejected},
		{activity.ActionRemediationExecuted},
		{activity.ActionRemediationFailed},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.action, func(t *testing.T) {
			info, ok := activity.LookupAction(tc.action)
			if !ok {
				t.Errorf("action %q missing from activity catalog", tc.action)
				return
			}
			if info.Target != activity.TargetRemediation {
				t.Errorf("action %q target = %q; want %q", tc.action, info.Target, activity.TargetRemediation)
			}
			if info.Category != "remediation" {
				t.Errorf("action %q category = %q; want %q", tc.action, info.Category, "remediation")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Service: proposal lifecycle with risk verification.
// ---------------------------------------------------------------------------

// TestGenerate_RiskAttachedToProposal verifies that the risk level embedded in
// the generated proposal is consistent with ClassifyRisk output.
func TestGenerate_RiskAttachedToProposal(t *testing.T) {
	cases := []struct {
		name     string
		commands []string
		wantRisk string
	}{
		{"low risk auto-heal", []string{"docker restart app"}, RiskLow},
		{"high risk chown", []string{"chown 999 /data/config"}, RiskHigh},
		{"destructive rm -rf", []string{"rm -rf /var/logs"}, RiskDestructive},
		{"multi command highest wins", []string{"docker restart app", "rm -rf /tmp/junk"}, RiskDestructive},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			store := newStubStore()
			svc := New(store, nil)

			p, err := svc.Generate(context.Background(), GenerateRequest{
				Source:   SourceDiagnostic,
				Title:    "test",
				NodeID:   "node-1",
				Commands: tc.commands,
			}, "user-1")
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if p.RiskLevel != tc.wantRisk {
				t.Errorf("risk = %q; want %q", p.RiskLevel, tc.wantRisk)
			}
			// RequiresStepUp must be consistent: low does not, everything else does.
			wantStepUp := tc.wantRisk != RiskLow
			if RequiresStepUp(p.RiskLevel) != wantStepUp {
				t.Errorf("RequiresStepUp(%q) = %v; want %v", p.RiskLevel, RequiresStepUp(p.RiskLevel), wantStepUp)
			}
		})
	}
}

// TestGenerate_ValidationErrors checks that missing required fields are
// rejected before any risk classification or DB write occurs.
func TestGenerate_ValidationErrors(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	cases := []struct {
		name string
		req  GenerateRequest
	}{
		{"missing title", GenerateRequest{Source: SourceAI, NodeID: "n", Commands: []string{"echo ok"}}},
		{"missing node_id", GenerateRequest{Source: SourceAI, Title: "t", Commands: []string{"echo ok"}}},
		{"missing commands", GenerateRequest{Source: SourceAI, Title: "t", NodeID: "n"}},
		{"invalid source", GenerateRequest{Source: "unknown", Title: "t", NodeID: "n", Commands: []string{"echo ok"}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Generate(ctx, tc.req, "u")
			if err == nil {
				t.Error("want error for invalid request; got nil")
			}
		})
	}
}

// TestApprove_SetsApprovedBy verifies that the approver identity is stored on
// the proposal.
func TestApprove_SetsApprovedBy(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo ok"},
	}, "alice")
	approved, err := svc.Approve(ctx, p.ID, "bob")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approved.ApprovedBy != "bob" {
		t.Errorf("approved_by = %q; want %q", approved.ApprovedBy, "bob")
	}
	if approved.Status != StatusApproved {
		t.Errorf("status = %q; want %q", approved.Status, StatusApproved)
	}
}

// TestReject_CannotRejectAlreadyRejected verifies double-reject returns a terminal error.
func TestReject_CannotRejectAlreadyRejected(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo ok"},
	}, "u")
	svc.Reject(ctx, p.ID) //nolint:errcheck

	_, err := svc.Reject(ctx, p.ID)
	if !errors.Is(err, ErrAlreadyTerminal) {
		t.Errorf("double-reject err = %v; want ErrAlreadyTerminal", err)
	}
}

// TestExecute_CannotExecuteRejected verifies a rejected proposal is not executable.
func TestExecute_CannotExecuteRejected(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo ok"},
	}, "u")
	svc.Reject(ctx, p.ID) //nolint:errcheck

	_, err := svc.Execute(ctx, p.ID)
	if !errors.Is(err, ErrNotApproved) {
		t.Errorf("Execute on rejected err = %v; want ErrNotApproved", err)
	}
}

// TestApprove_BlockedWhenTerminalAfterExecution confirms the terminal guard
// fires after a proposal has been successfully executed.
func TestApprove_BlockedWhenTerminalAfterExecution(t *testing.T) {
	store := newStubStore()
	exec := func(_ context.Context, _ string, _ string, _ ...string) (string, error) {
		return "ok", nil
	}
	svc := New(store, exec)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo done"},
	}, "u")
	svc.Approve(ctx, p.ID, "admin") //nolint:errcheck
	svc.Execute(ctx, p.ID)          //nolint:errcheck

	_, err := svc.Approve(ctx, p.ID, "admin")
	if !errors.Is(err, ErrAlreadyTerminal) {
		t.Errorf("Approve after Execute err = %v; want ErrAlreadyTerminal", err)
	}
}
