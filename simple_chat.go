package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const defaultPort = "5000"

// Message represents a simple chat message
type ChatMessage struct {
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	Mentions  []string  `json:"mentions,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// Hub maintains the set of active clients and broadcasts messages
type ChatHub struct {
	clients    map[*ChatClient]bool
	broadcast  chan ChatMessage
	register   chan *ChatClient
	unregister chan *ChatClient
}

func newChatHub() *ChatHub {
	return &ChatHub{
		broadcast:  make(chan ChatMessage),
		register:   make(chan *ChatClient),
		unregister: make(chan *ChatClient),
		clients:    make(map[*ChatClient]bool),
	}
}

func (h *ChatHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
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

// Client represents a chat client
type ChatClient struct {
	hub      *ChatHub
	conn     *websocket.Conn
	send     chan ChatMessage
	userID   string
	username string
}

// Receive messages from the client
func (c *ChatClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, rawMessage, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error: %v", err)
			}
			break
		}

		var msgData struct {
			Content string `json:"content"`
		}

		if err := json.Unmarshal(rawMessage, &msgData); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// Parse @mentions
		content := msgData.Content
		mentions := []string{}

		// Simple parsing of @mentions
		words := strings.Fields(content)
		for _, word := range words {
			if strings.HasPrefix(word, "@") && len(word) > 1 {
				mentions = append(mentions, word[1:]) // Remove the @ symbol
			}
		}

		// Create the message
		message := ChatMessage{
			ID:        uuid.New().String(),
			Sender:    c.username,
			Content:   content,
			Mentions:  mentions,
			Timestamp: time.Now(),
		}

		// Send the message to hub for broadcasting
		c.hub.broadcast <- message
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

	// Get user ID and name from query parameters
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

// Define the HTML template for our chat test page
const chatTestHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Simple CRM Chat Test</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        .container { display: flex; gap: 20px; }
        .chat-area { flex-grow: 1; }
        .sidebar { width: 200px; border: 1px solid #ccc; padding: 10px; }
        #messages { height: 300px; overflow-y: auto; border: 1px solid #ccc; margin-bottom: 10px; padding: 10px; }
        #messageForm { display: flex; }
        #messageInput { flex-grow: 1; padding: 8px; }
        button { padding: 8px 15px; background: #4CAF50; color: white; border: none; cursor: pointer; }
        .message { margin-bottom: 12px; padding-bottom: 12px; border-bottom: 1px solid #eee; }
        .message .sender { font-weight: bold; }
        .message .time { color: #999; font-size: 12px; }
        .message .content { margin-top: 5px; }
        .mention { background-color: #e6f7ff; padding: 2px 4px; border-radius: 2px; }
        #status { margin-bottom: 10px; color: #999; }
        .sidebar h3 { margin-top: 0; }
        .sidebar ul { list-style: none; padding: 0; margin: 0; }
        .sidebar li { padding: 5px 0; cursor: pointer; }
        .sidebar li:hover { background-color: #f0f0f0; }
    </style>
</head>
<body>
    <h1>Simple CRM Chat Test</h1>
    <p>
        This is a simple test interface for the CRM Chat API using WebSockets.
        <br>To mention someone, type @ followed by their name (e.g., @John)
    </p>
    
    <div>
        <label for="usernameInput">Username:</label>
        <input type="text" id="usernameInput" value="TestUser" />
        <button id="connectButton">Connect</button>
    </div>
    
    <div id="status">Disconnected</div>
    
    <div class="container">
        <div class="chat-area">
            <div id="messages"></div>
            
            <form id="messageForm">
                <input type="text" id="messageInput" placeholder="Type a message... (Use @ to mention someone)" />
                <button type="submit">Send</button>
            </form>
        </div>
        
        <div class="sidebar">
            <h3>Suggested Users to Mention</h3>
            <ul id="userList">
                <li data-user="John">John (Sales)</li>
                <li data-user="Maria">Maria (Support)</li>
                <li data-user="Carlos">Carlos (Dev)</li>
                <li data-user="TestClient">Test Client</li>
            </ul>
        </div>
    </div>
    
    <script>
    (function() {
        // Variables
        let socket;
        let username = '';
        
        // DOM Elements
        const usernameInput = document.getElementById('usernameInput');
        const connectButton = document.getElementById('connectButton');
        const statusElement = document.getElementById('status');
        const messageForm = document.getElementById('messageForm');
        const messageInput = document.getElementById('messageInput');
        const messagesContainer = document.getElementById('messages');
        const userList = document.getElementById('userList');
        
        // Event Listeners
        connectButton.addEventListener('click', connectToChat);
        messageForm.addEventListener('submit', sendMessage);
        
        // Set up user list click handlers
        const userItems = document.querySelectorAll('#userList li');
        userItems.forEach(item => {
            item.addEventListener('click', function() {
                const user = this.getAttribute('data-user');
                mentionUser(user);
            });
        });
        
        // Connect to WebSocket
        function connectToChat() {
            username = usernameInput.value.trim();
            if (!username) {
                alert('Please enter a username');
                return;
            }
            
            statusElement.textContent = 'Connecting...';
            
            // Create WebSocket connection
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws/chat?username=' + encodeURIComponent(username);
            socket = new WebSocket(wsUrl);
            
            // Connection opened
            socket.addEventListener('open', function(event) {
                statusElement.textContent = 'Connected as ' + username;
                statusElement.style.color = '#4CAF50';
                connectButton.disabled = true;
                usernameInput.disabled = true;
            });
            
            // Listen for messages
            socket.addEventListener('message', function(event) {
                try {
                    const message = JSON.parse(event.data);
                    displayMessage(message);
                } catch (e) {
                    console.error('Error parsing message:', e);
                }
            });
            
            // Connection closed
            socket.addEventListener('close', function(event) {
                statusElement.textContent = 'Disconnected';
                statusElement.style.color = '#999';
                connectButton.disabled = false;
                usernameInput.disabled = false;
            });
            
            // Connection error
            socket.addEventListener('error', function(event) {
                statusElement.textContent = 'Error connecting';
                statusElement.style.color = 'red';
                connectButton.disabled = false;
                usernameInput.disabled = false;
            });
        }
        
        // Send message
        function sendMessage(event) {
            event.preventDefault();
            
            if (!socket || socket.readyState !== WebSocket.OPEN) {
                alert('Please connect first');
                return;
            }
            
            const content = messageInput.value.trim();
            if (content) {
                socket.send(JSON.stringify({ content: content }));
                messageInput.value = '';
            }
        }
        
        // Mention a user by clicking on their name
        function mentionUser(username) {
            if (!messageInput.value.endsWith(' ') && messageInput.value.length > 0) {
                messageInput.value += ' ';
            }
            messageInput.value += '@' + username + ' ';
            messageInput.focus();
        }
        
        // Display message
        function displayMessage(message) {
            const messageElement = document.createElement('div');
            messageElement.className = 'message';
            
            // Add header with sender and time
            const headerElement = document.createElement('div');
            const senderElement = document.createElement('span');
            senderElement.className = 'sender';
            senderElement.textContent = message.sender;
            
            const timeElement = document.createElement('span');
            timeElement.className = 'time';
            timeElement.textContent = ' ' + new Date(message.timestamp).toLocaleTimeString();
            
            headerElement.appendChild(senderElement);
            headerElement.appendChild(timeElement);
            
            // Add content with highlighted mentions
            const contentElement = document.createElement('div');
            contentElement.className = 'content';
            
            let content = message.content;
            
            // Highlight mentions
            if (message.mentions && message.mentions.length > 0) {
                message.mentions.forEach(mention => {
                    const pattern = new RegExp('@' + mention, 'g');
                    content = content.replace(pattern, '<span class="mention">@' + mention + '</span>');
                });
                
                // Highlight if current user is mentioned
                if (username && message.mentions.includes(username)) {
                    messageElement.style.backgroundColor = '#fffbeb';
                }
            }
            
            contentElement.innerHTML = content;
            
            // Add elements to message
            messageElement.appendChild(headerElement);
            messageElement.appendChild(contentElement);
            
            // Add message to container and scroll to bottom
            messagesContainer.appendChild(messageElement);
            messagesContainer.scrollTop = messagesContainer.scrollHeight;
        }
    })();
    </script>
</body>
</html>`;

// Define the HTML template for our home page
const homeHTML = `<html>
<head>
    <title>CRM Chat API</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        h1 { color: #333; }
        .nav { margin: 20px 0; }
        .nav a { display: inline-block; margin-right: 15px; padding: 10px; background: #f4f4f4; text-decoration: none; color: #333; }
        .nav a:hover { background: #e0e0e0; }
    </style>
</head>
<body>
    <h1>CRM Chat API</h1>
    <div class="nav">
        <a href="/chat-test">Chat Test Interface</a>
    </div>
    <p>Click the link above to interact with the chat API</p>
</body>
</html>`;

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// Create a new HTTP server mux
	mux := http.NewServeMux()

	// Root route
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(homeHTML))
	})

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
		w.Write([]byte(chatTestHTML))
	})

	// Create a new server
	server := &http.Server{
		Addr:    "0.0.0.0:" + port,
		Handler: mux,
	}

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on port %s...\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}