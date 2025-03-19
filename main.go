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
        "sync"
        "syscall"
        "time"

        "github.com/google/uuid"
        "github.com/gorilla/websocket"
        
        "crm-communication-api/database"
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

// Parse mentions from message content
func parseMentions(content string) []string {
        words := strings.Fields(content)
        mentions := []string{}
        
        for _, word := range words {
                if strings.HasPrefix(word, "@") && len(word) > 1 {
                        // Remove any non-alphanumeric characters from the end of the mention
                        mention := word[1:]
                        mention = strings.TrimFunc(mention, func(r rune) bool {
                                return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
                                           (r >= '0' && r <= '9') || r == '_' || r == '.')
                        })
                        
                        if mention != "" {
                                mentions = append(mentions, mention)
                        }
                }
        }
        
        return mentions
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

                // Store message in database and create timeline event
                go storeMessageInDatabase(message, c.username, mentions)
                
                // Auto-respond to mentions for demo purposes
                if len(mentions) > 0 {
                    go autoRespondToMentions(c.hub, mentions, c.username, message.ID)
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

// Define the HTML template for our chat test page
const chatTestHTML = `<!DOCTYPE html>
<html>
<head>
    <title>CRM Chat Test</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 1000px; margin: 0 auto; padding: 20px; }
        .container { display: flex; gap: 20px; }
        .chat-area { flex-grow: 1; }
        .sidebar { width: 200px; }
        #messages { height: 400px; overflow-y: auto; border: 1px solid #ccc; margin-bottom: 10px; padding: 10px; }
        #messageForm { display: flex; position: relative; }
        #messageInput { flex-grow: 1; padding: 10px; font-size: 14px; }
        button { padding: 5px 15px; background: #4CAF50; color: white; border: none; cursor: pointer; }
        .message { margin-bottom: 15px; padding-bottom: 15px; border-bottom: 1px solid #eee; }
        .message .sender { font-weight: bold; }
        .message .time { color: #999; font-size: 12px; }
        .message .content { margin-top: 5px; }
        .mention { background-color: #e6f7ff; padding: 2px 4px; border-radius: 2px; font-weight: bold; }
        #status { margin-bottom: 10px; color: #999; }
        .user-list { border: 1px solid #ccc; padding: 10px; margin-bottom: 10px; }
        .user-list h3 { margin-top: 0; margin-bottom: 10px; }
        .user-list ul { list-style: none; padding: 0; margin: 0; }
        .user-list li { padding: 5px 0; cursor: pointer; }
        .user-list li:hover { background-color: #f9f9f9; }
        .instructions { background-color: #f8f9fa; padding: 15px; border-radius: 4px; margin-bottom: 20px; }
        .instructions h3 { margin-top: 0; }
        .instructions ul { padding-left: 20px; }
        .mentions-dropdown {
            position: absolute;
            bottom: 100%;
            left: 0;
            background: white;
            border: 1px solid #ccc;
            box-shadow: 0 2px 8px rgba(0,0,0,0.1);
            max-height: 150px;
            overflow-y: auto;
            width: 200px;
            display: none;
            z-index: 100;
        }
        .mentions-dropdown ul {
            list-style: none;
            padding: 0;
            margin: 0;
        }
        .mentions-dropdown li {
            padding: 8px 10px;
            border-bottom: 1px solid #eee;
            cursor: pointer;
        }
        .mentions-dropdown li:hover {
            background-color: #f0f7ff;
        }
        .mentions-dropdown.visible { 
            display: block; 
        }
    </style>
</head>
<body>
    <h1>CRM Chat API Test</h1>
    
    <div class="instructions">
        <h3>Testing Instructions</h3>
        <ul>
            <li>Enter your username and click "Connect" to join the chat</li>
            <li>Type <strong>@</strong> followed by a name to mention someone (e.g., @John)</li>
            <li>When typing @, a dropdown with suggested users will appear</li>
            <li>Click on a user in the sidebar to mention them automatically</li>
            <li>System will notify when users are mentioned in messages</li>
        </ul>
    </div>
    
    <div>
        <label for="username">Username:</label>
        <input type="text" id="username" value="TestUser" />
        <button onclick="connect()">Connect</button>
    </div>
    
    <div id="status">Disconnected</div>
    
    <div class="container">
        <div class="chat-area">
            <div id="messages"></div>
            
            <form id="messageForm" onsubmit="sendMessage(event)">
                <div id="mentionsDropdown" class="mentions-dropdown">
                    <ul id="mentionsList"></ul>
                </div>
                <input type="text" id="messageInput" placeholder="Type a message... (Use @ to mention someone)" oninput="handleInput(event)" onkeydown="handleKeyDown(event)" />
                <button type="submit">Send</button>
            </form>
        </div>
        
        <div class="sidebar">
            <div class="user-list">
                <h3>Users</h3>
                <ul id="userList">
                    <li onclick="mentionUser('Admin')">Admin (System)</li>
                    <li onclick="mentionUser('John')">John (Sales)</li>
                    <li onclick="mentionUser('Maria')">Maria (Support)</li>
                    <li onclick="mentionUser('Carlos')">Carlos (Dev)</li>
                    <li onclick="mentionUser('Sarah')">Sarah (Marketing)</li>
                </ul>
            </div>
            
            <div class="user-list">
                <h3>Clients</h3>
                <ul id="clientList">
                    <li onclick="mentionUser('TestClient')">Test Client</li>
                    <li onclick="mentionUser('Acme')">Acme Corp</li>
                    <li onclick="mentionUser('Globex')">Globex Inc</li>
                </ul>
            </div>
        </div>
    </div>
    
    <script>
        let socket;
        let username = '';
        
        // Predefined list of users for testing mentions
        const suggestedUsers = [
            'Admin', 'John', 'Maria', 'Carlos', 'Sarah', 
            'TestClient', 'Acme', 'Globex', 'System'
        ];
        
        function connect() {
            username = document.getElementById('username').value;
            if (!username) {
                alert('Please enter a username');
                return;
            }
            
            // Add current user to the list if not already there
            if (!suggestedUsers.includes(username)) {
                suggestedUsers.push(username);
            }
            
            const statusEl = document.getElementById('status');
            statusEl.textContent = 'Connecting...';
            
            // Create WebSocket connection
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws/chat?username=' + encodeURIComponent(username);
            socket = new WebSocket(wsUrl);
            
            // Connection opened
            socket.addEventListener('open', function (event) {
                statusEl.textContent = 'Connected as ' + username;
                statusEl.style.color = '#4CAF50';
                
                // Add the current user to the list
                if (!document.querySelector('#userList li[data-username="' + username + '"]')) {
                    const userList = document.getElementById('userList');
                    const userItem = document.createElement('li');
                    userItem.setAttribute('data-username', username);
                    userItem.textContent = username + ' (You)';
                    userItem.onclick = function() { mentionUser(username); };
                    userList.appendChild(userItem);
                }
            });
            
            // Listen for messages
            socket.addEventListener('message', function (event) {
                const message = JSON.parse(event.data);
                displayMessage(message);
                
                // Auto-respond to mentions of the current user
                if (message.mentions && message.mentions.includes(username) && message.sender !== username) {
                    setTimeout(() => {
                        const response = {
                            content: '@' + message.sender + ' Thanks for mentioning me! I will get back to you soon.'
                        };
                        socket.send(JSON.stringify(response));
                    }, 1000);
                }
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
            });
        }
        
        function sendMessage(event) {
            event.preventDefault();
            if (!socket || socket.readyState !== WebSocket.OPEN) {
                alert('Please connect first');
                return;
            }
            
            const inputEl = document.getElementById('messageInput');
            const content = inputEl.value.trim();
            
            if (content) {
                socket.send(JSON.stringify({ content: content }));
                inputEl.value = '';
                hideMentionsDropdown();
            }
        }
        
        function displayMessage(message) {
            const messagesEl = document.getElementById('messages');
            
            const messageDiv = document.createElement('div');
            messageDiv.className = 'message';
            
            const headerDiv = document.createElement('div');
            const senderSpan = document.createElement('span');
            senderSpan.className = 'sender';
            senderSpan.textContent = message.sender;
            
            const timeSpan = document.createElement('span');
            timeSpan.className = 'time';
            timeSpan.textContent = ' ' + new Date(message.timestamp).toLocaleTimeString();
            
            headerDiv.appendChild(senderSpan);
            headerDiv.appendChild(timeSpan);
            
            const contentDiv = document.createElement('div');
            contentDiv.className = 'content';
            
            // Highlight mentions in content
            let content = message.content;
            if (message.mentions && message.mentions.length > 0) {
                message.mentions.forEach(mention => {
                    const regex = new RegExp('@' + mention, 'g');
                    content = content.replace(regex, '<span class="mention">@' + mention + '</span>');
                });
                
                // If the current user is mentioned, highlight the message
                if (username && message.mentions.includes(username)) {
                    messageDiv.style.backgroundColor = '#fffbeb';
                    
                    // Also play a notification sound (if the sender is not the current user)
                    if (message.sender !== username) {
                        // Create a simple beep sound
                        const beep = new Audio("data:audio/wav;base64,//uQRAAAAWMSLwUIYAAsYkXgoQwAEaYLWfkWgAI0wWs/ItAAAGDgYtAgAyN+QWaAAihwMWm4G8QQRDiMcCBcH3Cc+CDv/7xA4Tvh9Rz/y8QADBwMWgQAZG/ILNAARQ4GLTcDeIIIhxGOBAuD7hOfBB3/94gcJ3w+o5/5eIAIAAAVwWgQAVQ2ORaIQwEMAJiDg95G4nQL7mQVWI6GwRcfsZAcsKkJvxgxEjzFUgfHoSQ9Qq7KNwqHwuB13MA4a1q/DmBrHgPcmjiGoh//EwC5nGPEmS4RcfkVKOhJf+WOgoxJclFz3kgn//dBA+ya1GhurNn8zb//9NNutNuhz31f////9vt///z+IdAEAAAK4LQIAKobHItEIYCGAExBwe8jcToF9zIKrEdDYIuP2MgOWFSE34wYiR5iqQPj0JIeoVdlG4VD4XA67mAcNa1fhzA1jwHuTRxDUQ//iYBczjHiTJcIuPyKlHQkv/LHQUYkuSi57yQT//uggfZNajQ3Vm//3i4//MP/vm//Hv00ULIOQAAAAwkCGw8a3Gw+QLX+P0IvNOcoQAAMAQAAAKFQgLIQAzHV/5hAHQxc/BmgCw0oFIAiAk4fDZwEmMZCPhsuAYPsT8CUMO3NlPAMENjSxfAUFwbE8kHcR9Ae2OhQ8jDQ0l/EbwB62wNxFNxgoDxCyDEe1CWBXMQ8Kgg2lKoVaFKB1wXmtTn9II+PF01rvP9ySPTr1gU33WwoCXdlPpG9RDo9TfBl24R4QEXRQ8IcQSFiQBMgYZV81o17jcL4FVeqYXJe7fmYOBe0msC9dQ4p8bADn3dS5YgxG4veTJ6mTtdNIqwfvnpZ5wK/sn0wW5kLXZY2t7mK6xoKYxvsIlV5kf/WuoYJG6a6m24PVL9pJyFfSdXnNi5ElIkr5K8PqIrz3FyJo69u+4gkVnkuS+1JQy/2sIr4GxPXPMjEYZyC1KrA8X8JBSk7XjcxPqxCJRiGj9MFBiM82JeMgLNj5EDWGQ4ZRpXLKo+4rKU1WFw+eEwB7FIwHT3MbQzdgBJpMlEXQFaItktUZvoNYVIgP9hwMH30UQZ2XnzP7ZDbytNEuH5t5fIL9+t7wU5/+nz+YPzRRkc7/N6sn6T97fOz74Z/+n38WUAAA");
                        beep.play();
                    }
                }
            }
            
            contentDiv.innerHTML = content;
            
            messageDiv.appendChild(headerDiv);
            messageDiv.appendChild(contentDiv);
            
            messagesEl.appendChild(messageDiv);
            messagesEl.scrollTop = messagesEl.scrollHeight;
        }
        
        // Handle input for @mentions autocomplete
        function handleInput(event) {
            const input = event.target;
            const text = input.value;
            const cursorPos = input.selectionStart;
            
            // Find the word the cursor is in
            const wordStart = text.lastIndexOf('@', cursorPos);
            
            if (wordStart !== -1 && wordStart + 1 <= cursorPos && (wordStart === 0 || text[wordStart - 1] === ' ')) {
                const word = text.substring(wordStart + 1, cursorPos).toLowerCase();
                const matches = suggestedUsers.filter(user => 
                    user.toLowerCase().startsWith(word)
                );
                
                if (matches.length > 0) {
                    showMentionsDropdown(matches, wordStart);
                } else {
                    hideMentionsDropdown();
                }
            } else {
                hideMentionsDropdown();
            }
        }
        
        // Handle keyboard navigation in the mentions dropdown
        function handleKeyDown(event) {
            const dropdown = document.getElementById('mentionsDropdown');
            if (!dropdown.classList.contains('visible')) return;
            
            const items = dropdown.querySelectorAll('li');
            const selectedItem = dropdown.querySelector('li.selected');
            let selectedIndex = -1;
            
            if (selectedItem) {
                selectedIndex = Array.from(items).indexOf(selectedItem);
            }
            
            switch (event.key) {
                case 'ArrowDown':
                    event.preventDefault();
                    if (selectedIndex < items.length - 1) {
                        if (selectedItem) selectedItem.classList.remove('selected');
                        items[selectedIndex + 1].classList.add('selected');
                    }
                    break;
                    
                case 'ArrowUp':
                    event.preventDefault();
                    if (selectedIndex > 0) {
                        if (selectedItem) selectedItem.classList.remove('selected');
                        items[selectedIndex - 1].classList.add('selected');
                    }
                    break;
                    
                case 'Enter':
                    if (selectedItem) {
                        event.preventDefault();
                        insertMention(selectedItem.textContent);
                    }
                    break;
                    
                case 'Escape':
                    hideMentionsDropdown();
                    break;
            }
        }
        
        // Show the mentions dropdown with filtered results
        function showMentionsDropdown(matches, wordStart) {
            const dropdown = document.getElementById('mentionsDropdown');
            const list = document.getElementById('mentionsList');
            list.innerHTML = '';
            
            matches.forEach(match => {
                const item = document.createElement('li');
                item.textContent = match;
                item.onclick = function() {
                    insertMention(match);
                };
                list.appendChild(item);
            });
            
            dropdown.classList.add('visible');
        }
        
        // Hide the mentions dropdown
        function hideMentionsDropdown() {
            document.getElementById('mentionsDropdown').classList.remove('visible');
        }
        
        // Insert a mention at the current @ position
        function insertMention(username) {
            const input = document.getElementById('messageInput');
            const text = input.value;
            const cursorPos = input.selectionStart;
            
            // Find the @ that started this mention
            const wordStart = text.lastIndexOf('@', cursorPos);
            
            if (wordStart !== -1) {
                // Replace the partial @mention with the full username
                const before = text.substring(0, wordStart);
                const after = text.substring(cursorPos);
                
                input.value = before + '@' + username + ' ' + after;
                input.focus();
                input.selectionStart = input.selectionEnd = wordStart + username.length + 2; // +2 for @ and space
            }
            
            hideMentionsDropdown();
        }
        
        // Function to mention a user by clicking on their name in the sidebar
        function mentionUser(username) {
            const input = document.getElementById('messageInput');
            
            // Add the mention at the end or at cursor position
            const cursorPos = input.selectionStart;
            const text = input.value;
            
            // If there's already text, add a space before the mention if needed
            const prefix = cursorPos > 0 && text[cursorPos - 1] !== ' ' && text.length > 0 ? ' ' : '';
            const mention = prefix + '@' + username + ' ';
            
            // Insert at cursor position
            const before = text.substring(0, cursorPos);
            const after = text.substring(cursorPos);
            input.value = before + mention + after;
            
            // Set cursor position after the inserted mention
            input.focus();
            input.selectionStart = input.selectionEnd = cursorPos + mention.length;
        }
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

// min returns the smaller of a and b
func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// parseMentions extracts @username mentions from a message
func parseMentions(content string) []string {
    mentionsMap := make(map[string]bool)
    words := strings.Fields(content)
    
    for _, word := range words {
        if strings.HasPrefix(word, "@") {
            mention := strings.TrimPrefix(word, "@")
            // Remove any trailing punctuation
            mention = strings.TrimRight(mention, ",.!?:;")
            if mention != "" {
                mentionsMap[mention] = true
            }
        }
    }
    
    // Convert map to slice
    mentions := make([]string, 0, len(mentionsMap))
    for mention := range mentionsMap {
        mentions = append(mentions, mention)
    }
    
    return mentions
}

// autoRespondToMentions creates automatic responses when users are mentioned
func autoRespondToMentions(hub *ChatHub, mentions []string, sender string, replyToID string) {
    // Wait a moment before responding
    time.Sleep(1500 * time.Millisecond)
    
    // Define some predefined responses by username
    responses := map[string]string{
        "John":       "I'll review the sales data and get back to you shortly.",
        "Maria":      "Thanks for the mention. I'll help address this support request.",
        "Carlos":     "I'll check the technical issues you've reported.",
        "Sarah":      "I'll include this in our next marketing campaign.",
        "Admin":      "This has been noted by the admin team.",
        "TestClient": "Thank you for reaching out. As a client, I appreciate your attention.",
        "Acme":       "Acme Corp acknowledges your message.",
        "Globex":     "Globex Inc will respond to your inquiry soon.",
    }
    
    // Create default response for users not in the map
    defaultResponse := "Thanks for the mention. I'll get back to you soon."
    
    // Send a response for each mentioned user
    for _, mention := range mentions {
        responseText := responses[mention]
        if responseText == "" {
            responseText = defaultResponse
        }
        
        // Don't respond to the sender mentioning themselves
        if mention == sender {
            continue
        }
        
        // Create a response message
        responseMsg := ChatMessage{
            ID:        uuid.New().String(),
            Sender:    mention,
            Content:   fmt.Sprintf("@%s %s", sender, responseText),
            Mentions:  []string{sender},
            Timestamp: time.Now(),
        }
        
        // Broadcast the response
        hub.broadcast <- responseMsg
        
        // Store the response in the database
        go storeMessageInDatabase(responseMsg, mention, []string{sender})
    }
}

// storeMessageInDatabase stores the message in the database and creates timeline events
func storeMessageInDatabase(message ChatMessage, senderUsername string, mentions []string) {
    defer func() {
        // Recover from any panics to prevent crashing the whole application
        if r := recover(); r != nil {
            log.Printf("Recovered from database error: %v", r)
        }
    }()

    // Get user ID or create a user if not exists
    var user database.User
    result := database.DB.Where("username = ?", senderUsername).First(&user)
    if result.Error != nil {
        // Create a new user
        user = database.User{
            Username: senderUsername,
            Email:    senderUsername + "@example.com", // Placeholder email
        }
        database.DB.Create(&user)
    }

    // Create the message record
    dbMessage := database.Message{
        SenderID: user.ID,
        Content:  message.Content,
    }
    
    // Convert mentions to JSON string
    if len(mentions) > 0 {
        mentionsJSON, _ := json.Marshal(mentions)
        dbMessage.Mentions = string(mentionsJSON)
    }
    
    // Save message to database
    result = database.DB.Create(&dbMessage)
    if result.Error != nil {
        log.Printf("Error storing message: %v", result.Error)
        return
    }
    
    // Create timeline events for the message
    log.Printf("Timeline Event: User %s sent a message", senderUsername)
    
    // In a real app, we would create timeline events for each client mentioned
    // For now, we'll create a generic timeline event without a specific client
    // In production, we'd need to determine which clients were mentioned and create events for each
    
    // Let's create a simple timeline event
    timelineDetails, _ := json.Marshal(map[string]interface{}{
        "message_id": dbMessage.ID,
        "sender": senderUsername,
        "content_preview": message.Content[:min(50, len(message.Content))],
        "has_mentions": len(mentions) > 0,
    })
    
    // Find first client (for demo purposes only)
    var client database.Client
    clientResult := database.DB.First(&client)
    
    // Only create timeline event if we have a client
    if clientResult.Error == nil {
        timelineEvent := database.TimelineEvent{
            ClientID:  client.ID,
            EventType: "message",
            Details:   string(timelineDetails),
        }
        database.DB.Create(&timelineEvent)
        
        if len(mentions) > 0 {
            log.Printf("Timeline Event: User %s mentioned users: %v", senderUsername, mentions)
        }
    }
}

// simulateTwoUserChat creates a simulated chat between two virtual users
func simulateTwoUserChat(hub *ChatHub) {
    log.Println("Starting automated chat simulation between two virtual users...")
    
    // Create two virtual users
    user1 := &VirtualUser{
        ID:       uuid.New().String(),
        Username: "SimBot1",
        hub:      hub,
    }
    
    user2 := &VirtualUser{
        ID:       uuid.New().String(),
        Username: "SimBot2",
        hub:      hub,
    }
    
    // Connect the virtual users
    user1.Connect()
    user2.Connect()
    
    // Create a channel for coordination
    done := make(chan bool)
    
    // Start a conversation
    go func() {
        // Wait a bit before starting the conversation
        time.Sleep(5 * time.Second)
        
        // User 1 sends a greeting
        user1.SendMessage("Hello @SimBot2, this is an automated conversation demonstration!")
        time.Sleep(3 * time.Second)
        
        // User 2 responds
        user2.SendMessage("Hi @SimBot1, thanks for your message. This shows how we can simulate users chatting!")
        time.Sleep(4 * time.Second)
        
        // User 1 mentions a client
        user1.SendMessage("I need to discuss the @Acme Corp account with you. Can we schedule a meeting?")
        time.Sleep(3 * time.Second)
        
        // User 2 replies with another mention
        user2.SendMessage("Sure @SimBot1, let's involve @Carlos from the dev team as well since there are technical questions.")
        time.Sleep(4 * time.Second)
        
        // User 1 confirms
        user1.SendMessage("Great idea to include @Carlos. I'll send a calendar invite for tomorrow.")
        time.Sleep(3 * time.Second)
        
        // Log that the simulation is complete but keep the users connected
        log.Println("Chat simulation completed. Virtual users remain connected.")
    }()
    
    // Keep the simulation running
    <-done
}

// VirtualUser represents a simulated user for testing
type VirtualUser struct {
    ID       string
    Username string
    hub      *ChatHub
    client   *ChatClient
}

// Connect registers the virtual user with the hub
func (vu *VirtualUser) Connect() {
    // Create a virtual client for this user
    vu.client = &ChatClient{
        hub:      vu.hub,
        userID:   vu.ID,
        username: vu.Username,
        send:     make(chan ChatMessage, 256),
    }
    
    // Register with the hub
    vu.hub.register <- vu.client
    
    // Start a goroutine to handle received messages
    go func() {
        for message := range vu.client.send {
            log.Printf("[%s received]: %s: %s", vu.Username, message.Sender, message.Content)
        }
    }()
    
    log.Printf("Virtual user %s connected", vu.Username)
}

// SendMessage sends a message from this virtual user
func (vu *VirtualUser) SendMessage(content string) {
    // Create a unique message ID
    messageID := uuid.New().String()
    
    // Parse mentions in the message
    mentions := parseMentions(content)
    
    // Create the message
    message := ChatMessage{
        ID:        messageID,
        Sender:    vu.Username,
        Content:   content,
        Mentions:  mentions,
        Timestamp: time.Now(),
    }
    
    // Send to the hub
    vu.hub.broadcast <- message
    
    // Store in database
    go storeMessageInDatabase(message, vu.Username, mentions)
    
    log.Printf("[%s sent]: %s", vu.Username, content)
}

// createTestData adds some test data to the database if needed
func createTestData() {
    // Create a test client if none exist
    var count int64
    database.DB.Model(&database.Client{}).Count(&count)
    
    if count == 0 {
        testClient := database.Client{
            Name:  "Test Client",
            Email: "test.client@example.com",
            Phone: "555-123-4567",
        }
        
        result := database.DB.Create(&testClient)
        if result.Error != nil {
            log.Printf("Error creating test client: %v", result.Error)
        } else {
            log.Println("Created test client for demo purposes")
        }
    }
}

func main() {
        // Initialize database
        database.InitDB()
        
        // Create test data
        createTestData()
        
        port := os.Getenv("PORT")
        if port == "" {
                port = defaultPort
        }

        // Create a new HTTP server mux
        mux := http.NewServeMux()

        // Root route for checking if server is running
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
        
        // Start the automated chat simulation
        go simulateTwoUserChat(hub)

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