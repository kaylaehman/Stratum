// Package hub is an in-process WebSocket fan-out registry. Connections register
// to receive messages published on opaque string topics (e.g. "logs:<id>",
// "tree:<nodeId>"). Sends are buffered per client; a client whose buffer is full
// (a slow consumer) has the message dropped rather than blocking the publisher —
// downstream features (SP2) detect the gap and resync.
package hub

import (
	"sync"

	"github.com/google/uuid"
)

// defaultBuffer is the per-client send-channel capacity. Sized for log
// firehoses; slow clients drop (counted) rather than blocking publishers.
const defaultBuffer = 256

// ClientID identifies a registered connection.
type ClientID string

type client struct {
	id     ClientID
	userID string
	send   chan []byte
	topics map[string]struct{}
}

// Hub fans out messages to subscribed clients. It is safe for concurrent use.
type Hub struct {
	mu      sync.Mutex
	clients map[ClientID]*client
	topics  map[string]map[ClientID]struct{}
	bufSize int

	// dropped counts messages skipped due to full client buffers (observability).
	dropped uint64
}

// New creates an empty Hub.
func New() *Hub {
	return &Hub{
		clients: make(map[ClientID]*client),
		topics:  make(map[string]map[ClientID]struct{}),
		bufSize: defaultBuffer,
	}
}

// Register adds a new client (owned by userID) and returns its id plus the
// channel the caller's writer goroutine pumps to the connection. The channel is
// closed when the client is unsubscribed.
func (h *Hub) Register(userID string) (ClientID, <-chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := &client{
		id:     ClientID(uuid.NewString()),
		userID: userID,
		send:   make(chan []byte, h.bufSize),
		topics: make(map[string]struct{}),
	}
	h.clients[c.id] = c
	return c.id, c.send
}

// ClientUser returns the userID that owns a client, so an HTTP endpoint can
// verify a caller may grant that connection a (sensitive) topic before calling
// Subscribe server-side.
func (h *Hub) ClientUser(id ClientID) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[id]
	if !ok {
		return "", false
	}
	return c.userID, true
}

// Subscribe joins a client to a topic. Unknown client ids are ignored.
func (h *Hub) Subscribe(id ClientID, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[id]
	if !ok {
		return
	}
	c.topics[topic] = struct{}{}
	subs := h.topics[topic]
	if subs == nil {
		subs = make(map[ClientID]struct{})
		h.topics[topic] = subs
	}
	subs[id] = struct{}{}
}

// Unsubscribe removes a client entirely, detaching it from all topics and
// closing its send channel. Safe to call more than once.
func (h *Hub) Unsubscribe(id ClientID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c, ok := h.clients[id]
	if !ok {
		return
	}
	for topic := range c.topics {
		if subs := h.topics[topic]; subs != nil {
			delete(subs, id)
			if len(subs) == 0 {
				delete(h.topics, topic)
			}
		}
	}
	delete(h.clients, id)
	close(c.send)
}

// Broadcast publishes payload to every client subscribed to topic. A client
// whose buffer is full has this message dropped (non-blocking send) so one slow
// consumer never stalls the publisher or other subscribers.
func (h *Hub) Broadcast(topic string, payload []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for id := range h.topics[topic] {
		c := h.clients[id]
		if c == nil {
			continue
		}
		select {
		case c.send <- payload:
		default:
			h.dropped++
		}
	}
}

// Dropped returns the number of messages dropped due to full client buffers.
func (h *Hub) Dropped() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dropped
}

// ClientCount returns the number of registered clients (for tests/metrics).
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
