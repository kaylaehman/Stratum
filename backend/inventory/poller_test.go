package inventory

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/kaylaehman/stratum/backend/crypto"
	"github.com/kaylaehman/stratum/backend/hub"
	"github.com/kaylaehman/stratum/backend/nodeconn"
)

func TestAcquireSkipIfBusy(t *testing.T) {
	p := &Poller{inflight: map[string]bool{}}
	if !p.acquire("n1") {
		t.Fatal("first acquire should succeed")
	}
	if p.acquire("n1") {
		t.Error("second acquire should be skipped while in-flight")
	}
	// A different node is independent.
	if !p.acquire("n2") {
		t.Error("acquire for a different node should succeed")
	}
	p.release("n1")
	if !p.acquire("n1") {
		t.Error("acquire after release should succeed")
	}
}

func TestRunStopsOnContextCancel(t *testing.T) {
	st := testStore(t) // creates one node "n1" with no reachable clients
	key := make([]byte, crypto.KeySize)
	cipher, _ := crypto.New(key)
	p := NewPoller(st, nodeconn.NewManager(st, cipher), hub.New(),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	p.interval = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	time.Sleep(60 * time.Millisecond) // let a couple ticks run
	cancel()

	select {
	case <-done:
		// returned promptly after cancel
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after context cancel")
	}
}
