package agent

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kaylaehman/stratum/backend/db"
	stratumv1 "github.com/kaylaehman/stratum/proto/gen/stratum/v1"
)

// stubStore captures InsertFileEvent calls for assertion.
type stubStore struct {
	db.Store // embed interface; all methods except InsertFileEvent panic if called
	inserted []db.FileEvent
}

func (s *stubStore) InsertFileEvent(_ context.Context, e db.FileEvent) error {
	s.inserted = append(s.inserted, e)
	return nil
}

func TestPersistEvents(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	events := []*stratumv1.WatchFilesResponse{
		{
			Path:      "/etc/passwd",
			EventType: stratumv1.FileEventType_FILE_EVENT_TYPE_MODIFY,
			Timestamp: timestamppb.New(now),
		},
		{
			Path:      "/root/.ssh/authorized_keys",
			EventType: stratumv1.FileEventType_FILE_EVENT_TYPE_CREATE,
			Timestamp: timestamppb.New(now),
		},
	}

	ch := make(chan *stratumv1.WatchFilesResponse, len(events))
	for _, ev := range events {
		ch <- ev
	}
	close(ch)

	store := &stubStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	PersistEvents(context.Background(), "node-1", ch, store, logger)

	if len(store.inserted) != len(events) {
		t.Fatalf("got %d inserted events, want %d", len(store.inserted), len(events))
	}

	for i, ev := range events {
		got := store.inserted[i]
		if got.NodeID != "node-1" {
			t.Errorf("row %d NodeID = %q, want %q", i, got.NodeID, "node-1")
		}
		if got.Path != ev.Path {
			t.Errorf("row %d Path = %q, want %q", i, got.Path, ev.Path)
		}
		if got.ID == "" {
			t.Errorf("row %d: ID must not be empty", i)
		}
		wantType := protoEventTypeToDB(ev.EventType)
		if got.EventType != wantType {
			t.Errorf("row %d EventType = %q, want %q", i, got.EventType, wantType)
		}
	}
}
