package uptime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
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
