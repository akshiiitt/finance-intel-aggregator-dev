package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/financeintel/backend/internal/api/middleware"
)

// Message is the standard envelope sent to all connected WebSocket clients.
type Message struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"data"`
	TS      int64           `json:"ts"`
}

// client represents one connected WebSocket browser tab.
type client struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
}

// Hub manages all active WebSocket connections and broadcasts messages to them.
// It is safe for concurrent use from multiple goroutines (workers + HTTP handlers).
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}

	broadcast  chan []byte
	register   chan *client
	unregister chan *client
	pool       *pgxpool.Pool
	closed     chan struct{} // closed when run() exits (shutdown) — unblocks channel sends

	// multiInstance enables cross-process fan-out via Postgres LISTEN/NOTIFY.
	// When false, Broadcast delivers directly to local clients (single-VM).
	multiInstance bool

	upgrader websocket.Upgrader
}

// New creates and starts a Hub. Call this once at startup. allowedOrigins is
// the exact set of browser origins permitted to open the socket (same list as
// CORS); multiInstance turns on Postgres-backed cross-process broadcasting.
func New(ctx context.Context, pool *pgxpool.Pool, allowedOrigins []string, multiInstance bool) *Hub {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	h := &Hub{
		clients:       make(map[*client]struct{}),
		broadcast:     make(chan []byte, 256),
		register:      make(chan *client, 32),
		unregister:    make(chan *client, 32),
		pool:          pool,
		closed:        make(chan struct{}),
		multiInstance: multiInstance,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			// Reject cross-site WebSocket hijacking: a browser always sends an
			// Origin header on the WS handshake, so an attacker's page is
			// blocked unless its origin is explicitly allow-listed. A missing
			// Origin (non-browser client like curl/a Go service) is allowed —
			// those aren't a CSRF vector.
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || allowed[origin]
			},
		},
	}
	go h.run(ctx)
	if pool != nil && multiInstance {
		h.startListener(ctx, pool)
	}
	return h
}

// run is the central event loop — it registers/unregisters clients and fans out broadcasts.
func (h *Hub) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			// Signal shutdown first so any readPump/ServeWS blocked sending to
			// register/unregister unblocks instead of leaking (their channels
			// are buffered but can fill under load during shutdown).
			close(h.closed)
			// Close all clients gracefully on shutdown
			h.mu.Lock()
			for c := range h.clients {
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Server shutting down"))
				c.conn.Close()
			}
			h.clients = make(map[*client]struct{})
			h.mu.Unlock()
			log.Info().Msg("ws: hub event loop stopped cleanly")
			return

		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
			log.Debug().Int("total", h.count()).Msg("ws: client connected")

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
				middleware.DecWSClients()
			}
			h.mu.Unlock()
			log.Debug().Int("total", h.count()).Msg("ws: client disconnected")

		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Slow client — drop the message to avoid blocking the hub.
				}
			}
			h.mu.RUnlock()
		}
	}
}

// startListener listens to PostgreSQL notifications and broadcasts them locally.
func (h *Hub) startListener(ctx context.Context, pool *pgxpool.Pool) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := pool.Acquire(ctx)
			if err != nil {
				log.Error().Err(err).Msg("ws pg listener: failed to acquire connection, retrying in 5s")
				time.Sleep(5 * time.Second)
				continue
			}

			_, err = conn.Exec(ctx, "LISTEN ws_broadcast")
			if err != nil {
				conn.Release()
				log.Error().Err(err).Msg("ws pg listener: LISTEN failed, retrying in 5s")
				time.Sleep(5 * time.Second)
				continue
			}

			log.Info().Msg("ws pg listener: listening to 'ws_broadcast'")

			for {
				notification, err := conn.Conn().WaitForNotification(ctx)
				if err != nil {
					log.Error().Err(err).Msg("ws pg listener: error waiting for notification, reconnecting")
					break
				}

				select {
				case h.broadcast <- []byte(notification.Payload):
				default:
				}
			}

			conn.Release()
			time.Sleep(1 * time.Second)
		}
	}()
}

// count returns the number of currently connected clients (must hold at least RLock).
func (h *Hub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast serializes a typed payload and sends it to every connected client.
func (h *Hub) Broadcast(msgType string, payload interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("ws: marshal payload failed")
		return
	}

	msg := Message{
		Type:    msgType,
		Payload: json.RawMessage(data),
		TS:      time.Now().UnixMilli(),
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}

	// Single-instance (default): deliver directly to local clients. This
	// avoids a Postgres round-trip and a spawned goroutine per broadcast, and
	// means no dedicated LISTEN connection is held — all of which matters on
	// the free-tier connection budget. Only fan out through pg_notify when
	// explicitly running multiple processes.
	if h.multiInstance && h.pool != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, err := h.pool.Exec(ctx, "SELECT pg_notify('ws_broadcast', $1)", string(raw))
			if err != nil {
				log.Error().Err(err).Msg("ws: pg_notify failed")
			}
		}()
		return
	}

	select {
	case h.broadcast <- raw:
	default:
		log.Warn().Msg("ws: broadcast channel full, dropping message")
	}
}

// ServeWS upgrades an HTTP connection to WebSocket and registers the client.
// This is the handler for GET /api/ws/terminal.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	const maxClients = 1000
	if h.count() >= maxClients {
		log.Warn().Msg("ws: max client limit reached, refusing connection")
		http.Error(w, "Connection limit exceeded", http.StatusServiceUnavailable)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("ws: upgrade failed")
		return
	}

	c := &client{
		conn: conn,
		send: make(chan []byte, 64),
		done: make(chan struct{}),
	}

	// Don't block forever registering during shutdown.
	select {
	case h.register <- c:
	case <-h.closed:
		conn.Close()
		return
	}
	middleware.IncWSClients()

	// Send welcome message immediately after connecting.
	welcome, _ := json.Marshal(Message{
		Type: "SYSTEM_CONNECTED",
		Payload: json.RawMessage(`{"message":"FinanceIntel Go Backend Connected","timestamp":"` +
			time.Now().Format(time.RFC3339) + `"}`),
		TS: time.Now().UnixMilli(),
	})
	c.send <- welcome

	// Start read/write pumps as goroutines.
	go c.writePump(h)
	go c.readPump(h)
}

// writePump sends messages from the client's send channel to the WebSocket connection.
func (c *client) writePump(h *Hub) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump reads from the WebSocket to detect disconnections and handle pings.
func (c *client) readPump(h *Hub) {
	defer func() {
		// Non-blocking during shutdown — run() may have already exited.
		select {
		case h.unregister <- c:
		case <-h.closed:
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				log.Debug().Err(err).Msg("ws: unexpected close")
			}
			return
		}
	}
}
