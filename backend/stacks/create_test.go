// Internal tests for CreateStack validation and flow. Using package stacks
// (not stacks_test) so the unexported fileIO interface can be implemented by
// the test fake without being exported.
package stacks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	appdb "github.com/kaylaehman/stratum/backend/db"
	"github.com/kaylaehman/stratum/backend/fs"
)

// --- fake fileIO ---

type execCall struct {
	cmd  string
	args []string
}

// fakeFileIO records calls to Exec/Write/StatEntry/Preview.
type fakeFileIO struct {
	execCalls []execCall
	writes    map[string][]byte
	execErrs  map[string]error // keyed by cmd; err is returned when cmd matches
	statFound bool             // when true, StatEntry succeeds (file exists)
	statPaths []string         // records every path passed to StatEntry
}

func newFakeFileIO() *fakeFileIO {
	return &fakeFileIO{
		writes:   make(map[string][]byte),
		execErrs: make(map[string]error),
	}
}

func (f *fakeFileIO) Exec(_ context.Context, _ string, cmd string, args ...string) (string, error) {
	f.execCalls = append(f.execCalls, execCall{cmd: cmd, args: args})
	if err, ok := f.execErrs[cmd]; ok {
		return "", err
	}
	return "", nil
}

func (f *fakeFileIO) Write(_ context.Context, _ string, p string, data []byte, _ *time.Time) error {
	f.writes[p] = data
	return nil
}

func (f *fakeFileIO) StatEntry(_ context.Context, _ string, p string) (fs.Entry, error) {
	f.statPaths = append(f.statPaths, p)
	if f.statFound {
		return fs.Entry{}, nil
	}
	return fs.Entry{}, fmt.Errorf("not found")
}

func (f *fakeFileIO) Preview(_ context.Context, _ string, p string) ([]byte, bool, time.Time, error) {
	if data, ok := f.writes[p]; ok {
		return data, false, time.Time{}, nil
	}
	return nil, false, time.Time{}, fmt.Errorf("not found")
}

// --- fake store (minimal, reuses shape from service_test.go) ---

type minimalStore struct {
	appdb.Store // embed to get nil-panic on unused methods
	envVars     map[string]appdb.StackEnvRow
	secrets     map[string]appdb.SecretRow
}

func newMinimalStore() *minimalStore {
	return &minimalStore{
		envVars: make(map[string]appdb.StackEnvRow),
		secrets: make(map[string]appdb.SecretRow),
	}
}

func (m *minimalStore) UpsertStackEnvVar(_ context.Context, r appdb.StackEnvRow) error {
	m.envVars[r.NodeID+"/"+r.ProjectName+"/"+r.Key] = r
	return nil
}

