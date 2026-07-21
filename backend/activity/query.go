package activity

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"context"

	"github.com/KAE-Labs/stratum/backend/db"
)

// ErrBadCursor is returned when a cursor string cannot be decoded.
var ErrBadCursor = errors.New("activity: invalid cursor")

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

// EncodeCursor encodes a rowid as an opaque base64url cursor string.
func EncodeCursor(rowID int64) string {
	return base64.URLEncoding.EncodeToString([]byte(strconv.FormatInt(rowID, 10)))
}

// DecodeCursor decodes an opaque cursor string back to a rowid.
func DecodeCursor(s string) (int64, error) {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrBadCursor, err)
	}
	id, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrBadCursor, err)
	}
	return id, nil
}

// Page fetches one page of activity entries using keyset pagination.
// cursor is the opaque next_cursor from a prior response; empty means first page.
// limit <=0 defaults to 50; capped at 200.
// Returns the entries, the next cursor (empty string if no more pages), and any error.
func Page(ctx context.Context, store db.Store, f db.ActivityQuery, cursor string, limit int) ([]db.ActivityEntry, string, error) {
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	if cursor != "" {
		rowID, err := DecodeCursor(cursor)
		if err != nil {
			return nil, "", ErrBadCursor
		}
		f.CursorRowID = &rowID
	}

	f.Limit = limit + 1

	rows, err := store.QueryActivityLog(ctx, f)
	if err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(rows) > limit {
		nextCursor = EncodeCursor(rows[limit-1].RowID)
		rows = rows[:limit]
	}

	return rows, nextCursor, nil
}
