package filewatch

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/db"
)

func TestShellQuoteNeutralizesInjection(t *testing.T) {
	cases := map[string]string{
		"/etc":            `'/etc'`,
		"/etc$(rm -rf /)": `'/etc$(rm -rf /)'`,
		"/a`whoami`":       "'/a`whoami`'",
		"/a'b":            `'/a'\''b'`,
		"/a;b|c&d":        `'/a;b|c&d'`,
	}
	for in, want := range cases {
		if got := shellQuote(in); got != want {
			t.Errorf("shellQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

// captureExec records the script the service would run so we can assert the
// malicious path is single-quoted (inert), not interpolated.
type captureExec struct{ script string }

func (c *captureExec) exec(_ context.Context, _ string, _ string, args ...string) (string, error) {
	// args = ["-c", script]
	if len(args) == 2 {
		c.script = args[1]
	}
	return "", nil
}

func TestChangedPathsQuotesMaliciousPath(t *testing.T) {
	cap := &captureExec{}
	s := &Service{exec: cap.exec}
	w := db.FileWatch{Path: "/etc$(touch /tmp/pwned)", Recursive: true}
	s.changedPaths(context.Background(), "n1", w, time.Now())

	// The dangerous substring must appear ONLY inside single quotes, so the
	// shell treats it literally — never as an unquoted command substitution.
	if !strings.Contains(cap.script, `'/etc$(touch /tmp/pwned)'`) {
		t.Fatalf("path not single-quoted in script: %q", cap.script)
	}
}