func (m *minimalStore) ListStackEnvVars(_ context.Context, nodeID, project string) ([]appdb.StackEnvRow, error) {
	var out []appdb.StackEnvRow
	for _, r := range m.envVars {
		if r.NodeID == nodeID && r.ProjectName == project {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *minimalStore) DeleteStackEnvVar(_ context.Context, nodeID, project, key string) error {
	k := nodeID + "/" + project + "/" + key
	if _, ok := m.envVars[k]; !ok {
		return appdb.ErrNotFound
	}
	delete(m.envVars, k)
	return nil
}

func (m *minimalStore) GetSecret(_ context.Context, id string) (appdb.SecretRow, error) {
	r, ok := m.secrets[id]
	if !ok {
		return appdb.SecretRow{}, appdb.ErrNotFound
	}
	return r, nil
}

// buildTestService creates a Service backed by a fakeFileIO for unit tests.
func buildTestService(fio *fakeFileIO) *Service {
	return &Service{store: newMinimalStore(), files: fio, cipher: nil}
}

// --- validation table tests ---

func TestCreateStack_ProjectValidation(t *testing.T) {
	cases := []struct {
		project string
		errMsg  string // substring that must appear in error; empty = no validation error
	}{
		{"", "required"},
		{"../evil", "not allowed"},  // sanitize changes it → rejected
		{"my stack", "not allowed"}, // space → sanitize → mismatch
		{"proj.v2", "not allowed"},  // dot → mismatch
		{"valid-project_01", ""},    // ok; any error is from I/O not validation
	}

	for _, tc := range cases {
		fio := newFakeFileIO()
		svc := buildTestService(fio)
		_, _, err := svc.CreateStack(context.Background(), "node1", tc.project, "/opt/test", []byte("v: '3'"), nil)
		if tc.errMsg == "" {
			if err != nil && (strings.Contains(err.Error(), "not allowed") || strings.Contains(err.Error(), "required")) {
				t.Errorf("project=%q: unexpected validation error: %v", tc.project, err)
			}
		} else {
			if err == nil {
				t.Errorf("project=%q: expected error containing %q, got nil", tc.project, tc.errMsg)
				continue
			}
			if !strings.Contains(err.Error(), tc.errMsg) {
				t.Errorf("project=%q: error %q does not contain %q", tc.project, err.Error(), tc.errMsg)
			}
		}
	}
}

func TestCreateStack_DirectoryValidation(t *testing.T) {
	cases := []struct {
		dir     string
		wantErr bool
	}{
		{"", false},             // empty → default /opt/<project>
		{"/opt/myapp", false},   // clean absolute
		{"/opt/../etc", true},   // contains ".."
		{"relative/path", true}, // not absolute
	}

	for _, tc := range cases {
		fio := newFakeFileIO()
		svc := buildTestService(fio)
		_, _, err := svc.CreateStack(context.Background(), "node1", "myproject", tc.dir, []byte("v: '3'"), nil)
		if tc.wantErr {
			if !errors.Is(err, ErrInvalidDirectory) {
				t.Errorf("dir=%q: expected ErrInvalidDirectory, got %v", tc.dir, err)
			}
		} else {
			if errors.Is(err, ErrInvalidDirectory) {
				t.Errorf("dir=%q: got unexpected ErrInvalidDirectory", tc.dir)
			}
		}
	}
}

func TestCreateStack_DefaultDirectory(t *testing.T) {
	fio := newFakeFileIO()
	svc := buildTestService(fio)

	_, _, _ = svc.CreateStack(context.Background(), "node1", "myproject", "", []byte("v: '3'"), nil)

	for _, call := range fio.execCalls {
		if call.cmd == "mkdir" {
			for _, arg := range call.args {
				if arg == "/opt/myproject" {
					return // found
				}
			}
			t.Errorf("mkdir not called with /opt/myproject; args: %v", call.args)
			return
		}
	}
	t.Error("mkdir was never called")
}

func TestCreateStack_RejectsExistingCompose(t *testing.T) {
	fio := newFakeFileIO()
	fio.statFound = true // stat returns nil = file exists

	svc := buildTestService(fio)
	_, _, err := svc.CreateStack(context.Background(), "node1", "myproject", "/opt/myproject", []byte("v: '3'"), nil)

	if !errors.Is(err, ErrProjectExists) {
		t.Errorf("expected ErrProjectExists, got %v", err)
	}
}

func TestCreateStack_MkdirError(t *testing.T) {
	fio := newFakeFileIO()
	fio.execErrs["mkdir"] = fmt.Errorf("permission denied")

	svc := buildTestService(fio)
	_, _, err := svc.CreateStack(context.Background(), "node1", "proj", "/opt/proj", []byte("v: '3'"), nil)

	if err == nil {
		t.Error("expected error when mkdir fails, got nil")
	}
	if !strings.Contains(err.Error(), "create directory") {
		t.Errorf("error %q does not mention 'create directory'", err.Error())
	}
}

func TestCreateStack_WritesComposePath(t *testing.T) {
	fio := newFakeFileIO()
	svc := buildTestService(fio)

	yaml := []byte("services:\n  web:\n    image: nginx\n")
	composePath, _, err := svc.CreateStack(context.Background(), "node1", "myapp", "/srv/myapp", yaml, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/srv/myapp/docker-compose.yml"
	if composePath != want {
		t.Errorf("composePath = %q, want %q", composePath, want)
	}
	if got, ok := fio.writes[want]; !ok {
		t.Errorf("compose file not written to %q", want)
	} else if string(got) != string(yaml) {
		t.Error("written YAML differs from input")
	}
}

func TestCreateStack_CallsDockerComposeUp(t *testing.T) {
	fio := newFakeFileIO()
	svc := buildTestService(fio)

	_, _, _ = svc.CreateStack(context.Background(), "node1", "proj", "/opt/proj", []byte("v: '3'"), nil)

	for _, call := range fio.execCalls {
		if call.cmd != "docker" {
			continue
		}
		for _, arg := range call.args {
			if arg == "up" {
				return // found "up" in docker args — Deploy was called
			}
		}
	}
	t.Error("docker compose up was never called")
}

// TestValidateDirectory locks the pure helper used by CreateStack.
func TestValidateDirectory(t *testing.T) {
	cases := []struct {
		project string
		dir     string
		want    string
		wantErr bool
	}{
		{"myapp", "", "/opt/myapp", false},
		{"myapp", "/opt/myapp", "/opt/myapp", false},
		{"myapp", "/opt/../etc", "", true},
		{"myapp", "relative", "", true},
		{"myapp", "/opt/my/./app", "/opt/my/app", false},
	}
	for _, tc := range cases {
		got, err := validateDirectory(tc.project, tc.dir)
		if tc.wantErr {
			if err == nil {
				t.Errorf("validateDirectory(%q, %q): expected error, got %q", tc.project, tc.dir, got)
			}
		} else {
			if err != nil {
				t.Errorf("validateDirectory(%q, %q): unexpected error: %v", tc.project, tc.dir, err)
			} else if got != tc.want {
				t.Errorf("validateDirectory(%q, %q) = %q, want %q", tc.project, tc.dir, got, tc.want)
			}
		}
	}
}
