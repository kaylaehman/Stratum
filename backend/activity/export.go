package activity

import (
	"encoding/csv"
	"io"
	"time"
	"unicode/utf8"

	"github.com/kaylaehman/stratum/backend/db"
)

var csvInjectionPrefixes = []rune{'=', '+', '-', '@', '\t', '\r'}

// SanitizeCSVField prevents formula injection by prefixing dangerous first runes
// with a single quote so spreadsheets treat the cell as literal text.
// Exported so the api package can reuse it for streaming row batches.
func SanitizeCSVField(s string) string {
	if s == "" {
		return s
	}
	r, _ := utf8.DecodeRuneInString(s)
	for _, dangerous := range csvInjectionPrefixes {
		if r == dangerous {
			return "'" + s
		}
	}
	return s
}

// WriteCSV writes entries as CSV to w. The header row is always written first.
// Fields are sanitized against spreadsheet formula injection.
func WriteCSV(w io.Writer, entries []db.ActivityEntry) error {
	cw := csv.NewWriter(w)

	header := []string{"timestamp", "user_id", "action", "target_type", "target_id", "result", "detail"}
	if err := cw.Write(header); err != nil {
		return err
	}

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
			SanitizeCSVField(userID),
			SanitizeCSVField(e.Action),
			SanitizeCSVField(targetType),
			SanitizeCSVField(targetID),
			SanitizeCSVField(e.Result),
			SanitizeCSVField(detail),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}
