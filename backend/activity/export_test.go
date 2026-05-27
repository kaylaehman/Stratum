package activity_test

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/activity"
	appdb "github.com/kaylaehman/stratum/backend/db"
)

func strp(s string) *string { return &s }

func TestWriteCSVHeaderAndRows(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	entries := []appdb.ActivityEntry{
		{
			ID:         "e1",
			RowID:      1,
			UserID:     strp("uid-1"),
			Action:     "fs.write",
			TargetType: strp("file"),
			TargetID:   strp("/etc/hosts"),
			DetailJSON: strp(`{"path":"/etc/hosts"}`),
			Result:     "success",
			CreatedAt:  now,
		},
		{
			ID:        "e2",
			RowID:     2,
			Action:    "auth.login",
			Result:    "success",
			CreatedAt: now,
		},
	}

	var buf bytes.Buffer
	if err := activity.WriteCSV(&buf, entries); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	r := csv.NewReader(&buf)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}

	// Header
	wantHeader := []string{"timestamp", "user_id", "action", "target_type", "target_id", "result", "detail"}
	if len(records) < 1 {
		t.Fatal("no records")
	}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Errorf("header[%d] = %q, want %q", i, records[0][i], h)
		}
	}

	// Row 1
	if len(records) < 2 {
		t.Fatal("expected 2 data rows")
	}
	if records[1][1] != "uid-1" {
		t.Errorf("user_id = %q", records[1][1])
	}
	if records[1][2] != "fs.write" {
		t.Errorf("action = %q", records[1][2])
	}
	if records[1][6] != `{"path":"/etc/hosts"}` {
		t.Errorf("detail = %q", records[1][6])
	}

	// Row 2: empty optional fields
	if records[2][1] != "" {
		t.Errorf("user_id should be empty, got %q", records[2][1])
	}
}

func TestSanitizeCSVFieldFormulaInjection(t *testing.T) {
	injections := []string{
		`=cmd|' /C calc'!A0`,
		`@SUM(1+1)`,
		`+cmd`,
		`-1+2`,
	}
	for _, s := range injections {
		got := activity.SanitizeCSVField(s)
		if !strings.HasPrefix(got, "'") {
			t.Errorf("SanitizeCSVField(%q) = %q, want leading single-quote", s, got)
		}
	}
}

func TestSanitizeCSVFieldTabCR(t *testing.T) {
	tab := "\tinjection"
	if got := activity.SanitizeCSVField(tab); !strings.HasPrefix(got, "'") {
		t.Errorf("tab field not sanitized: %q", got)
	}
	cr := "\rinjection"
	if got := activity.SanitizeCSVField(cr); !strings.HasPrefix(got, "'") {
		t.Errorf("CR field not sanitized: %q", got)
	}
}

func TestSanitizeCSVFieldSafeValues(t *testing.T) {
	safe := []string{
		"/etc/hosts",
		"fs.write",
		"success",
		"",
		"node-create",
		"2026-05-27T12:00:00Z",
	}
	for _, s := range safe {
		got := activity.SanitizeCSVField(s)
		if got != s {
			t.Errorf("SanitizeCSVField(%q) = %q, want unchanged", s, got)
		}
	}
}

func TestWriteCSVParsesBack(t *testing.T) {
	// Ensure the written CSV parses back as valid RFC 4180 CSV.
	entries := []appdb.ActivityEntry{
		{
			ID:        "e1",
			Action:    `=DANGEROUS,"quoted"`,
			Result:    "success",
			CreatedAt: time.Now().UTC(),
		},
	}
	var buf bytes.Buffer
	if err := activity.WriteCSV(&buf, entries); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}
	r := csv.NewReader(&buf)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse-back failed: %v", err)
	}
	if len(records) != 2 { // header + 1 data row
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	// The action field should be prefixed with a single quote.
	if !strings.HasPrefix(records[1][2], "'") {
		t.Errorf("dangerous action field not sanitized in output: %q", records[1][2])
	}
}
