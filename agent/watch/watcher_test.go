package watch

import (
	"io"
	"log/slog"
	"testing"

	"github.com/fsnotify/fsnotify"

	stratumv1 "github.com/KAE-Labs/stratum/proto/gen/stratum/v1"
)

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestTranslateOp(t *testing.T) {
	cases := []struct {
		op   fsnotify.Op
		want stratumv1.FileEventType
	}{
		{fsnotify.Create, stratumv1.FileEventType_FILE_EVENT_TYPE_CREATE},
		{fsnotify.Write, stratumv1.FileEventType_FILE_EVENT_TYPE_MODIFY},
		{fsnotify.Remove, stratumv1.FileEventType_FILE_EVENT_TYPE_DELETE},
		{fsnotify.Rename, stratumv1.FileEventType_FILE_EVENT_TYPE_RENAME},
		{fsnotify.Chmod, stratumv1.FileEventType_FILE_EVENT_TYPE_ATTRIB},
		// Combined ops: Create wins.
		{fsnotify.Create | fsnotify.Write, stratumv1.FileEventType_FILE_EVENT_TYPE_CREATE},
		// Unrecognised op.
		{0, stratumv1.FileEventType_FILE_EVENT_TYPE_UNSPECIFIED},
	}

	for _, tc := range cases {
		got := translateOp(tc.op)
		if got != tc.want {
			t.Errorf("translateOp(%v) = %v, want %v", tc.op, got, tc.want)
		}
	}
}

// TestHandleEventDropsFull ensures handleEvent does not block when the events
// channel is at capacity.
func TestHandleEventDropsFull(t *testing.T) {
	w := &Watcher{
		events: make(chan *stratumv1.WatchFilesResponse), // unbuffered: always full when nothing reads
		logger: noopLogger(),
	}
	// Must not block or panic.
	w.handleEvent(fsnotify.Event{Name: "/tmp/x", Op: fsnotify.Write}, false)
}
