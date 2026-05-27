package ssh

import (
	"bytes"
	"context"
	"strings"

	"golang.org/x/crypto/ssh"
)

// This is the platform-wide SSH command-exec module. NO SSH command may be
// constructed anywhere else: every argument flows through shellSingleQuote, so
// shell metacharacters can never inject, and callers pass "--" before any
// path/user-controlled argument so a leading '-' can't be read as a flag.

// shellSingleQuote wraps s in single quotes for safe POSIX shell use, escaping
// any embedded single quote as '\''. The result is a single shell word.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// QuoteArg is the exported form of shellSingleQuote.
func QuoteArg(s string) string { return shellSingleQuote(s) }

// buildCommandLine assembles a shell command line with every token
// single-quoted. Tested directly with adversarial inputs.
func buildCommandLine(cmd string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, shellSingleQuote(cmd))
	for _, a := range args {
		parts = append(parts, shellSingleQuote(a))
	}
	return strings.Join(parts, " ")
}

// Run executes cmd with args over the SSH client and returns stdout. Every token
// is single-quoted; callers must place "--" (as an arg) before any path or
// user-controlled argument. Returns the combined stdout; a non-zero exit yields
// an error along with whatever stdout was produced.
func Run(ctx context.Context, client *ssh.Client, cmd string, args ...string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	line := buildCommandLine(cmd, args)
	done := make(chan error, 1)
	go func() { done <- session.Run(line) }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return "", ctx.Err()
	case err := <-done:
		return stdout.String(), err
	}
}
