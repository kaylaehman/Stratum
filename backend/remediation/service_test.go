package remediation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
)

// --- stub store ---------------------------------------------------------------

type stubStore struct {
	proposals map[string]db.RemediationProposal
}

func newStubStore() *stubStore {
	return &stubStore{proposals: make(map[string]db.RemediationProposal)}
}

func (s *stubStore) CreateProposal(_ context.Context, p db.RemediationProposal) error {
	s.proposals[p.ID] = p
	return nil
}
func (s *stubStore) GetProposal(_ context.Context, id string) (db.RemediationProposal, error) {
	p, ok := s.proposals[id]
	if !ok {
		return db.RemediationProposal{}, db.ErrNotFound
	}
	return p, nil
}
func (s *stubStore) ListProposals(_ context.Context, _ string) ([]db.RemediationProposal, error) {
	out := make([]db.RemediationProposal, 0, len(s.proposals))
	for _, p := range s.proposals {
		out = append(out, p)
	}
	return out, nil
}
func (s *stubStore) UpdateProposalStatus(_ context.Context, id, status, approvedBy string) error {
	p, ok := s.proposals[id]
	if !ok {
		return db.ErrNotFound
	}
	p.Status = status
	p.ApprovedBy = approvedBy
	s.proposals[id] = p
	return nil
}
func (s *stubStore) UpdateProposalExecution(_ context.Context, id, status, stdout, stderr string, exitCode int) error {
	p, ok := s.proposals[id]
	if !ok {
		return db.ErrNotFound
	}
	p.Status = status
	p.Stdout = stdout
	p.Stderr = stderr
	p.ExitCode = &exitCode
	t := time.Now()
	p.ExecutedAt = &t
	s.proposals[id] = p
	return nil
}

