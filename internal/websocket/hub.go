package websocket

import (
	"log"
	"sync"

	"github.com/google/uuid"
)

// Hub maintains the set of active clients and broadcasts messages
type Hub struct {
	// Map of client connections indexed by userID
	clients map[uuid.UUID]*Client

	// Map of room subscriptions indexed by chatID
	rooms map[uuid.UUID]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Lock for thread safety
	mu sync.RWMutex
}

// Client represents a connected websocket client
type Client struct {
	hub      *Hub
	userID   uuid.UUID
	send     chan []byte
	roomSubs map[uuid.UUID]bool
	mu       sync.RWMutex
}

// Global hub accessible throughout the app
var GlobalHub = NewHub()

// NewHub creates a new hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uuid.UUID]*Client),
		rooms:      make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub processing loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.userID] = client
			h.mu.Unlock()
			log.Printf("Client registered: %s", client.userID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.userID]; ok {
				delete(h.clients, client.userID)
				close(client.send)
				
				// Remove client from all rooms
				client.mu.RLock()
				for roomID := range client.roomSubs {
					if room, exists := h.rooms[roomID]; exists {
						delete(room, client)
						// Clean up empty rooms
						if len(room) == 0 {
							delete(h.rooms, roomID)
						}
					}
				}
				client.mu.RUnlock()
				
				log.Printf("Client unregistered: %s", client.userID)
			}
			h.mu.Unlock()
		}
	}
}

// SubscribeToRoom adds a client to a chat room
func (h *Hub) SubscribeToRoom(client *Client, roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if _, exists := h.rooms[roomID]; !exists {
		h.rooms[roomID] = make(map[*Client]bool)
	}
	h.rooms[roomID][client] = true
	
	client.mu.Lock()
	client.roomSubs[roomID] = true
	client.mu.Unlock()
	
	log.Printf("Client %s subscribed to room %s", client.userID, roomID)
}

// UnsubscribeFromRoom removes a client from a chat room
func (h *Hub) UnsubscribeFromRoom(client *Client, roomID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	if room, exists := h.rooms[roomID]; exists {
		delete(room, client)
		// Clean up empty rooms
		if len(room) == 0 {
			delete(h.rooms, roomID)
		}
	}
	
	client.mu.Lock()
	delete(client.roomSubs, roomID)
	client.mu.Unlock()
	
	log.Printf("Client %s unsubscribed from room %s", client.userID, roomID)
}

// BroadcastToRoom sends a message to all clients in a room
func (h *Hub) BroadcastToRoom(roomID uuid.UUID, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if room, exists := h.rooms[roomID]; exists {
		for client := range room {
			select {
			case client.send <- message:
				// Message sent successfully
			default:
				// Failed to send, client may be slow or disconnected
				go h.unregister <- client
			}
		}
	}
}

// SendToUser sends a message to a specific user
func (h *Hub) SendToUser(userID uuid.UUID, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if client, exists := h.clients[userID]; exists {
		select {
		case client.send <- message:
			// Message sent successfully
		default:
			// Failed to send, client may be slow or disconnected
			go h.unregister <- client
		}
	}
}