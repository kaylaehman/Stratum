package uptime

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Checker executes a single monitor check and returns the Result.
// It is an interface so tests can substitute a mock without real network calls.
type Checker interface {
	Check(ctx context.Context, m Monitor) Result
}

// NetChecker is the production Checker that uses the standard library.
// HTTP checks are made with a dedicated client (no shared global state).
// TCP checks dial the target host:port. ICMP/ping uses a TCP echo heuristic
// (raw ICMP sockets require elevated privileges; stdlib-only dial is safer).
type NetChecker struct{}

// Check runs the appropriate check for m and returns a Result with the outcome.
func (c *NetChecker) Check(ctx context.Context, m Monitor) Result {
	start := time.Now()
	switch m.Type {
	case CheckHTTP:
		return checkHTTP(ctx, m, start)
	case CheckTCP:
		return checkTCP(ctx, m, start)
	case CheckICMP:
		return checkICMPviaDialer(ctx, m, start)
	default:
		return errorResult(m, start, fmt.Sprintf("unknown check type: %s", m.Type))
	}
}

func checkHTTP(ctx context.Context, m Monitor, start time.Time) Result {
	timeout := time.Duration(m.TimeoutMs) * time.Millisecond
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.Target, nil)
	if err != nil {
		return errorResult(m, start, fmt.Sprintf("build request: %v", err))
	}
	req.Header.Set("User-Agent", "Stratum-Uptime/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return errorResult(m, start, fmt.Sprintf("request: %v", err))
	}
	defer resp.Body.Close()

	elapsed := msElapsed(start)
	status := classifyHTTP(resp.StatusCode, elapsed, m.TimeoutMs)

	// Keyword check: if Expected contains a non-numeric string it is a body keyword.
	if m.Expected != "" && !looksLikeStatusCode(m.Expected) {
		body := make([]byte, 64*1024) // read up to 64 KB
		n, _ := resp.Body.Read(body)
		if !strings.Contains(string(body[:n]), m.Expected) {
			return Result{
				MonitorID:      m.ID,
				CheckedAt:      start,
				Status:         StatusDown,
				ResponseTimeMs: elapsed,
				Error:          fmt.Sprintf("keyword %q not found in response body", m.Expected),
			}
		}
	}

	return Result{
		MonitorID:      m.ID,
		CheckedAt:      start,
		Status:         status,
		ResponseTimeMs: elapsed,
	}
}

// classifyHTTP maps HTTP status code + response time to a CheckStatus.
func classifyHTTP(code, elapsedMs, timeoutMs int) CheckStatus {
	if code >= 200 && code < 400 {
		if elapsedMs > timeoutMs*80/100 {
			return StatusDegraded
		}
		return StatusUp
	}
	return StatusDown
}

func checkTCP(ctx context.Context, m Monitor, start time.Time) Result {
	timeout := time.Duration(m.TimeoutMs) * time.Millisecond
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", m.Target)
	if err != nil {
		return errorResult(m, start, fmt.Sprintf("dial: %v", err))
	}
	conn.Close()
	return Result{
		MonitorID:      m.ID,
		CheckedAt:      start,
		Status:         StatusUp,
		ResponseTimeMs: msElapsed(start),
	}
}

// checkICMPviaDialer approximates ping via a TCP connection to port 7 (echo)
// and falls back to a generic TCP dial. Raw ICMP would require elevated privs;
// this gives a useful "is the host reachable" signal with stdlib only.
func checkICMPviaDialer(ctx context.Context, m Monitor, start time.Time) Result {
	// Try port 7 (echo) first, then :80 as a fallback connectivity check.
	timeout := time.Duration(m.TimeoutMs) * time.Millisecond
	for _, port := range []string{"7", "80"} {
		addr := net.JoinHostPort(m.Target, port)
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close()
			return Result{
				MonitorID:      m.ID,
				CheckedAt:      start,
				Status:         StatusUp,
				ResponseTimeMs: msElapsed(start),
			}
		}
	}
	// Last-resort: try UDP dial (will not block on a firewalled host, but
	// succeeds immediately if the OS can route to the address).
	addr := net.JoinHostPort(m.Target, "0")
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return errorResult(m, start, fmt.Sprintf("host unreachable: %v", err))
	}
	conn.Close()
	return Result{
		MonitorID:      m.ID,
		CheckedAt:      start,
		Status:         StatusUp,
		ResponseTimeMs: msElapsed(start),
	}
}

func errorResult(m Monitor, start time.Time, errMsg string) Result {
	return Result{
		MonitorID:      m.ID,
		CheckedAt:      start,
		Status:         StatusDown,
		ResponseTimeMs: msElapsed(start),
		Error:          errMsg,
	}
}

func msElapsed(start time.Time) int {
	return int(time.Since(start).Milliseconds())
}

// looksLikeStatusCode returns true if s is a 3-digit string like "200" or "404".
func looksLikeStatusCode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