// stubStore also needs to satisfy db.Store for the service constructor.
// Embed a nil pointer to the real store; only the 5 remediation methods are
// called in these tests so panics on other methods are intentional.
func (s *stubStore) CreateUser(_ context.Context, _ db.User) error { panic("not used") }
func (s *stubStore) GetUserByID(_ context.Context, _ string) (db.User, error) {
	panic("not used")
}
func (s *stubStore) GetUserByUsername(_ context.Context, _ string) (db.User, error) {
	panic("not used")
}
func (s *stubStore) CountUsers(_ context.Context) (int, error)                { panic("not used") }
func (s *stubStore) ListUsers(_ context.Context) ([]db.User, error)           { panic("not used") }
func (s *stubStore) UpdateUserRole(_ context.Context, _, _ string) error       { panic("not used") }
func (s *stubStore) UpdatePasswordHash(_ context.Context, _, _ string) error   { panic("not used") }
func (s *stubStore) UpdateUserProfile(_ context.Context, _, _, _ string) error { panic("not used") }
func (s *stubStore) DeleteUser(_ context.Context, _ string) error              { panic("not used") }
func (s *stubStore) CountUsersByRole(_ context.Context, _ string) (int, error) { panic("not used") }
func (s *stubStore) CreateSession(_ context.Context, _ db.Session) error       { panic("not used") }
func (s *stubStore) GetSession(_ context.Context, _ string) (db.Session, error) {
	panic("not used")
}
func (s *stubStore) RevokeSession(_ context.Context, _ string, _ time.Time) error {
	panic("not used")
}
func (s *stubStore) ListSessionsByUser(_ context.Context, _ string) ([]db.Session, error) {
	panic("not used")
}
func (s *stubStore) RevokeAllUserSessions(_ context.Context, _ string, _ time.Time) error {
	panic("not used")
}
func (s *stubStore) DeleteExpiredSessionsByUser(_ context.Context, _ string, _ time.Time) error {
	panic("not used")
}
func (s *stubStore) AppendActivity(_ context.Context, _ db.ActivityEntry) error { panic("not used") }
func (s *stubStore) ListActivity(_ context.Context, _ db.ActivityFilter) ([]db.ActivityEntry, error) {
	panic("not used")
}
func (s *stubStore) QueryActivityLog(_ context.Context, _ db.ActivityQuery) ([]db.ActivityEntry, error) {
	panic("not used")
}
func (s *stubStore) CreateNode(_ context.Context, _ db.Node) error        { panic("not used") }
func (s *stubStore) GetNode(_ context.Context, _ string) (db.Node, error) { panic("not used") }
func (s *stubStore) ListNodes(_ context.Context) ([]db.Node, error)       { panic("not used") }
func (s *stubStore) UpdateNode(_ context.Context, _ db.Node) error        { panic("not used") }
func (s *stubStore) DeleteNode(_ context.Context, _ string) error         { panic("not used") }
func (s *stubStore) UpsertVM(_ context.Context, _ db.VM) error            { panic("not used") }
func (s *stubStore) ListVMsByNode(_ context.Context, _ string) ([]db.VM, error) {
	panic("not used")
}
func (s *stubStore) DeleteVM(_ context.Context, _ string) error { panic("not used") }
func (s *stubStore) UpsertContainer(_ context.Context, _ db.Container) error { panic("not used") }
func (s *stubStore) ListContainersByNode(_ context.Context, _ string) ([]db.Container, error) {
	panic("not used")
}
func (s *stubStore) GetContainer(_ context.Context, _ string) (db.Container, error) {
	panic("not used")
}
func (s *stubStore) DeleteContainer(_ context.Context, _ string) error { panic("not used") }
func (s *stubStore) ReplaceContainerMounts(_ context.Context, _ string, _ []db.MountRow) error {
	panic("not used")
}
func (s *stubStore) ListMountsByNode(_ context.Context, _ string) ([]db.MountRow, error) {
	panic("not used")
}
func (s *stubStore) ListMountsByContainer(_ context.Context, _ string) ([]db.MountRow, error) {
	panic("not used")
}
func (s *stubStore) UpsertContainerSecurity(_ context.Context, _ db.ContainerSecurityRow) error {
	panic("not used")
}
func (s *stubStore) GetContainerSecurity(_ context.Context, _ string) (db.ContainerSecurityRow, error) {
	panic("not used")
}
func (s *stubStore) ListContainerSecurity(_ context.Context) ([]db.ContainerSecurityRow, error) {
	panic("not used")
}
func (s *stubStore) SetPortExposures(_ context.Context, _ string, _ []db.PortExposureRow) error {
	panic("not used")
}
func (s *stubStore) ListPortExposuresByContainer(_ context.Context, _ string) ([]db.PortExposureRow, error) {
	panic("not used")
}
func (s *stubStore) ListAllPortExposures(_ context.Context) ([]db.PortExposureRow, error) {
	panic("not used")
}
func (s *stubStore) InsertAck(_ context.Context, _ db.SecurityAck) error   { panic("not used") }
func (s *stubStore) DeleteAck(_ context.Context, _ string) error           { panic("not used") }
func (s *stubStore) ListAcks(_ context.Context) ([]db.SecurityAck, error) { panic("not used") }
func (s *stubStore) InsertVolumeSample(_ context.Context, _ db.VolumeSample) error {
	panic("not used")
}
func (s *stubStore) ListVolumeSamplesByNode(_ context.Context, _ string) ([]db.VolumeSample, error) {
	panic("not used")
}
func (s *stubStore) PruneVolumeSamplesBefore(_ context.Context, _ time.Time) (int64, error) {
	panic("not used")
}
func (s *stubStore) InsertResourceSample(_ context.Context, _ db.ResourceSample) error {
	panic("not used")
}
func (s *stubStore) ListResourceSamples(_ context.Context, _ string, _, _ time.Time) ([]db.ResourceSample, error) {
	panic("not used")
}
func (s *stubStore) PruneResourceSamplesBefore(_ context.Context, _ time.Time) (int64, error) {
	panic("not used")
}
func (s *stubStore) UpsertWOLConfig(_ context.Context, _ db.WOLConfig) error { panic("not used") }
func (s *stubStore) GetWOLConfig(_ context.Context, _ string) (db.WOLConfig, error) {
	panic("not used")
}
func (s *stubStore) UpsertImageUpdate(_ context.Context, _ db.ImageUpdateRow) error {
	panic("not used")
}
func (s *stubStore) ListImageUpdates(_ context.Context) ([]db.ImageUpdateRow, error) {
	panic("not used")
}
func (s *stubStore) UpsertImageScan(_ context.Context, _ db.ImageScanRow) error { panic("not used") }
func (s *stubStore) ListImageScans(_ context.Context) ([]db.ImageScanRow, error) { panic("not used") }
func (s *stubStore) GetImageScan(_ context.Context, _ string) (db.ImageScanRow, error) {
	panic("not used")
}
func (s *stubStore) ReplaceCVEResults(_ context.Context, _ string, _ []db.CVEResultRow) error {
	panic("not used")
}
func (s *stubStore) ListCVEResults(_ context.Context, _ string) ([]db.CVEResultRow, error) {
	panic("not used")
}
func (s *stubStore) UpsertUserTOTP(_ context.Context, _ db.UserTOTP) error { panic("not used") }
func (s *stubStore) GetUserTOTP(_ context.Context, _ string) (db.UserTOTP, error) {
	panic("not used")
}
func (s *stubStore) DeleteUserTOTP(_ context.Context, _ string) error { panic("not used") }
func (s *stubStore) CreateBackup(_ context.Context, _ db.BackupRow) error { panic("not used") }
func (s *stubStore) UpdateBackup(_ context.Context, _ db.BackupRow) error { panic("not used") }
func (s *stubStore) ListBackups(_ context.Context) ([]db.BackupRow, error) { panic("not used") }
func (s *stubStore) CreateSnapshot(_ context.Context, _ db.Snapshot) error { panic("not used") }
func (s *stubStore) GetSnapshot(_ context.Context, _ string) (db.Snapshot, error) {
	panic("not used")
}
func (s *stubStore) DeleteSnapshot(_ context.Context, _ string) error { panic("not used") }
func (s *stubStore) ListSnapshotsByContainer(_ context.Context, _, _ string) ([]db.Snapshot, error) {
	panic("not used")
}
func (s *stubStore) PruneSnapshots(_ context.Context, _, _ string, _ int) error { panic("not used") }
func (s *stubStore) GetAIConfig(_ context.Context) (db.AIConfig, error)          { panic("not used") }
func (s *stubStore) UpsertAIConfig(_ context.Context, _ db.AIConfig) error       { panic("not used") }
func (s *stubStore) ReplaceCertsByNode(_ context.Context, _ string, _ []db.CertInfo) error {
	panic("not used")
}
func (s *stubStore) ListCerts(_ context.Context) ([]db.CertInfo, error) { panic("not used") }
func (s *stubStore) GetProxyConfig(_ context.Context, _ string) (db.ProxyConfig, error) {
	panic("not used")
}
func (s *stubStore) UpsertProxyConfig(_ context.Context, _ db.ProxyConfig) error { panic("not used") }
func (s *stubStore) GetDNSConfig(_ context.Context, _ string) (db.DNSConfig, error) {
	panic("not used")
}
func (s *stubStore) UpsertDNSConfig(_ context.Context, _ db.DNSConfig) error { panic("not used") }
func (s *stubStore) ListFeatureFlags(_ context.Context) (map[string]bool, error) { panic("not used") }
func (s *stubStore) SetFeatureFlag(_ context.Context, _ string, _ bool) error    { panic("not used") }
func (s *stubStore) GetChatConfig(_ context.Context) (db.ChatConfig, error)      { panic("not used") }
func (s *stubStore) UpsertChatConfig(_ context.Context, _ db.ChatConfig) error   { panic("not used") }
func (s *stubStore) ListSSOConfigs(_ context.Context) ([]db.SSOConfig, error)    { panic("not used") }
func (s *stubStore) UpsertSSOConfig(_ context.Context, _ db.SSOConfig) (db.SSOConfig, error) {
	panic("not used")
}
func (s *stubStore) DeleteSSOConfig(_ context.Context, _ string) error { panic("not used") }
func (s *stubStore) CreateFileWatch(_ context.Context, _ db.FileWatch) error { panic("not used") }
func (s *stubStore) ListFileWatchesByNode(_ context.Context, _ string) ([]db.FileWatch, error) {
	panic("not used")
}
func (s *stubStore) DeleteFileWatch(_ context.Context, _ string) error { panic("not used") }
func (s *stubStore) InsertFileEvent(_ context.Context, _ db.FileEvent) error { panic("not used") }
func (s *stubStore) ListFileEvents(_ context.Context, _ string, _ int) ([]db.FileEvent, error) {
	panic("not used")
}
func (s *stubStore) CreateRunbook(_ context.Context, _ db.Runbook) error { panic("not used") }
func (s *stubStore) GetRunbook(_ context.Context, _ string) (db.Runbook, error) { panic("not used") }
func (s *stubStore) ListRunbooks(_ context.Context) ([]db.Runbook, error) { panic("not used") }
func (s *stubStore) UpdateRunbook(_ context.Context, _ db.Runbook) error  { panic("not used") }
func (s *stubStore) DeleteRunbook(_ context.Context, _ string) error      { panic("not used") }
func (s *stubStore) UpsertCustomSkill(_ context.Context, _ db.CustomSkill) error { panic("not used") }
func (s *stubStore) GetCustomSkill(_ context.Context, _ string) (db.CustomSkill, error) {
	panic("not used")
}
func (s *stubStore) ListCustomSkills(_ context.Context) ([]db.CustomSkill, error) { panic("not used") }
func (s *stubStore) DeleteCustomSkill(_ context.Context, _ string) error          { panic("not used") }
func (s *stubStore) CreateAgentMemory(_ context.Context, _ db.AgentMemory) error  { panic("not used") }
func (s *stubStore) GetAgentMemory(_ context.Context, _ string) (db.AgentMemory, error) {
	panic("not used")
}
func (s *stubStore) UpdateAgentMemory(_ context.Context, _ db.AgentMemory) error { panic("not used") }
func (s *stubStore) DeleteAgentMemory(_ context.Context, _ string) error         { panic("not used") }
func (s *stubStore) ListAgentMemory(_ context.Context, _, _ string, _ bool) ([]db.AgentMemory, error) {
	panic("not used")
}
func (s *stubStore) CreateScript(_ context.Context, _ db.Script) error   { panic("not used") }
func (s *stubStore) ListScripts(_ context.Context) ([]db.Script, error)  { panic("not used") }
func (s *stubStore) GetScript(_ context.Context, _ string) (db.Script, error) { panic("not used") }
func (s *stubStore) UpdateScript(_ context.Context, _ db.Script) error   { panic("not used") }
func (s *stubStore) DeleteScript(_ context.Context, _ string) error      { panic("not used") }
func (s *stubStore) CreateSecretGroup(_ context.Context, _ db.SecretGroup) error { panic("not used") }
func (s *stubStore) ListSecretGroups(_ context.Context) ([]db.SecretGroup, error) { panic("not used") }
func (s *stubStore) DeleteSecretGroup(_ context.Context, _ string) error          { panic("not used") }
func (s *stubStore) UpsertSecret(_ context.Context, _ db.SecretRow) error         { panic("not used") }
func (s *stubStore) ListSecretsByGroup(_ context.Context, _ string) ([]db.SecretRow, error) {
	panic("not used")
}
func (s *stubStore) ListSecretKeysByGroup(_ context.Context, _ string) ([]db.SecretRow, error) {
	panic("not used")
}
func (s *stubStore) GetSecret(_ context.Context, _ string) (db.SecretRow, error) { panic("not used") }
func (s *stubStore) DeleteSecret(_ context.Context, _ string) error               { panic("not used") }
func (s *stubStore) CreateTemplate(_ context.Context, _ db.Template) error        { panic("not used") }
func (s *stubStore) ListTemplates(_ context.Context) ([]db.Template, error)       { panic("not used") }
func (s *stubStore) GetTemplate(_ context.Context, _ string) (db.Template, error) { panic("not used") }
func (s *stubStore) UpdateTemplate(_ context.Context, _ db.Template) error        { panic("not used") }
func (s *stubStore) DeleteTemplate(_ context.Context, _ string) error             { panic("not used") }
func (s *stubStore) AddTemplateVersion(_ context.Context, _ string, _ db.TemplateVersion) error {
	panic("not used")
}
func (s *stubStore) ListTemplateVersions(_ context.Context, _ string) ([]db.TemplateVersion, error) {
	panic("not used")
}
func (s *stubStore) CreateWebhook(_ context.Context, _ db.WebhookConfig) error { panic("not used") }
func (s *stubStore) ListWebhooks(_ context.Context) ([]db.WebhookConfig, error) { panic("not used") }
func (s *stubStore) GetWebhook(_ context.Context, _ string) (db.WebhookConfig, error) {
	panic("not used")
}
func (s *stubStore) UpdateWebhook(_ context.Context, _ db.WebhookConfig) error { panic("not used") }
func (s *stubStore) DeleteWebhook(_ context.Context, _ string) error           { panic("not used") }
func (s *stubStore) CreateBookmark(_ context.Context, _ db.Bookmark) error     { panic("not used") }
func (s *stubStore) ListBookmarksByUser(_ context.Context, _ string) ([]db.Bookmark, error) {
	panic("not used")
}
func (s *stubStore) DeleteBookmark(_ context.Context, _, _ string) error { panic("not used") }
func (s *stubStore) SetBookmarkOrder(_ context.Context, _ string, _ []string) error {
	panic("not used")
}
func (s *stubStore) Close() error { return nil }

