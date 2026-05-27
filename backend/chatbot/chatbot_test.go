package chatbot

import (
	"context"
	"strings"
	"testing"
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
