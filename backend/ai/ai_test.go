package ai

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc lets a test stand in for the network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestOllamaAsk(t *testing.T) {
	var gotPath, gotBody string
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		return jsonResp(200, `{"message":{"role":"assistant","content":"hello there"},"prompt_eval_count":12,"eval_count":5}`), nil
	})}
	o := NewOllama("http://ollama.local:11434/", "llama3", hc)

	resp, err := o.Ask(context.Background(), AskRequest{System: "be brief", Prompt: "hi"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if gotPath != "/api/chat" {
		t.Errorf("path = %q, want /api/chat", gotPath)
	}
	if !strings.Contains(gotBody, `"stream":false`) || !strings.Contains(gotBody, `"model":"llama3"`) {
		t.Errorf("body missing fields: %s", gotBody)
	}
	if resp.Answer != "hello there" || resp.OutputTokens != 5 {
		t.Errorf("resp = %+v", resp)
	}
}

func TestClaudeAskSendsHeadersAndParses(t *testing.T) {
	var hdrKey, hdrVer string
	hc := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		hdrKey = r.Header.Get("x-api-key")
		hdrVer = r.Header.Get("anthropic-version")
		return jsonResp(200, `{"content":[{"type":"text","text":"answer"}],"usage":{"input_tokens":7,"output_tokens":3}}`), nil
	})}
	c := NewClaude("sk-test", "", hc)

	resp, err := c.Ask(context.Background(), AskRequest{Prompt: "why?"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if hdrKey != "sk-test" || hdrVer != claudeAPIVersion {
		t.Errorf("headers: key=%q ver=%q", hdrKey, hdrVer)
	}
	if resp.Answer != "answer" || resp.InputTokens != 7 {
		t.Errorf("resp = %+v", resp)
	}
	if c.model != DefaultClaudeModel {
		t.Errorf("model = %q, want default", c.model)
	}
}

func TestClaudeAskErrorDoesNotEchoRequest(t *testing.T) {
	hc := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResp(401, `{"error":{"type":"authentication_error","message":"invalid x-api-key"}}`), nil
	})}
	c := NewClaude("sk-secret-value", "", hc)
	_, err := c.Ask(context.Background(), AskRequest{Prompt: "hi"})
	if err == nil {
		t.Fatal("want error on 401")
	}
	if strings.Contains(err.Error(), "sk-secret-value") {
		t.Errorf("error leaked the api key: %v", err)
	}
}

func TestTruncateContext(t *testing.T) {
	if got := truncateContext(""); got != "" {
		t.Errorf("empty = %q", got)
	}
	// More than maxContextLines lines -> tail kept + marker.
	var lines []string
	for i := 0; i < maxContextLines+50; i++ {
		lines = append(lines, "line")
	}
	out := truncateContext(strings.Join(lines, "\n"))
	if !strings.HasPrefix(out, "[earlier context truncated]") {
		t.Errorf("expected truncation marker, got prefix %q", out[:30])
	}
	if n := strings.Count(out, "line"); n != maxContextLines {
		t.Errorf("kept %d lines, want %d", n, maxContextLines)
	}
}

func TestValidateHTTPURL(t *testing.T) {
	for _, ok := range []string{"http://localhost:11434", "https://ollama.lan"} {
		if err := validateHTTPURL(ok); err != nil {
			t.Errorf("validateHTTPURL(%q) = %v, want nil", ok, err)
		}
	}
	// http(s) host-only required; path/query/fragment smuggling rejected.
	for _, bad := range []string{
		"", "ftp://x", "file:///etc/passwd", "notaurl", "://nohost",
		"http://h/injected", "http://h/?q=", "http://h#frag", "http://h/api?x=1",
	} {
		if err := validateHTTPURL(bad); err == nil {
			t.Errorf("validateHTTPURL(%q) = nil, want error", bad)
		}
	}
	// A trailing slash is allowed (treated as host-only).
	if err := validateHTTPURL("http://localhost:11434/"); err != nil {
		t.Errorf("validateHTTPURL with trailing slash = %v, want nil", err)
	}
}