// Uptime methods (added when the uptime feature extended db.Store) — unused here.
func (s *stubStore) CreateUptimeMonitor(_ context.Context, _ db.UptimeMonitor) error { panic("not used") }
func (s *stubStore) GetUptimeMonitor(_ context.Context, _ string) (db.UptimeMonitor, error) {
	panic("not used")
}
func (s *stubStore) ListUptimeMonitors(_ context.Context) ([]db.UptimeMonitor, error) {
	panic("not used")
}
func (s *stubStore) UpdateUptimeMonitor(_ context.Context, _ db.UptimeMonitor) error { panic("not used") }
func (s *stubStore) DeleteUptimeMonitor(_ context.Context, _ string) error           { panic("not used") }
func (s *stubStore) InsertUptimeResult(_ context.Context, _ db.UptimeResult) error   { panic("not used") }
func (s *stubStore) ListUptimeResults(_ context.Context, _ string, _, _ time.Time) ([]db.UptimeResult, error) {
	panic("not used")
}
func (s *stubStore) LatestUptimeResult(_ context.Context, _ string) (db.UptimeResult, error) {
	panic("not used")
}
func (s *stubStore) PruneUptimeResultsBefore(_ context.Context, _ time.Time) (int64, error) {
	panic("not used")
}

// --- tests -------------------------------------------------------------------

