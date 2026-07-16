package broker

import (
	"encoding/json"
	"sync"
)

// SSEBroker manages connected SSE clients.
type SSEBroker struct {
	clients map[chan []byte]bool
	mu      sync.Mutex
}

// GlobalSSEBroker is the singleton broker.
var GlobalSSEBroker = NewSSEBroker()

// NewSSEBroker creates a new SSE broker.
func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		clients: make(map[chan []byte]bool),
	}
}

// AddClient adds a new client channel.
func (b *SSEBroker) AddClient(c chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients[c] = true
}

// RemoveClient removes a client channel.
func (b *SSEBroker) RemoveClient(c chan []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.clients, c)
	close(c)
}

// Broadcast sends a message to all connected SSE clients.
func (b *SSEBroker) Broadcast(eventType string, data any) {
	b.mu.Lock()
	defer b.mu.Unlock()

	payload, err := json.Marshal(data)
	if err != nil {
		return
	}

	for client := range b.clients {
		select {
		case client <- payload:
		default:
			// If client buffer is full, drop the message.
		}
	}
}
