package docker

import (
	"context"
	"testing"
	"time"
)

// TestNew_LocalDefault verifies that constructing a client with the local
// default (empty endpoint, no TLS) succeeds without requiring a live daemon.
func TestNew_LocalDefault(t *testing.T) {
	c, err := New("", nil)
	if err != nil {
		t.Fatalf("New(\"\", nil) unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("New(\"\", nil) returned nil client")
	}
}

// TestNew_InvalidTLS verifies that malformed PEM material is rejected at
// construction time (before any network call is made).
func TestNew_InvalidTLS(t *testing.T) {
	bogus := &TLS{
		CA:   "not-valid-pem",
		Cert: "not-valid-pem",
		Key:  "not-valid-pem",
	}
	_, err := New("tcp://127.0.0.1:2376", bogus)
	if err == nil {
		t.Fatal("New with invalid PEM expected an error, got nil")
	}
}

// TestClose_Fresh verifies that Close on a freshly-constructed client returns nil.
func TestClose_Fresh(t *testing.T) {
	c, err := New("", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() unexpected error: %v", err)
	}
}

// TestPing_Unreachable verifies that Ping against an unreachable endpoint
// returns an error quickly (no hanging). No live daemon required.
func TestPing_Unreachable(t *testing.T) {
	// Use a TCP endpoint that is guaranteed to refuse connections fast.
	c, err := New("tcp://127.0.0.1:19999", nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = c.Ping(ctx)
	if err == nil {
		t.Fatal("Ping to unreachable endpoint expected error, got nil")
	}
}