func TestGenerate_CreatesProposal(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, err := svc.Generate(ctx, GenerateRequest{
		Source:   SourceDiagnostic,
		Title:    "Fix permission",
		Rationale: "container cannot read file",
		NodeID:   "node-1",
		Commands: []string{"chmod o+r /data/config"},
	}, "user-1")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if p.Status != StatusProposed {
		t.Errorf("status = %q; want %q", p.Status, StatusProposed)
	}
	if p.RiskLevel != RiskHigh {
		t.Errorf("risk = %q; want %q", p.RiskLevel, RiskHigh)
	}
	if p.CreatedBy != "user-1" {
		t.Errorf("created_by = %q; want %q", p.CreatedBy, "user-1")
	}
}

func TestApprove_RequiresProposedStatus(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo ok"},
	}, "u")

	// Approve once.
	_, err := svc.Approve(ctx, p.ID, "admin-1")
	if err != nil {
		t.Fatalf("first Approve: %v", err)
	}

	// Second approve on an already-approved proposal → invalid transition.
	_, err = svc.Approve(ctx, p.ID, "admin-1")
	if !errors.Is(err, ErrInvalidTransition) {
		t.Errorf("second Approve err = %v; want ErrInvalidTransition", err)
	}
}

func TestApprove_RejectsSelfApproval(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo ok"},
	}, "alice")

	// The creator may not approve their own proposal (separation of duties).
	_, err := svc.Approve(ctx, p.ID, "alice")
	if !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("self-approve err = %v; want ErrSelfApproval", err)
	}

	// A different user can approve it.
	if _, err := svc.Approve(ctx, p.ID, "bob"); err != nil {
		t.Errorf("approve by other user: %v", err)
	}
}

