// Package activity is the append-only audit trail. It wraps db.Store's activity
// methods, generating ids and marshaling structured detail, and exposes only
// Append and List — there is intentionally no update or delete. A context-
// carried *Entry lets handlers enrich the audit row the middleware will write.
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kaylaehman/stratum/backend/db"
)

// Result values.
const (
	ResultSuccess = "success"
	ResultError   = "error"
)

// Entry is the data for one audit row prior to persistence. Detail is marshaled
// to JSON when non-nil.
type Entry struct {
	UserID     *string
	Action     string
	TargetType *string
	TargetID   *string
	Detail     any
	Result     string
}

// Store is the append-only activity writer/reader.
type Store struct {
	db db.Store
}

// NewStore wraps a db.Store.
func NewStore(s db.Store) *Store { return &Store{db: s} }

// Append writes one audit row. It generates the id and timestamp; an empty
// Result defaults to success.
func (s *Store) Append(ctx context.Context, e Entry) error {
	if e.Action == "" {
		return fmt.Errorf("activity: action is required")
	}
	result := e.Result
	if result == "" {
		result = ResultSuccess
	}
	var detail *string
	if e.Detail != nil {
		b, err := json.Marshal(e.Detail)
		if err != nil {
			return fmt.Errorf("activity: marshal detail: %w", err)
		}
		ds := string(b)
		detail = &ds
	}
	return s.db.AppendActivity(ctx, db.ActivityEntry{
		ID:         uuid.NewString(),
		UserID:     e.UserID,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		DetailJSON: detail,
		Result:     result,
		CreatedAt:  time.Now(),
	})
}

// List returns recent audit rows matching the filter, newest first.
func (s *Store) List(ctx context.Context, f db.ActivityFilter) ([]db.ActivityEntry, error) {
	return s.db.ListActivity(ctx, f)
}

// --- context plumbing for middleware enrichment ---

type ctxKey struct{}

// NewContext seeds an empty *Entry into ctx for a request. The activity
// middleware calls this; handlers call FromContext to enrich it.
func NewContext(ctx context.Context, e *Entry) context.Context {
	return context.WithValue(ctx, ctxKey{}, e)
}

// FromContext returns the request's *Entry, or nil if none was seeded.
func FromContext(ctx context.Context) *Entry {
	e, _ := ctx.Value(ctxKey{}).(*Entry)
	return e
}
