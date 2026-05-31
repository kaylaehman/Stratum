// Package watch wraps github.com/fsnotify/fsnotify to provide inotify-based
// real-time file change detection. It translates fsnotify events into proto
// WatchFilesResponse messages and delivers them on a channel. Recursive
// watching is achieved by adding new subdirectories as they are created.
package watch

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/protobuf/types/known/timestamppb"

	stratumv1 "github.com/kaylaehman/stratum/proto/gen/stratum/v1"
)

// Watcher watches one or more paths and emits WatchFilesResponse events.
type Watcher struct {
	fsw     *fsnotify.Watcher
	logger  *slog.Logger
	events  chan *stratumv1.WatchFilesResponse
}

// New creates a Watcher backed by the OS file-watch facility. It is the
// caller's responsibility to call Close when done.
func New(logger *slog.Logger) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fsw:    fsw,
		logger: logger,
		events: make(chan *stratumv1.WatchFilesResponse, 256),
	}, nil
}

// Add registers path (and, when recursive=true, all existing subdirectories)
// with the OS watcher.
func (w *Watcher) Add(path string, recursive bool) error {
	if err := w.fsw.Add(path); err != nil {
		return err
	}
	if !recursive {
		return nil
	}
	return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || p == path {
			return nil
		}
		if werr := w.fsw.Add(p); werr != nil {
			w.logger.Warn("watch: add subdir", "path", p, "error", werr)
		}
		return nil
	})
}

// Events returns the channel on which translated events are delivered.
func (w *Watcher) Events() <-chan *stratumv1.WatchFilesResponse {
	return w.events
}

// Run pumps the underlying fsnotify channels, translates events, and auto-adds
// new subdirectories when recursive is true. It blocks until the watcher is
// closed or an unrecoverable error occurs.
func (w *Watcher) Run(recursive bool) {
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(ev, recursive)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.logger.Warn("watch: fsnotify error", "error", err)
		}
	}
}

// Close shuts down the underlying watcher and drains the events channel.
func (w *Watcher) Close() error {
	return w.fsw.Close()
}

func (w *Watcher) handleEvent(ev fsnotify.Event, recursive bool) {
	et := translateOp(ev.Op)
	if et == stratumv1.FileEventType_FILE_EVENT_TYPE_UNSPECIFIED {
		return
	}

	info, statErr := os.Lstat(ev.Name)
	isDir := statErr == nil && info.IsDir()

	// Auto-add newly created subdirectories when recursive watching is on.
	if recursive && isDir && ev.Op&fsnotify.Create != 0 {
		if err := w.fsw.Add(ev.Name); err != nil {
			w.logger.Warn("watch: auto-add new dir", "path", ev.Name, "error", err)
		}
	}

	msg := &stratumv1.WatchFilesResponse{
		Path:      ev.Name,
		EventType: et,
		Timestamp: timestamppb.New(time.Now().UTC()),
		IsDir:     isDir,
	}
	select {
	case w.events <- msg:
	default:
		w.logger.Warn("watch: event channel full; dropping event", "path", ev.Name)
	}
}

// translateOp maps an fsnotify Op bitmask to our proto enum. Fsnotify can
// set multiple bits; we pick the most semantically significant one.
func translateOp(op fsnotify.Op) stratumv1.FileEventType {
	switch {
	case op&fsnotify.Create != 0:
		return stratumv1.FileEventType_FILE_EVENT_TYPE_CREATE
	case op&fsnotify.Write != 0:
		return stratumv1.FileEventType_FILE_EVENT_TYPE_MODIFY
	case op&fsnotify.Remove != 0:
		return stratumv1.FileEventType_FILE_EVENT_TYPE_DELETE
	case op&fsnotify.Rename != 0:
		return stratumv1.FileEventType_FILE_EVENT_TYPE_RENAME
	case op&fsnotify.Chmod != 0:
		return stratumv1.FileEventType_FILE_EVENT_TYPE_ATTRIB
	default:
		return stratumv1.FileEventType_FILE_EVENT_TYPE_UNSPECIFIED
	}
}