func TestReject_BlocksExecution(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceRunbook, Title: "t", NodeID: "n", Commands: []string{"echo ok"},
	}, "u")

	if _, err := svc.Reject(ctx, p.ID); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	// Execute on a rejected proposal must fail with ErrNotApproved.
	_, err := svc.Execute(ctx, p.ID)
	if !errors.Is(err, ErrNotApproved) {
		t.Errorf("Execute after Reject err = %v; want ErrNotApproved", err)
	}
}

func TestExecute_RequiresApproval(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo hello"},
	}, "u")

	// Execute without approving → must fail.
	_, err := svc.Execute(ctx, p.ID)
	if !errors.Is(err, ErrNotApproved) {
		t.Fatalf("Execute on proposed err = %v; want ErrNotApproved", err)
	}
}

func TestExecute_CapturesOutput(t *testing.T) {
	store := newStubStore()
	execFn := func(_ context.Context, _ string, _ string, _ ...string) (string, error) {
		return "hello\n", nil
	}
	svc := New(store, execFn)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo hello"},
	}, "u")
	if _, err := svc.Approve(ctx, p.ID, "admin"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	result, err := svc.Execute(ctx, p.ID)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != StatusExecuted {
		t.Errorf("status = %q; want %q", result.Status, StatusExecuted)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("stdout = %q; want %q", result.Stdout, "hello\n")
	}
}

