package uptime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kaylaehman/stratum/backend/db"
)

// --- looksLikeStatusCode ---

func TestLooksLikeStatusCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"200", true},
		{"404", true},
		{"500", true},
		{"20", false},
		{"2000", false},
		{"abc", false},
		{"20a", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := looksLikeStatusCode(tc.in); got != tc.want {
			t.Errorf("looksLikeStatusCode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// --- classifyHTTP ---

func TestClassifyHTTP(t *testing.T) {
	cases := []struct {
		code, elapsed, timeout int
		want                   CheckStatus
	}{
		{200, 100, 5000, StatusUp},
		{201, 4500, 5000, StatusDegraded}, // > 80 % of timeout
		{301, 100, 5000, StatusUp},
		{400, 100, 5000, StatusDown},
		{500, 100, 5000, StatusDown},
	}
	for _, tc := range cases {
		got := classifyHTTP(tc.code, tc.elapsed, tc.timeout)
		if got != tc.want {
			t.Errorf("classifyHTTP(%d, %d, %d) = %s, want %s", tc.code, tc.elapsed, tc.timeout, got, tc.want)
		}
	}
}

// --- HTTP checker integration (uses httptest, no real network) ---

func TestNetCheckerHTTP_Up(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "hello world")
	}))
	defer srv.Close()

	c := &NetChecker{}
	m := Monitor{
		ID:        "m1",
		Type:      CheckHTTP,
		Target:    srv.URL,
		TimeoutMs: 5000,
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusUp {
		t.Errorf("expected up, got %s (error: %s)", result.Status, result.Error)
	}
	if result.ResponseTimeMs < 0 {
		t.Errorf("negative response time: %d", result.ResponseTimeMs)
	}
}

func TestNetCheckerHTTP_Down(t *testing.T) {
	c := &NetChecker{}
	m := Monitor{
		ID:        "m2",
		Type:      CheckHTTP,
		Target:    "http://127.0.0.1:1", // nothing listening on port 1
		TimeoutMs: 200,
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusDown {
		t.Errorf("expected down, got %s", result.Status)
	}
	if result.Error == "" {
		t.Error("expected non-empty error for failed connection")
	}
}

func TestNetCheckerHTTP_KeywordFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "application is healthy")
	}))
	defer srv.Close()

	c := &NetChecker{}
	m := Monitor{
		ID:        "m3",
		Type:      CheckHTTP,
		Target:    srv.URL,
		TimeoutMs: 5000,
		Expected:  "healthy",
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusUp {
		t.Errorf("expected up (keyword found), got %s: %s", result.Status, result.Error)
	}
}

func TestNetCheckerHTTP_KeywordMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "service unavailable")
	}))
	defer srv.Close()

	c := &NetChecker{}
	m := Monitor{
		ID:        "m4",
		Type:      CheckHTTP,
		Target:    srv.URL,
		TimeoutMs: 5000,
		Expected:  "healthy",
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusDown {
		t.Errorf("expected down (keyword missing), got %s", result.Status)
	}
}

// --- TCP checker ---

