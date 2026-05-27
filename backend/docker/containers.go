package docker

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// ContainerInfo is one container from ContainerList.
type ContainerInfo struct {
	ID             string // full container ID
	Name           string // primary name, leading "/" stripped
	Image          string // image ref/tag as reported
	ImageID        string // local content-addressable image ID (sha256:...), NOT a repo digest
	State          string // "running" | "exited" | "paused" | "restarting" | "dead" | "created"
	ComposeProject string // label "com.docker.compose.project" (or "")
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
