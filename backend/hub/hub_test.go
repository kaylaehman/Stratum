package hub

import (
	"sync"
	"testing"
	"time"
)

func TestBroadcastDeliversToSubscribers(t *testing.T) {
	h := New()
	id, ch := h.Register()
	h.Subscribe(id, "topic-a")

	h.Broadcast("topic-a", []byte("hello"))

	select {
	case msg := <-ch:
		if string(msg) != "hello" {
			t.Errorf("got %q, want hello", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive broadcast")
	}
}

func TestBroadcastSkipsNonSubscribers(t *testing.T) {
	h := New()
	id, ch := h.Register()
	h.Subscribe(id, "topic-a")

	h.Broadcast("topic-b", []byte("nope"))

	select {
	case msg := <-ch:
		t.Errorf("received unexpected message %q on unsubscribed topic", msg)
	case <-time.After(50 * time.Millisecond):
		// expected: nothing delivered
	}
}

func TestUnsubscribeRemovesAndCloses(t *testing.T) {
	h := New()
	id, ch := h.Register()
	h.Subscribe(id, "t")
	h.Unsubscribe(id)

	if h.ClientCount() != 0 {
		t.Errorf("ClientCount = %d after unsubscribe, want 0", h.ClientCount())
	}
	if _, open := <-ch; open {
		t.Error("send channel should be closed after unsubscribe")
	}
	// Broadcasting to a now-empty topic must not panic.
	h.Broadcast("t", []byte("x"))
}

func TestSlowClientDropsInsteadOfBlocking(t *testing.T) {
	h := New()
	id, _ := h.Register() // never drain the channel
	h.Subscribe(id, "flood")

	// Publish more than the buffer; must not block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < defaultBuffer*4; i++ {
			h.Broadcast("flood", []byte("m"))
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Broadcast blocked on a slow client")
	}
	if h.Dropped() == 0 {
		t.Error("expected some dropped messages for a slow client")
	}
}

func TestConcurrentRegisterBroadcastUnsubscribe(t *testing.T) {
	h := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, ch := h.Register()
			go func() {
				for range ch {
				}
			}()
			h.Subscribe(id, "shared")
			h.Broadcast("shared", []byte("ping"))
			h.Unsubscribe(id)
		}()
	}
	wg.Wait()
	if h.ClientCount() != 0 {
		t.Errorf("ClientCount = %d, want 0", h.ClientCount())
	}
}
