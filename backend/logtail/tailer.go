package logtail

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/pkg/stdcopy"
	"github.com/KAE-Labs/stratum/backend/docker"
)

// tailClient is the subset of docker.Client used by tailer (allows stub in tests).
type tailClient interface {
	Inspect(ctx context.Context, id string) (docker.InspectInfo, error)
	ContainerLogs(ctx context.Context, id, tail string, follow bool) (io.ReadCloser, error)
}

const (
	backoffBase    = 500 * time.Millisecond
	backoffCeiling = 32 * time.Second
	maxRetries     = 10
)

// tailer follows a container's log stream, demultiplexes it, and calls publish
// for every parsed LogLine. It restarts automatically on error until ctx is
// cancelled or maxRetries is exhausted.
type tailer struct {
	client      tailClient
	containerID string
	publish     func(LogLine)
}

// run blocks until ctx is cancelled or the retry budget is exhausted.
// initialTail is the backlog depth passed on the first start ("100", "all", …).
func (t *tailer) run(ctx context.Context, initialTail string) {
	tail := initialTail
	backoff := backoffBase
	attempts := 0

	for {
		if ctx.Err() != nil {
			return
		}

		err := t.stream(ctx, tail)
		if ctx.Err() != nil {
			return
		}

		if err != nil {
			t.publish(LogLine{
				ContainerID: t.containerID,
				Stream:      "stdout",
				Text:        "— tailer error —",
			})
		} else {
			// Clean EOF from the stream.
			t.publish(LogLine{
				ContainerID: t.containerID,
				Stream:      "stdout",
				Text:        "— stream ended —",
			})
		}

		attempts++
		if attempts >= maxRetries {
			t.publish(LogLine{
				ContainerID: t.containerID,
				Stream:      "stdout",
				Text:        "— tailer gave up —",
			})
			return
		}

		// On restart use tail="0" to avoid replaying the entire backlog.
		// NOTE: because ContainerLogs here takes only tail+follow (no Since),
		// there is a small gap/overlap window on reconnect; dedup is out of scope.
		tail = "0"

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > backoffCeiling {
			backoff = backoffCeiling
		}
	}
}

// stream opens one log session and reads until EOF or ctx cancellation.
// Returns nil on clean EOF, non-nil on read error.
func (t *tailer) stream(ctx context.Context, tail string) error {
	// Re-inspect on every (re)start to get a fresh Tty value.
	info, err := t.client.Inspect(ctx, t.containerID)
	if err != nil {
		return err
	}

	rc, err := t.client.ContainerLogs(ctx, t.containerID, tail, true)
	if err != nil {
		return err
	}

	// Run the read in a separate goroutine so we can close rc when ctx fires.
	type result struct{ err error }
	done := make(chan result, 1)

	go func() {
		done <- result{err: t.readAll(rc, info.Tty)}
	}()

	select {
	case <-ctx.Done():
		_ = rc.Close() // unblock stdcopy / raw read
		<-done         // wait for goroutine
		return nil     // treat ctx cancel as clean exit (caller checks ctx.Err)
	case res := <-done:
		_ = rc.Close()
		return res.err
	}
}

// readAll drains rc, calling publish for every line.
// If tty is false the stream is stdcopy-demultiplexed; otherwise raw.
func (t *tailer) readAll(rc io.Reader, tty bool) error {
	if tty {
		w := newLineWriter(t.containerID, "stdout", t.publish)
		_, err := io.Copy(w, rc)
		w.flush()
		if err == io.EOF {
			return nil
		}
		return err
	}

	// Non-TTY: use stdcopy to demux stdout and stderr.
	stdoutW := newLineWriter(t.containerID, "stdout", t.publish)
	stderrW := newLineWriter(t.containerID, "stderr", t.publish)
	_, err := stdcopy.StdCopy(stdoutW, stderrW, rc)
	stdoutW.flush()
	stderrW.flush()
	if err == io.EOF {
		return nil
	}
	return err
}
