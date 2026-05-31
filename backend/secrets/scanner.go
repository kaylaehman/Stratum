package secrets

import (
	"bufio"
	"math"
	"strings"
)

// ScanFinding is one detected plaintext-secret candidate. The value is NEVER
// included — only path, line number, key name, and the detection reason are
// returned. This is a read-only findings type; it never writes to any store.
type ScanFinding struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	KeyName string `json:"key_name"`
	Reason  string `json:"reason"`
}

// sensitiveKeyPatterns are substrings that, when found in an env key name
// (case-insensitive), suggest the value may be a secret.
var sensitiveKeyPatterns = []string{
	"password", "passwd", "secret", "api_key", "apikey",
	"token", "private_key", "privatekey", "access_key", "accesskey",
	"auth_key", "authkey", "credentials", "credential",
	"db_pass", "database_pass",
}

// pemHeaders are the start-of-line markers that identify a PEM-encoded private
// key or certificate in any text file.
var pemHeaders = []string{
	"-----BEGIN RSA PRIVATE KEY-----",
	"-----BEGIN EC PRIVATE KEY-----",
	"-----BEGIN OPENSSH PRIVATE KEY-----",
	"-----BEGIN PRIVATE KEY-----",
	"-----BEGIN ENCRYPTED PRIVATE KEY-----",
	"-----BEGIN DSA PRIVATE KEY-----",
}

// minEntropyBits is the Shannon-entropy threshold (bits per character) above
// which a value triggers the high-entropy rule. Values below this are benign
// placeholders (e.g. "changeme", "localhost").
const minEntropyBits = 3.5

// ScanText scans the content of a single env/compose file and returns findings.
// path is used only for annotation — the text is never stored or logged.
// The VALUE of any key is never included in any finding.
func ScanText(path, content string) []ScanFinding {
	var findings []ScanFinding
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip blank lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Detect PEM private key headers in any file.
		for _, hdr := range pemHeaders {
			if strings.Contains(trimmed, hdr) {
				findings = append(findings, ScanFinding{
					Path:    path,
					Line:    lineNum,
					KeyName: "(pem block)",
					Reason:  "PEM private key header detected in file",
				})
				break
			}
		}

		// Parse KEY=VALUE lines. Strip "export " prefix.
		kv := strings.TrimPrefix(trimmed, "export ")
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		key := strings.TrimSpace(kv[:eq])
		val := strings.TrimSpace(kv[eq+1:])
		val = unquote(val)

		if key == "" || val == "" {
			continue
		}

		keyLower := strings.ToLower(key)

		// Rule 1: key name matches a sensitive pattern.
		for _, pat := range sensitiveKeyPatterns {
			if strings.Contains(keyLower, pat) {
				findings = append(findings, ScanFinding{
					Path:    path,
					Line:    lineNum,
					KeyName: key,
					Reason:  "key name matches sensitive pattern (" + pat + ")",
				})
				break
			}
		}

		// Rule 2: value has high Shannon entropy (potential token/secret).
		if entropy(val) >= minEntropyBits && len(val) >= 16 {
			// Only flag if the key name doesn't look like a path/URL/hostname
			// (avoid false positives on e.g. BASE_URL=https://...).
			if !isPathOrURL(val) {
				findings = append(findings, ScanFinding{
					Path:    path,
					Line:    lineNum,
					KeyName: key,
					Reason:  "value has high entropy, may be a secret token",
				})
			}
		}
	}
	return findings
}

// entropy computes the Shannon entropy (bits per character) of s.
func entropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int, 64)
	for _, c := range s {
		freq[c]++
	}
	n := float64(len([]rune(s)))
	var e float64
	for _, count := range freq {
		p := float64(count) / n
		e -= p * math.Log2(p)
	}
	return e
}

// isPathOrURL returns true when val starts with a scheme, slash, or tilde —
// heuristic to avoid flagging filesystem paths and URLs as high-entropy secrets.
func isPathOrURL(val string) bool {
	if len(val) == 0 {
		return false
	}
	for _, pfx := range []string{"http://", "https://", "ftp://", "ssh://", "/", "~/", "./", "file://"} {
		if strings.HasPrefix(val, pfx) {
			return true
		}
	}
	return false
}
