package chatbot

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// NodeBrief / ContainerBrief are the minimal shapes the command handler renders.
type NodeBrief struct {
	Name   string
	Type   string
	Status string
}

type ContainerBrief struct {
	Name     string
	Image    string
	Status   string
	NodeName string
}

// DataProvider supplies inventory to the read-only command handler.
type DataProvider interface {
	Nodes(ctx context.Context) ([]NodeBrief, error)
	Containers(ctx context.Context) ([]ContainerBrief, error)
}

// Handle parses and executes a read-only command, returning the reply text.
// Unknown or mutating commands return a helpful message (no state change).
func Handle(ctx context.Context, dp DataProvider, text string) string {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return helpText()
	}
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	// Strip a possible @botname suffix (Telegram group mentions).
	if i := strings.Index(cmd, "@"); i >= 0 {
		cmd = cmd[:i]
	}
	args := fields[1:]

	switch cmd {
	case "help", "start":
		return helpText()
	case "nodes":
		return renderNodes(ctx, dp)
	case "status":
		if len(args) == 0 {
			return renderSummary(ctx, dp)
		}
		return renderContainer(ctx, dp, args[0])
	case "restart", "stop", "start_container", "delete", "rm":
		return "Mutating commands aren't available from chat yet — use the Stratum UI (they require a 2FA-style confirmation)."
	default:
		return "Unknown command. " + helpText()
	}
}

func helpText() string {
	return "Stratum bot — read-only commands:\n" +
		"/status — summary of nodes and containers\n" +
		"/status <name> — detail for a container\n" +
		"/nodes — list connected hosts\n" +
		"/help — this message"
}

func renderNodes(ctx context.Context, dp DataProvider) string {
	nodes, err := dp.Nodes(ctx)
	if err != nil {
		return "Couldn't read nodes."
	}
	if len(nodes) == 0 {
		return "No nodes connected."
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Connected hosts (%d):\n", len(nodes)))
	for _, n := range nodes {
		b.WriteString(fmt.Sprintf("• %s [%s] — %s\n", n.Name, n.Type, n.Status))
	}
	return strings.TrimRight(b.String(), "\n")
}

func renderSummary(ctx context.Context, dp DataProvider) string {
	nodes, _ := dp.Nodes(ctx)
	containers, err := dp.Containers(ctx)
	if err != nil {
		return "Couldn't read inventory."
	}
	running, stopped, other := 0, 0, 0
	for _, c := range containers {
		switch c.Status {
		case "running":
			running++
		case "exited", "stopped":
			stopped++
		default:
			other++
		}
	}
	return fmt.Sprintf("Stratum: %d host(s), %d container(s)\nrunning %d · stopped %d · other %d",
		len(nodes), len(containers), running, stopped, other)
}

func renderContainer(ctx context.Context, dp DataProvider, name string) string {
	containers, err := dp.Containers(ctx)
	if err != nil {
		return "Couldn't read containers."
	}
	q := strings.ToLower(name)
	for _, c := range containers {
		if strings.ToLower(c.Name) == q {
			return fmt.Sprintf("%s\nstatus: %s\nimage: %s\nhost: %s", c.Name, c.Status, c.Image, c.NodeName)
		}
	}
	return fmt.Sprintf("No container named %q found.", name)
}
