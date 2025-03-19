package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}

	// Create a simple HTTP server
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>CRM Chat API Demo</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
        pre { background: #f4f4f4; padding: 10px; border-radius: 5px; }
    </style>
</head>
<body>
    <h1>CRM Chat API Demo</h1>
    <p>This is a demonstration of the CRM Communication API. The full version includes:</p>
    <ul>
        <li>WebSocket-based real-time chat</li>
        <li>@mention functionality with user suggestions</li>
        <li>Message storage in PostgreSQL database</li>
        <li>Timeline tracking for client interactions</li>
        <li>JWT authentication and Google OAuth support</li>
    </ul>
    
    <h2>API Structure</h2>
    <pre>
POST /api/auth/login       # Login with username/password
POST /api/auth/google      # Login with Google OAuth
GET  /api/messages         # Get message history
POST /api/messages         # Send a new message
GET  /api/users            # Get list of users for mentions
GET  /api/clients          # Get list of clients for mentions
GET  /api/timeline/:client # Get timeline for a specific client
    </pre>
    
    <h2>Message Format</h2>
    <pre>
{
  "id": "unique-message-id",
  "sender": "username",
  "content": "Message content with @mentions",
  "mentions": ["mentioned_user1", "mentioned_user2"],
  "timestamp": "2025-03-19T12:34:56Z"
}
    </pre>
    
    <h2>@mention Functionality</h2>
    <p>When a user is @mentioned in a message:</p>
    <ol>
        <li>The mention is highlighted in the UI</li>
        <li>A notification is sent to the mentioned user</li>
        <li>The mention is logged in the client's timeline</li>
        <li>An auto-response is generated (for demo purposes)</li>
    </ol>
    
    <h2>Timeline Events</h2>
    <pre>
{
  "id": "timeline-event-id",
  "client_id": "client-id",
  "event_type": "message", 
  "details": {
    "message_id": "message-id",
    "sender": "username",
    "content_preview": "First 50 characters of message...",
    "has_mentions": true
  },
  "timestamp": "2025-03-19T12:34:56Z"
}
    </pre>
    
    <footer>
        <p>Note: Due to disk quota limitations, this static demonstration is shown instead of the full application.</p>
    </footer>
</body>
</html>
		`)
	})

	// Start the server
	log.Printf("Starting static CRM API demo server on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}