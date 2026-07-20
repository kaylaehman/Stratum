package api

import (
	"testing"
	"time"
)

func TestKeyedLimiter_BurstThenThrottle(t *testing.T) {
	// Refill is deliberately slow so the burst is the only budget within the test.
	l := NewKeyedLimiter(time.Hour, 3)

	allowed := 0
	for i := 0; i < 5; i++ {
		if l.Allow("1.2.3.4") {
			allowed++
		}
	}
	if allowed != 3 {
		t.Fatalf("burst=3: allowed %d of 5, want 3", allowed)
	}
}

func TestKeyedLimiter_PerKeyIsolation(t *testing.T) {
	l := NewKeyedLimiter(time.Hour, 1)

	if !l.Allow("a") {
		t.Fatal("first request for key a should be allowed")
	}
	if l.Allow("a") {
		t.Fatal("second request for key a should be throttled (burst=1)")
	}
	// A different key must have its own bucket — one abusive IP must not throttle
	// everyone else on an unauthenticated endpoint.
	if !l.Allow("b") {
		t.Fatal("key b must not be affected by key a's exhausted bucket")
	}
}

func TestKeyedLimiter_NilAllows(t *testing.T) {
	var l *keyedLimiter
	if !l.Allow("anything") {
		t.Fatal("nil limiter must fail open (allow)")
	}
}
