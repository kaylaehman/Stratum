package docker

import (
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// ContainerLogs returns a streaming reader of a container's logs. With
// follow=true it tails live; tail is the backlog count ("100", "all").
// Timestamps are included (RFC3339Nano prefix per line). The stream is
// stdcopy-multiplexed unless the container has a TTY (check Inspect().Tty).
func (c *Client) ContainerLogs(ctx context.Context, id, tail string, follow bool) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
		Tail:       tail,
	})
}

// ContainerInfo is one container from ContainerList.
type ContainerInfo struct {
	ID             string // full container ID
	Name           string // primary name, leading "/" stripped
	Image          string // image ref/tag as reported
	ImageID        string // local content-addressable image ID (sha256:...), NOT a repo digest
	State          string // "running" | "exited" | "paused" | "restarting" | "dead" | "created"
	ComposeProject string // label "com.docker.compose.project" (or "")
	ComposeService string // label "com.docker.compose.service" (or "")

	// HealthStatus is "healthy" | "unhealthy" | "starting" | "" (none/unknown).
	// Parsed from the human-readable Status field in the Docker list summary.
	HealthStatus string
}

// ContainerEvent is a lifecycle event for one container.
type ContainerEvent struct {
	ContainerID string
	Action      string // "start" | "die" | "create" | "destroy" | "health_status" (raw base action)
}

// mapSummary converts a container.Summary into a ContainerInfo.
// It strips the leading "/" from the first name and extracts the compose
// project label. Guards against an empty Names slice.
func mapSummary(s container.Summary) ContainerInfo {
	name := ""
	if len(s.Names) > 0 {
		name = strings.TrimPrefix(s.Names[0], "/")
	}
	return ContainerInfo{
		ID:             s.ID,
		Name:           name,
		Image:          s.Image,
		ImageID:        s.ImageID,
		State:          s.State,
		ComposeProject: s.Labels["com.docker.compose.project"],
		ComposeService: s.Labels["com.docker.compose.service"],
		HealthStatus:   parseHealthStatus(s.Status),
	}
}

// parseHealthStatus extracts the health status from the Docker container
// summary Status string (e.g. "Up 2 hours (unhealthy)" → "unhealthy").
// Returns "" when no health information is present.
func parseHealthStatus(status string) string {
	switch {
	case strings.Contains(status, "(unhealthy)"):
		return "unhealthy"
	case strings.Contains(status, "(healthy)"):
		return "healthy"
	case strings.Contains(status, "(health: starting)"):
		return "starting"
	default:
		return ""
	}
}

// mapEventAction extracts the base action string from a raw Docker event
// action, stripping any ":suffix" (e.g. "health_status: healthy" → "health_status").
func mapEventAction(raw string) string {
	action := string(raw)
	if idx := strings.Index(action, ":"); idx >= 0 {
		action = strings.TrimSpace(action[:idx])
	}
	return action
}

// ContainerList returns all containers (running and stopped).
func (c *Client) ContainerList(ctx context.Context) ([]ContainerInfo, error) {
	summaries, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}
	result := make([]ContainerInfo, len(summaries))
	for i, s := range summaries {
		result[i] = mapSummary(s)
	}
	return result, nil
}

// Events streams container lifecycle events until ctx is cancelled.
// It returns a receive channel of events and a receive channel of errors.
// When ctx is cancelled or the stream ends, both channels are closed.
func (c *Client) Events(ctx context.Context) (<-chan ContainerEvent, <-chan error) {
	evCh := make(chan ContainerEvent)
	errCh := make(chan error, 1)

	f := filters.NewArgs(filters.Arg("type", string(events.ContainerEventType)))
	msgCh, sdkErrCh := c.cli.Events(ctx, events.ListOptions{Filters: f})

	go func() {
		defer close(evCh)
		defer close(errCh)
		for {
			select {
			case msg, ok := <-msgCh:
				if !ok {
					return
				}
				evCh <- ContainerEvent{
					ContainerID: msg.Actor.ID,
					Action:      mapEventAction(string(msg.Action)),
				}
			case err, ok := <-sdkErrCh:
				if !ok {
					return
				}
				errCh <- err
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return evCh, errCh
}
