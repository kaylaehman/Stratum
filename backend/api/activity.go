package api

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/kaylaehman/stratum/backend/activity"
	"github.com/kaylaehman/stratum/backend/db"
)

// parseOptString returns a *string from a query param, nil if empty.
func parseOptString(v string) *string {
	if v == "" {
		return nil
	}
	s := v
	return &s
}

// parseOptTime parses RFC3339 or date-only "2006-01-02"; returns nil on empty
// input. endOfDay matters for an inclusive upper bound: a date-only value is
// midnight, so a `to` filter of "2026-05-27" must cover the whole day, not just
// its first instant — pass endOfDay=true for the `to` bound.
func parseOptTime(v string, endOfDay bool) (*time.Time, bool) {
	if v == "" {
		return nil, true
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t, true
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		return &t, true
	}
	return nil, false
}

type activityEntryView struct {
	ID         string          `json:"id"`
	CreatedAt  string          `json:"created_at"`
	UserID     *string         `json:"user_id"`
	Username   string          `json:"username,omitempty"`
	Action     string          `json:"action"`
	TargetType *string         `json:"target_type"`
	TargetID   *string         `json:"target_id"`
	Detail     json.RawMessage `json:"detail"`
	Result     string          `json:"result"`
}

func toEntryView(e db.ActivityEntry, username string) activityEntryView {
	v := activityEntryView{
		ID:         e.ID,
		CreatedAt:  e.CreatedAt.UTC().Format(time.RFC3339),
		UserID:     e.UserID,
		Username:   username,
		Action:     e.Action,
		TargetType: e.TargetType,
		TargetID:   e.TargetID,
		Result:     e.Result,
	}
	if e.DetailJSON != nil {
		v.Detail = json.RawMessage(*e.DetailJSON)
	}
	return v
}

func buildActivityQuery(r *http.Request) (db.ActivityQuery, string, int, bool) {
	q := r.URL.Query()

	from, fromOK := parseOptTime(q.Get("from"), false)
	if !fromOK {
		return db.ActivityQuery{}, "", 0, false
	}
	to, toOK := parseOptTime(q.Get("to"), true)
	if !toOK {
		return db.ActivityQuery{}, "", 0, false
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	cursor := q.Get("cursor")

	aq := db.ActivityQuery{
		UserID:       parseOptString(q.Get("user")),
		Action:       parseOptString(q.Get("action")),
		ActionPrefix: parseOptString(q.Get("action_prefix")),
		TargetType:   parseOptString(q.Get("target_type")),
		TargetID:     parseOptString(q.Get("target_id")),
		Result:       parseOptString(q.Get("result")),
		Q:            q.Get("q"),
		From:         from,
		To:           to,
	}
	return aq, cursor, limit, true
}

// ActivityList handles GET /api/activity — paginated, filtered activity log.
func (h *Handlers) ActivityList(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	aq, cursor, limit, ok := buildActivityQuery(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_date")
		return
	}

	entries, nextCursor, err := activity.Page(r.Context(), h.Store, aq, cursor, limit)
	if err != nil {
		if errors.Is(err, activity.ErrBadCursor) {
			writeError(w, http.StatusBadRequest, "invalid_cursor")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error")
		return
	}

	usernames := h.resolveUsernamesCtx(r, entries)

	views := make([]activityEntryView, len(entries))
	for i, e := range entries {
		un := ""
		if e.UserID != nil {
			un = usernames[*e.UserID]
		}
		views[i] = toEntryView(e, un)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries":     views,
		"next_cursor": nextCursor,
	})
}

func (h *Handlers) resolveUsernamesCtx(r *http.Request, entries []db.ActivityEntry) map[string]string {
	cache := map[string]string{}
	for _, e := range entries {
		if e.UserID == nil {
			continue
		}
		uid := *e.UserID
		if _, seen := cache[uid]; seen {
			continue
		}
		u, err := h.Store.GetUserByID(r.Context(), uid)
		if err == nil {
			cache[uid] = u.Username
		} else {
			cache[uid] = ""
		}
	}
	return cache
}

// ActivityExportCSV handles GET /api/activity/export.csv — streams filtered log as CSV.
func (h *Handlers) ActivityExportCSV(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	aq, _, _, ok := buildActivityQuery(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_date")
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="activity.csv"`)

	const batchSize = 500
	cursor := ""
	headerWritten := false

	for {
		entries, next, err := activity.Page(r.Context(), h.Store, aq, cursor, batchSize)
		if err != nil {
			if !headerWritten {
				writeError(w, http.StatusInternalServerError, "internal_error")
			}
			return
		}

		if !headerWritten {
			// Write header + first batch together via WriteCSV.
			if err := activity.WriteCSV(w, entries); err != nil {
				return
			}
			headerWritten = true
		} else {
			// Subsequent batches: skip header by writing rows directly.
			if err := writeCSVRows(w, entries); err != nil {
				return
			}
		}

		if next == "" {
			break
		}
		cursor = next
	}
}

// ActivityActions handles GET /api/activity/actions — returns the action taxonomy.
func (h *Handlers) ActivityActions(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"actions": activity.Catalog()})
}

// writeCSVRows writes data rows only (no header) for subsequent streaming batches.
func writeCSVRows(w http.ResponseWriter, entries []db.ActivityEntry) error {
	cw := csv.NewWriter(w)
	for _, e := range entries {
		userID := ""
		if e.UserID != nil {
			userID = *e.UserID
		}
		targetType := ""
		if e.TargetType != nil {
			targetType = *e.TargetType
		}
		targetID := ""
		if e.TargetID != nil {
			targetID = *e.TargetID
		}
		detail := ""
		if e.DetailJSON != nil {
			detail = *e.DetailJSON
		}
		row := []string{
			e.CreatedAt.UTC().Format(time.RFC3339),
			activity.SanitizeCSVField(userID),
			activity.SanitizeCSVField(e.Action),
			activity.SanitizeCSVField(targetType),
			activity.SanitizeCSVField(targetID),
			activity.SanitizeCSVField(e.Result),
			activity.SanitizeCSVField(detail),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
