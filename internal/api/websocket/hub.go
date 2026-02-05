package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"sixtyseven/internal/domain"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for now
	},
}

// Client represents a WebSocket client
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	runID  uuid.UUID
	send   chan []byte
	closed bool
	mu     sync.Mutex
}

// Hub manages WebSocket connections and broadcasts
type Hub struct {
	// Clients subscribed to each run
	clients map[uuid.UUID]map[*Client]bool
	mu      sync.RWMutex

	// Channel for broadcasting messages
	broadcast chan broadcastMessage

	// Register/unregister clients
	register   chan *Client
	unregister chan *Client

	// Shutdown
	done chan struct{}
}

type broadcastMessage struct {
	runID uuid.UUID
	data  []byte
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]map[*Client]bool),
		broadcast:  make(chan broadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.runID] == nil {
				h.clients[client.runID] = make(map[*Client]bool)
			}
			h.clients[client.runID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.runID]; ok {
				delete(h.clients[client.runID], client)
				if len(h.clients[client.runID]) == 0 {
					delete(h.clients, client.runID)
				}
			}
			client.mu.Lock()
			if !client.closed {
				close(client.send)
				client.closed = true
			}
			client.mu.Unlock()
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[msg.runID]
			for client := range clients {
				select {
				case client.send <- msg.data:
				default:
					// Client buffer full, skip
				}
			}
			h.mu.RUnlock()

		case <-h.done:
			return
		}
	}
}

// Close shuts down the hub
func (h *Hub) Close() {
	close(h.done)

	h.mu.Lock()
	defer h.mu.Unlock()

	for runID := range h.clients {
		for client := range h.clients[runID] {
			client.mu.Lock()
			if !client.closed {
				close(client.send)
				client.closed = true
			}
			client.mu.Unlock()
		}
	}
}

// Broadcast sends a metric update to all clients subscribed to a run
func (h *Hub) Broadcast(runID uuid.UUID, update domain.MetricUpdate) {
	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("Failed to marshal metric update: %v", err)
		return
	}

	select {
	case h.broadcast <- broadcastMessage{runID: runID, data: data}:
	default:
		// Broadcast buffer full, log and skip
		log.Printf("Broadcast buffer full for run %s", runID)
	}
}

// HandleConnection handles a new WebSocket connection
func (h *Hub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	runIDStr := chi.URLParam(r, "runID")
	runID, err := uuid.Parse(runIDStr)
	if err != nil {
		http.Error(w, "invalid run ID", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:   h,
		conn:  conn,
		runID: runID,
		send:  make(chan []byte, 256),
	}

	h.register <- client

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
		// We don't expect any messages from clients, but keep the connection alive
	}
}

// ClientCount returns the number of clients subscribed to a run
func (h *Hub) ClientCount(runID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[runID])
}

// TotalClientCount returns the total number of connected clients
func (h *Hub) TotalClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}
