// Package activity is the append-only audit trail. It wraps db.Store's activity
// methods, generating ids and marshaling structured detail, and exposes only
// Append and List — there is intentionally no update or delete. A context-
// carried *Entry lets handlers enrich the audit row the middleware will write.
package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

	// Suppressed, when set by a handler, tells the activity middleware NOT to
	// write its deferred row. Streaming handlers (e.g. file upload) set this and
	// call Append directly at the point the outcome is known, avoiding a double
	// entry (foundation design §5.4).
	Suppressed bool
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
		// Defense-in-depth secret scrub. The table is append-only AND undeletable,
		// so a single emitter that forgets to redact would be a PERMANENT,
		// CSV-exportable leak. Re-walk the marshaled detail and redact the values
		// of any sensitive-looking key. This is a backstop, not a substitute for
		// redaction at the emitter. Fail SAFE: if the detail can't be parsed or
		// re-marshaled, write a sentinel rather than the unscrubbed bytes — a
		// permanent leak is worse than a lost detail payload.
		var generic any
		if err := json.Unmarshal(b, &generic); err != nil {
			b = []byte(`{"redacted":"detail_unparseable"}`)
		} else if sb, err := json.Marshal(scrubSecrets(generic)); err != nil {
			b = []byte(`{"redacted":"detail_unscrubbable"}`)
		} else {
			b = sb
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

const redacted = "[REDACTED]"

// sensitiveKeySubstrings: a JSON object key containing any of these (case-
// insensitive), or ending in "_key", has its value redacted. Cert/CA fields are
// public and intentionally not matched.
var sensitiveKeySubstrings = []string{
	"password", "passwd", "secret", "token", "passphrase",
	"private_key", "credential", "encryption_key", "jwt",
	"bearer", "authorization", "apikey",
}

func isSensitiveKey(k string) bool {
	lk := strings.ToLower(k)
	if strings.HasSuffix(lk, "_key") || lk == "key" {
		return true
	}
	for _, sub := range sensitiveKeySubstrings {
		if strings.Contains(lk, sub) {
			return true
		}
	}
	return false
}

// scrubSecrets recursively replaces the values of sensitive-looking object keys
// with a redaction marker, leaving structure and non-secret values intact.
func scrubSecrets(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if isSensitiveKey(k) {
				if val != nil {
					t[k] = redacted
				}
				continue
			}
			t[k] = scrubSecrets(val)
		}
		return t
	case []any:
		for i, item := range t {
			t[i] = scrubSecrets(item)
		}
		return t
	default:
		return v
	}
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
