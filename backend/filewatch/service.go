// Package filewatch implements File Change Detection (Feature 22). Real-time
// inotify requires the agent; absent it, this polls each node's watched paths
// over SSH for files modified since the last scan and records change events.
// It is therefore near-real-time (per scan), not instantaneous — the agent
// upgrade is a follow-on. Events fire the file.change notification trigger.
package filewatch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/KAE-Labs/stratum/backend/db"
)

// ExecFunc runs a command on a node over SSH (fs.Service.Exec).
type ExecFunc func(ctx context.Context, nodeID, cmd string, args ...string) (string, error)

// maxEventsPerScan bounds how many changed files one scan records per watch.
const maxEventsPerScan = 500

// initialWindow is how far back the first scan of a node looks (no prior scan).
const initialWindow = 24 * time.Hour

// Service polls watched paths and records change events.
type Service struct {
	store db.Store
	exec  ExecFunc

	mu       sync.Mutex
	lastScan map[string]time.Time // nodeID -> last scan time
	notify   func(ctx context.Context, trigger, title, text string)
}

// New wires the store + SSH exec.
func New(store db.Store, exec ExecFunc) *Service {
	return &Service{store: store, exec: exec, lastScan: map[string]time.Time{}}
}

// SetNotify wires the webhook notification callback.
func (s *Service) SetNotify(fn func(ctx context.Context, trigger, title, text string)) {
	s.notify = fn
}

// Scan polls a node's watched paths for files modified since the last scan,
// records each as a file event, and fires a notification if any were found.
// Returns the events recorded.
func (s *Service) Scan(ctx context.Context, nodeID string) ([]db.FileEvent, error) {
	watches, err := s.store.ListFileWatchesByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if len(watches) == 0 {
		return nil, nil
	}

	s.mu.Lock()
	since, ok := s.lastScan[nodeID]
	if !ok {
		since = time.Now().Add(-initialWindow)
	}
	s.mu.Unlock()

	now := time.Now()
	var events []db.FileEvent
	for _, w := range watches {
		paths := s.changedPaths(ctx, nodeID, w, since)
		for _, p := range paths {
			ev := db.FileEvent{ID: uuid.NewString(), NodeID: nodeID, Path: p, EventType: "modified", DetectedAt: now}
			if err := s.store.InsertFileEvent(ctx, ev); err == nil {
				events = append(events, ev)
			}
		}
	}

	s.mu.Lock()
	s.lastScan[nodeID] = now
	s.mu.Unlock()

	if len(events) > 0 && s.notify != nil {
		s.notify(ctx, "file.change", "Watched files changed",
			fmt.Sprintf("%d file(s) changed under watched paths on a node (e.g. %s)", len(events), events[0].Path))
	}
	return events, nil
}

// changedPaths runs `find <path> -newermt <since> -type f` over SSH and returns
// the modified file paths (capped). The path comes from admin-configured watch
// rows; the timestamp is our own formatted value — both injection-safe as
// quoted args.
func (s *Service) changedPaths(ctx context.Context, nodeID string, w db.FileWatch, since time.Time) []string {
	depth := ""
	if !w.Recursive {
		depth = " -maxdepth 1"
	}
	// The path is admin-configured but MUST be single-quoted for the shell:
	// inside the sh -c script a bare or %q-quoted path would still allow $(...),
	// backticks, and $VAR expansion. Single quotes expand nothing; an embedded
	// single quote is escaped as '\''. The RFC3339 timestamp is our own value,
	// quoted the same way for consistency.
	script := fmt.Sprintf(`find %s%s -type f -newermt %s 2>/dev/null | head -n %d`,
		shellQuote(w.Path), depth, shellQuote(since.Format(time.RFC3339)), maxEventsPerScan)
	out, err := s.exec(ctx, nodeID, "sh", "-c", script)
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(out, "\n") {
		if p := strings.TrimSpace(line); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// shellQuote wraps s in single quotes for safe use in an sh -c script, escaping
// embedded single quotes as '\''. Nothing inside single quotes is expanded, so
// shell metacharacters in the path are inert.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// AddWatch registers a path to monitor on a node.
func (s *Service) AddWatch(ctx context.Context, nodeID, path string, recursive bool, by string) (db.FileWatch, error) {
	w := db.FileWatch{ID: uuid.NewString(), NodeID: nodeID, Path: path, Recursive: recursive, CreatedBy: by, CreatedAt: time.Now()}
	if err := s.store.CreateFileWatch(ctx, w); err != nil {
		return db.FileWatch{}, err
	}
	return w, nil
}

// ListWatches returns a node's watch config.
func (s *Service) ListWatches(ctx context.Context, nodeID string) ([]db.FileWatch, error) {
	return s.store.ListFileWatchesByNode(ctx, nodeID)
}

// RemoveWatch deletes a watch.
func (s *Service) RemoveWatch(ctx context.Context, id string) error {
	return s.store.DeleteFileWatch(ctx, id)
}

// Events lists recent file events (optionally for one node).
func (s *Service) Events(ctx context.Context, nodeID string, limit int) ([]db.FileEvent, error) {
	return s.store.ListFileEvents(ctx, nodeID, limit)
}
