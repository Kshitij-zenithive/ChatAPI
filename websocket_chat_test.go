package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Message represents a simple chat message
type ChatMessage struct {
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Mentions  []string  `json:"mentions,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ChatHub maintains the set of active clients and broadcasts messages
type ChatHub struct {
	// Registered clients
	clients map[*ChatClient]bool

	// Register requests from clients
	register chan *ChatClient

	// Unregister requests from clients
	unregister chan *ChatClient

	// Inbound messages from clients
	broadcast chan ChatMessage

	// Message history
	history     []ChatMessage
	historyLock sync.RWMutex
}

// ChatClient represents a single websocket connection
type ChatClient struct {
	hub *ChatHub

	// The websocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan ChatMessage

	// User information
	userID   string
	username string
}

// Initialize a new chat hub
func newChatHub() *ChatHub {
	return &ChatHub{
		clients:    make(map[*ChatClient]bool),
		register:   make(chan *ChatClient),
		unregister: make(chan *ChatClient),
		broadcast:  make(chan ChatMessage),
		history:    make([]ChatMessage, 0),
	}
}

// Run the chat hub
func (h *ChatHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
			// Send chat history to new client
			h.historyLock.RLock()
			for _, msg := range h.history {
				client.send <- msg
			}
			h.historyLock.RUnlock()

		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}

		case message := <-h.broadcast:
			// Store in history
			h.historyLock.Lock()
			h.history = append(h.history, message)
			h.historyLock.Unlock()

			// Send to all clients
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// Parse mentions from message content (simple implementation)
func parseMentions(content string) []string {
	// This is a simple placeholder implementation
	// In a real application, you would parse @username mentions from content
	return []string{"user1", "user2"}
}

// Process incoming messages
func (c *ChatClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// Set message size limit and read deadline
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Parse the message
		var messageData struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(msgBytes, &messageData); err != nil {
			log.Printf("error parsing message: %v", err)
			continue
		}

		// Create a new message with parsed mentions
		mentions := parseMentions(messageData.Content)
		message := ChatMessage{
			ID:        uuid.New().String(),
			Sender:    c.username,
			Content:   messageData.Content,
			Mentions:  mentions,
			Timestamp: time.Now(),
		}

		// Send the message to hub for broadcasting
		c.hub.broadcast <- message

		// Log timeline event (in a real app this would go to the database)
		log.Printf("Timeline Event: User %s sent a message", c.username)
		if len(mentions) > 0 {
			log.Printf("Timeline Event: User %s mentioned users: %v", c.username, mentions)
		}
	}
}

// Send messages to the client
func (c *ChatClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			
			// Marshal the message to JSON
			messageJSON, _ := json.Marshal(message)
			w.Write(messageJSON)

			if err := w.Close(); err != nil {
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

// Configure WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections for testing
	},
}

// ServeWs handles WebSocket requests from clients
func serveWs(hub *ChatHub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Get user ID and name from query parameters (in a real app, this would come from auth)
	userID := r.URL.Query().Get("user_id")
	username := r.URL.Query().Get("username")
	
	if userID == "" || username == "" {
		// Generate random IDs for testing
		userID = uuid.New().String()
		username = fmt.Sprintf("User-%s", userID[:5])
	}

	// Create a new client
	client := &ChatClient{
		hub:      hub,
		conn:     conn,
		send:     make(chan ChatMessage, 256),
		userID:   userID,
		username: username,
	}
	
	// Register client
	client.hub.register <- client

	// Send welcome message
	welcomeMsg := ChatMessage{
		ID:        uuid.New().String(),
		Sender:    "System",
		Content:   fmt.Sprintf("Welcome to the chat, %s!", username),
		Timestamp: time.Now(),
	}
	client.send <- welcomeMsg

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()
}

// Initialize and start the chat test server
func initChatTestServer(mux *http.ServeMux) {
	// Create a new hub
	hub := newChatHub()
	go hub.run()

	// Add chat test route
	mux.HandleFunc("/ws/chat", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	// Add test page for chat
	mux.HandleFunc("/chat-test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		html := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>CRM Chat Test</title>
			<style>
				body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
				#messages { height: 300px; overflow-y: auto; border: 1px solid #ccc; margin-bottom: 10px; padding: 10px; }
				#messageForm { display: flex; }
				#messageInput { flex-grow: 1; padding: 5px; }
				button { padding: 5px 15px; background: #4CAF50; color: white; border: none; cursor: pointer; }
				.message { margin-bottom: 10px; }
				.message .sender { font-weight: bold; }
				.message .time { color: #999; font-size: 12px; }
				.message .content { margin-top: 5px; }
				.mention { background-color: #e6f7ff; padding: 2px 4px; border-radius: 2px; }
				#status { margin-bottom: 10px; color: #999; }
			</style>
		</head>
		<body>
			<h1>CRM Chat API Test</h1>
			<p>This is a simple test interface for the CRM Chat API using WebSockets.</p>
			
			<div>
				<label for="username">Username:</label>
				<input type="text" id="username" value="TestUser" />
				<button onclick="connect()">Connect</button>
			</div>
			
			<div id="status">Disconnected</div>
			
			<div id="messages"></div>
			
			<form id="messageForm" onsubmit="sendMessage(event)">
				<input type="text" id="messageInput" placeholder="Type a message..." />
				<button type="submit">Send</button>
			</form>
			
			<script>
				let socket;
				let username = '';
				
				function connect() {
					username = document.getElementById('username').value;
					if (!username) {
						alert('Please enter a username');
						return;
					}
					
					const statusEl = document.getElementById('status');
					statusEl.textContent = 'Connecting...';
					
					// Create WebSocket connection
					const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
					const wsUrl = '${protocol}//${window.location.host}/ws/chat?username=' + encodeURIComponent(username);
					socket = new WebSocket(wsUrl);
					
					// Connection opened
					socket.addEventListener('open', function (event) {
						statusEl.textContent = 'Connected as ' + username;
						statusEl.style.color = '#4CAF50';
					});
					
					// Listen for messages
					socket.addEventListener('message', function (event) {
						const message = JSON.parse(event.data);
						displayMessage(message);
					});
					
					// Connection closed
					socket.addEventListener('close', function (event) {
						statusEl.textContent = 'Disconnected';
						statusEl.style.color = '#999';
					});
					
					// Connection error
					socket.addEventListener('error', function (event) {
						statusEl.textContent = 'Error connecting';
						statusEl.style.color = 'red';
						console.error('WebSocket error:', event);
					});
				}
				
				function sendMessage(event) {
					event.preventDefault();
					
					if (!socket || socket.readyState !== WebSocket.OPEN) {
						alert('Not connected');
						return;
					}
					
					const input = document.getElementById('messageInput');
					const message = {
						content: input.value
					};
					
					socket.send(JSON.stringify(message));
					input.value = '';
				}
				
				function displayMessage(message) {
					const messagesContainer = document.getElementById('messages');
					
					const messageEl = document.createElement('div');
					messageEl.className = 'message';
					
					const headerEl = document.createElement('div');
					
					const senderEl = document.createElement('span');
					senderEl.className = 'sender';
					senderEl.textContent = message.sender;
					headerEl.appendChild(senderEl);
					
					headerEl.appendChild(document.createTextNode(' '));
					
					const timeEl = document.createElement('span');
					timeEl.className = 'time';
					const date = new Date(message.timestamp);
					timeEl.textContent = date.toLocaleTimeString();
					headerEl.appendChild(timeEl);
					
					messageEl.appendChild(headerEl);
					
					// Process content for mentions
					let content = message.content;
					if (message.mentions && message.mentions.length > 0) {
						// Simple highlighting of mentions
						message.mentions.forEach(mention => {
							content = content.replace(
								new RegExp('@' + mention, 'g'),
								'<span class="mention">@' + mention + '</span>'
							);
						});
					}
					
					const contentEl = document.createElement('div');
					contentEl.className = 'content';
					contentEl.innerHTML = content;
					messageEl.appendChild(contentEl);
					
					messagesContainer.appendChild(messageEl);
					messagesContainer.scrollTop = messagesContainer.scrollHeight;
				}
			</script>
		</body>
		</html>
		`;
		w.Write([]byte(html))
	})

	log.Println("Chat test server initialized at /chat-test")
	log.Println("WebSocket endpoint available at /ws/chat")
}

// RegisterChatTestRoutes adds the chat test routes to the main server
func RegisterChatTestRoutes(mux *http.ServeMux) {
	initChatTestServer(mux)
}