package api

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRunCommandRoundTrip(t *testing.T) {
	content := "echo hi\nrm -rf './a dir'\n# a comment with 'quotes' and $vars"
	cmd := runCommand(content)
	// The command embeds the content only as base64 (no raw content), so single
	// quotes / $ / metacharacters in the script can't break out of the shell line.
	if strings.Contains(cmd, "rm -rf") {
		t.Fatal("raw script content leaked into the command line (injection risk)")
	}
	if !strings.HasSuffix(cmd, "| base64 -d | sh 2>&1") {
		t.Errorf("command tail unexpected: %q", cmd)
	}
	// Extract the base64 (the second single-quoted token) and confirm it decodes.
	marker := "printf '%s' '"
	start := strings.Index(cmd, marker) + len(marker)
	end := strings.Index(cmd[start:], "'") + start
	b64 := cmd[start:end]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("embedded payload is not valid base64: %v", err)
	}
	if string(decoded) != content {
		t.Errorf("decoded payload = %q, want original content", decoded)
	}
}