func TestExecute_FailureRecorded(t *testing.T) {
	store := newStubStore()
	execFn := func(_ context.Context, _ string, _ string, _ ...string) (string, error) {
		return "", errors.New("permission denied")
	}
	svc := New(store, execFn)
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"cat /root/secret"},
	}, "u")
	svc.Approve(ctx, p.ID, "admin") //nolint:errcheck

	result, err := svc.Execute(ctx, p.ID)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Status != StatusFailed {
		t.Errorf("status = %q; want %q", result.Status, StatusFailed)
	}
}

func TestDestructiveCommand_ClassifiedCorrectly(t *testing.T) {
	store := newStubStore()
	svc := New(store, nil)
	ctx := context.Background()

	p, err := svc.Generate(ctx, GenerateRequest{
		Source:   SourceAI,
		Title:    "Wipe logs",
		NodeID:   "n",
		Commands: []string{"rm -rf /var/log/app"},
	}, "u")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if p.RiskLevel != RiskDestructive {
		t.Errorf("risk = %q; want %q", p.RiskLevel, RiskDestructive)
	}
	if !RequiresStepUp(p.RiskLevel) {
		t.Error("RequiresStepUp should be true for destructive proposal")
	}
}

func TestTerminalStateTransitions(t *testing.T) {
	store := newStubStore()
	svc := New(store, func(_ context.Context, _ string, _ string, _ ...string) (string, error) {
		return "ok", nil
	})
	ctx := context.Background()

	p, _ := svc.Generate(ctx, GenerateRequest{
		Source: SourceAI, Title: "t", NodeID: "n", Commands: []string{"echo done"},
	}, "u")
	svc.Approve(ctx, p.ID, "admin")   //nolint:errcheck
	svc.Execute(ctx, p.ID)            //nolint:errcheck

	// After execution the proposal is terminal; reject and approve must fail.
	_, err := svc.Reject(ctx, p.ID)
	if !errors.Is(err, ErrAlreadyTerminal) {
		t.Errorf("Reject after Execute err = %v; want ErrAlreadyTerminal", err)
	}
	_, err = svc.Approve(ctx, p.ID, "admin")
	if !errors.Is(err, ErrAlreadyTerminal) {
		t.Errorf("Approve after Execute err = %v; want ErrAlreadyTerminal", err)
	}
}