func TestNetCheckerTCP_Up(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	c := &NetChecker{}
	m := Monitor{
		ID:        "m5",
		Type:      CheckTCP,
		Target:    ln.Addr().String(),
		TimeoutMs: 2000,
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusUp {
		t.Errorf("expected up, got %s: %s", result.Status, result.Error)
	}
}

func TestNetCheckerTCP_Down(t *testing.T) {
	c := &NetChecker{}
	m := Monitor{
		ID:        "m6",
		Type:      CheckTCP,
		Target:    "127.0.0.1:1", // nothing listening
		TimeoutMs: 200,
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusDown {
		t.Errorf("expected down, got %s", result.Status)
	}
}

// --- unknown check type ---

// TestNetCheckerUnknownType verifies that an unknown CheckType returns StatusDown
// with a non-empty Error and does not panic.
func TestNetCheckerUnknownType(t *testing.T) {
	c := &NetChecker{}
	m := Monitor{
		ID:        "m7",
		Type:      CheckType("grpc"),
		Target:    "127.0.0.1:9090",
		TimeoutMs: 200,
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusDown {
		t.Errorf("unknown type: expected down, got %s", result.Status)
	}
	if result.Error == "" {
		t.Error("unknown type: expected non-empty Error")
	}
}

// --- classifyHTTP boundary table ---

// TestClassifyHTTPBoundaries covers the full status-code × time matrix including
// the 80%-of-timeout degraded boundary.
func TestClassifyHTTPBoundaries(t *testing.T) {
	const timeout = 5000
	cases := []struct {
		code, elapsed int
		want          CheckStatus
	}{
		// 2xx/3xx below threshold → Up
		{200, 100, StatusUp},
		{201, 100, StatusUp},
		{204, 100, StatusUp},
		{301, 100, StatusUp},
		{302, 100, StatusUp},
		{399, 100, StatusUp},
		// 2xx/3xx above 80% threshold → Degraded (elapsedMs > timeoutMs*80/100)
		{200, 4001, StatusDegraded}, // 4001 > 4000 (80% of 5000)
		{200, 5000, StatusDegraded}, // at timeout itself
		// 2xx/3xx exactly at 80% boundary (4000ms): NOT strictly greater → Up
		{200, 4000, StatusUp},
		// 4xx/5xx → Down regardless of response time
		{400, 100, StatusDown},
		{401, 100, StatusDown},
		{403, 100, StatusDown},
		{404, 100, StatusDown},
		{429, 100, StatusDown},
		{500, 100, StatusDown},
		{503, 100, StatusDown},
	}
	for _, tc := range cases {
		got := classifyHTTP(tc.code, tc.elapsed, timeout)
		if got != tc.want {
			t.Errorf("classifyHTTP(%d, %d, %d) = %s, want %s",
				tc.code, tc.elapsed, timeout, got, tc.want)
		}
	}
}

// --- HTTP checker with bad URL ---

// TestNetCheckerHTTP_BadURL verifies that a malformed URL returns StatusDown
// with a non-empty Error and does not panic.
func TestNetCheckerHTTP_BadURL(t *testing.T) {
	c := &NetChecker{}
	m := Monitor{
		ID:        "m8",
		Type:      CheckHTTP,
		Target:    "://not-a-url",
		TimeoutMs: 500,
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusDown {
		t.Errorf("bad URL: expected down, got %s", result.Status)
	}
	if result.Error == "" {
		t.Error("bad URL: expected non-empty Error")
	}
}

// TestNetCheckerHTTP_StatusCodeExpectedSkipsKeyword verifies that when Expected
// is a 3-digit string (status code sentinel) the keyword-check branch is NOT
// entered — the check passes on HTTP status alone even when the body does not
// contain the string "200".
func TestNetCheckerHTTP_StatusCodeExpectedSkipsKeyword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Body does NOT contain "200" — keyword-check would fail if entered.
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "hello")
	}))
	defer srv.Close()

	c := &NetChecker{}
	m := Monitor{
		ID:        "m9",
		Type:      CheckHTTP,
		Target:    srv.URL,
		TimeoutMs: 5000,
		Expected:  "200", // looks like a status code → keyword branch skipped
	}
	result := c.Check(context.Background(), m)
	if result.Status != StatusUp {
		t.Errorf("status-code Expected: got %s, want up", result.Status)
	}
}

// --- dbToMonitor conversion ---

// TestDbToMonitor verifies that all fields are copied correctly from db.UptimeMonitor.
func TestDbToMonitor(t *testing.T) {
	nodeID := "n1"
	src := db.UptimeMonitor{
		ID:              "mid",
		Name:            "my-monitor",
		Type:            "http",
		Target:          "https://example.com/health",
		IntervalSeconds: 60,
		TimeoutMs:       5000,
		Expected:        "healthy",
		Enabled:         true,
		NodeID:          &nodeID,
	}
	got := dbToMonitor(src)
	if got.ID != src.ID {
		t.Errorf("ID: got %q, want %q", got.ID, src.ID)
	}
	if got.Name != src.Name {
		t.Errorf("Name: got %q, want %q", got.Name, src.Name)
	}
	if got.Type != CheckType(src.Type) {
		t.Errorf("Type: got %q, want %q", got.Type, src.Type)
	}
	if got.Target != src.Target {
		t.Errorf("Target: got %q, want %q", got.Target, src.Target)
	}
	if got.IntervalSeconds != src.IntervalSeconds {
		t.Errorf("IntervalSeconds: got %d, want %d", got.IntervalSeconds, src.IntervalSeconds)
	}
	if got.TimeoutMs != src.TimeoutMs {
		t.Errorf("TimeoutMs: got %d, want %d", got.TimeoutMs, src.TimeoutMs)
	}
	if got.Expected != src.Expected {
		t.Errorf("Expected: got %q, want %q", got.Expected, src.Expected)
	}
	if !got.Enabled {
		t.Error("Enabled: got false, want true")
	}
	if got.NodeID == nil || *got.NodeID != nodeID {
		t.Errorf("NodeID: got %v, want %q", got.NodeID, nodeID)
	}
}

// TestDbToMonitor_NilNodeID verifies that a nil NodeID is preserved without panic.
func TestDbToMonitor_NilNodeID(t *testing.T) {
	src := db.UptimeMonitor{ID: "m1", NodeID: nil}
	got := dbToMonitor(src)
	if got.NodeID != nil {
		t.Errorf("NodeID: got %v, want nil", got.NodeID)
	}
}
