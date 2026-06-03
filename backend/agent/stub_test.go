package agent

import (
	"context"

	"github.com/kaylaehman/stratum/backend/db"
)

// stubStore implements db.Store for unit tests.  Only the methods exercised by
// agent package tests are implemented; all others are provided by the embedded
// interface (any call to an unimplemented method will panic with a nil pointer
// dereference, which is intentional — it means the test is calling something
// it shouldn't).
type stubStore struct {
	db.Store // embed for all unimplemented methods

	inserted   []db.FileEvent
	getNodeFn  func(ctx context.Context, id string) (db.Node, error)
}

func (s *stubStore) InsertFileEvent(_ context.Context, e db.FileEvent) error {
	s.inserted = append(s.inserted, e)
	return nil
}

func (s *stubStore) GetNode(ctx context.Context, id string) (db.Node, error) {
	if s.getNodeFn != nil {
		return s.getNodeFn(ctx, id)
	}
	return db.Node{}, db.ErrNotFound
}
