package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/SSujoy-Samanta/leaderboard-backend/internal/models"
)

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	// Registered clients
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan []byte

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Mutex for thread-safe operations
	mu sync.RWMutex
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("✅ WebSocket client connected (total: %d)", count)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("❌ WebSocket client disconnected (total: %d)", count)

		case message := <-h.broadcast:
			h.mu.Lock()
			// We're potentially modifying the map (deleting failed clients)
			for client := range h.clients {
				select {
				case client.send <- message:
					// Successfully sent
				default:
					// Client's send buffer is full, remove client
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastScoreUpdate sends score update to all connected clients
func (h *Hub) BroadcastScoreUpdate(payload *models.ScoreUpdatePayload) {
	message := models.WebSocketMessage{
		Type:    "score_update",
		Payload: payload,
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("⚠️  Failed to marshal WebSocket message: %v", err)
		return
	}

	h.broadcast <- data
}

// BroadcastLeaderboardUpdate sends full leaderboard refresh signal
func (h *Hub) BroadcastLeaderboardUpdate() {
	message := models.WebSocketMessage{
		Type:    "leaderboard_refresh",
		Payload: map[string]string{"action": "refresh"},
	}

	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("⚠️  Failed to marshal WebSocket message: %v", err)
		return
	}

	h.broadcast <- data
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Register adds a client to the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}