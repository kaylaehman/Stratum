package ssh

import (
	"strings"
	"testing"
)

func TestShellSingleQuoteNeutralizesInjection(t *testing.T) {
	adversarial := []string{
		"; rm -rf /",
		"`reboot`",
		"$(curl evil)",
		"a' ; rm -rf / ; '",
		"line1\nline2",
		"--flag-injection",
		"-rf",
		"path with spaces",
		"$HOME",
		"a&&b||c|d",
	}
	for _, in := range adversarial {
		q := shellSingleQuote(in)
		// Must start and end with a single quote.
		if !strings.HasPrefix(q, "'") || !strings.HasSuffix(q, "'") {
			t.Errorf("shellSingleQuote(%q) = %q: not wrapped in single quotes", in, q)
		}
		// The only way out of single quotes is the '\'' sequence. Strip those,
		// then there must be NO remaining bare single quote (which would end the
		// quoting and expose metacharacters).
		stripped := strings.ReplaceAll(q, `'\''`, "")
		inner := stripped[1 : len(stripped)-1]
		if strings.Contains(inner, "'") {
			t.Errorf("shellSingleQuote(%q) = %q leaves a bare quote that breaks quoting", in, q)
		}
	}
}

func TestShellSingleQuoteRoundTripContent(t *testing.T) {
	// The quoted form must contain the original content (escaped), not drop it.
	q := shellSingleQuote("it's a /etc/passwd")
	if !strings.Contains(q, `it'\''s a /etc/passwd`) {
		t.Errorf("unexpected quoting: %q", q)
	}
}

func TestBuildCommandLineQuotesEveryToken(t *testing.T) {
	line := buildCommandLine("getfacl", []string{"-c", "--", "/home/kayla/'; rm -rf /"})
	// The dangerous path must be fully enclosed; no unquoted "rm" token.
	if !strings.HasPrefix(line, `'getfacl' '-c' '--' `) {
		t.Errorf("command line not fully quoted: %q", line)
	}
	// The "--" is quoted but still yields a literal -- after shell parsing.
	if !strings.Contains(line, `'--'`) {
		t.Errorf("missing -- terminator: %q", line)
	}
	// A leading-dash path must be quoted so it isn't read as a flag.
	dash := buildCommandLine("getfacl", []string{"--", "-rf"})
	if !strings.Contains(dash, `'-rf'`) {
		t.Errorf("leading-dash path not quoted: %q", dash)
	}
}
