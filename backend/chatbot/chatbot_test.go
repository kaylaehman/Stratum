package chatbot

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

type fakeProvider struct {
	nodes      []NodeBrief
	containers []ContainerBrief
}

func (f *fakeProvider) Nodes(context.Context) ([]NodeBrief, error)        { return f.nodes, nil }
func (f *fakeProvider) Containers(context.Context) ([]ContainerBrief, error) { return f.containers, nil }

func newFake() *fakeProvider {
	return &fakeProvider{
		nodes: []NodeBrief{{Name: "pve", Type: "proxmox", Status: "online"}, {Name: "nas", Type: "ssh", Status: "online"}},
		containers: []ContainerBrief{
			{Name: "jellyfin", Image: "jellyfin/jellyfin", Status: "running", NodeName: "nas"},
			{Name: "plex", Image: "plex", Status: "exited", NodeName: "nas"},
		},
	}
}

func TestExtractAIQuestion(t *testing.T) {
	cases := []struct {
		input    string
		wantQ    string
		wantIsAI bool
	}{
		{"hello world", "hello world", true},
		{"  what is docker?  ", "what is docker?", true},
		{"/ask why is nginx failing", "why is nginx failing", true},
		{"/ask@stratumbot explain this", "explain this", true},
		{"/ask", "", false}, // no question
		{"/nodes", "", false},
		{"/help", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		q, isAI := extractAIQuestion(tc.input)
		if isAI != tc.wantIsAI || q != tc.wantQ {
			t.Errorf("extractAIQuestion(%q) = (%q, %v), want (%q, %v)",
				tc.input, q, isAI, tc.wantQ, tc.wantIsAI)
		}
	}
}

func TestSplitMessage(t *testing.T) {
	// Short text is returned as-is.
	chunks := splitMessage("hello")
	if len(chunks) != 1 || chunks[0] != "hello" {
		t.Errorf("short text: %v", chunks)
	}
	// Empty text returns a placeholder.
	if got := splitMessage(""); len(got) != 1 || got[0] == "" {
		t.Errorf("empty text: %v", got)
	}
	// Long text is split into chunks all within telegramMaxLen runes.
	long := strings.Repeat("a", telegramMaxLen+100)
	chunks = splitMessage(long)
	if len(chunks) < 2 {
		t.Errorf("long text should be split, got %d chunks", len(chunks))
	}
	for i, c := range chunks {
		if utf8.RuneCountInString(c) > telegramMaxLen {
			t.Errorf("chunk %d exceeds telegramMaxLen: %d runes", i, utf8.RuneCountInString(c))
		}
	}
	// Verify all content is preserved.
	joined := strings.Join(chunks, "")
	if joined != long {
		t.Errorf("joined chunks differ from original (lengths: %d vs %d)", len(joined), len(long))
	}
}

func TestHandleCommands(t *testing.T) {
	dp := newFake()
	ctx := context.Background()

	if got := Handle(ctx, dp, "/help"); !strings.Contains(got, "/status") {
		t.Errorf("help missing commands: %q", got)
	}
	if got := Handle(ctx, dp, "/nodes"); !strings.Contains(got, "pve") || !strings.Contains(got, "nas") {
		t.Errorf("nodes = %q", got)
	}
	// Summary counts.
	sum := Handle(ctx, dp, "/status")
	if !strings.Contains(sum, "2 host(s)") || !strings.Contains(sum, "running 1") || !strings.Contains(sum, "stopped 1") {
		t.Errorf("summary = %q", sum)
	}
	// Container detail (case-insensitive) + @botname suffix handling.
	det := Handle(ctx, dp, "/status@stratumbot JELLYFIN")
	if !strings.Contains(det, "jellyfin") || !strings.Contains(det, "running") || !strings.Contains(det, "nas") {
		t.Errorf("detail = %q", det)
	}
	if got := Handle(ctx, dp, "/status nope"); !strings.Contains(got, "No container") {
		t.Errorf("missing-container = %q", got)
	}
	// Mutating commands are refused (read-only bot).
	if got := Handle(ctx, dp, "/restart jellyfin"); !strings.Contains(strings.ToLower(got), "aren't available") {
		t.Errorf("restart should be refused: %q", got)
	}
	// Unknown command.
	if got := Handle(ctx, dp, "/frobnicate"); !strings.Contains(got, "Unknown") {
		t.Errorf("unknown = %q", got)
	}
}
