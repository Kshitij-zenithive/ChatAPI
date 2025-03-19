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

	// Define our routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		
		html := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>CRM Chat API - Minimal Version</title>
			<style>
				body { font-family: Arial, sans-serif; max-width: 800px; margin: 0 auto; padding: 20px; }
				h1 { color: #333; }
				.info { background-color: #f8f9fa; padding: 15px; border-radius: 4px; margin: 20px 0; }
				.feature { margin-bottom: 10px; }
				.feature-title { font-weight: bold; }
				code { background: #f0f0f0; padding: 2px 4px; border-radius: 3px; }
			</style>
		</head>
		<body>
			<h1>CRM Chat API - Minimal Version</h1>
			
			<div class="info">
				<p>This is a minimal version of the CRM Chat API to conserve disk space. The complete version includes:</p>
				
				<div class="feature">
					<div class="feature-title">✓ WebSocket-based real-time chat</div>
					<p>The full version includes bidirectional WebSocket communication for instant message delivery.</p>
				</div>
				
				<div class="feature">
					<div class="feature-title">✓ @mentions with user suggestions</div>
					<p>Users can type @ to mention others, with suggestions appearing as they type.</p>
				</div>
				
				<div class="feature">
					<div class="feature-title">✓ Timeline events</div>
					<p>All messages are stored in the database with associated timeline events for client history tracking.</p>
				</div>
				
				<div class="feature">
					<div class="feature-title">✓ Auto-responses</div>
					<p>The system automatically responds when users are mentioned, simulating real interactions.</p>
				</div>
			</div>
			
			<h2>API Documentation</h2>
			<p>The full CRM Communication API supports the following:</p>
			<ul>
				<li>WebSocket endpoint at <code>/ws/chat</code> for real-time messaging</li>
				<li>GraphQL API (planned) for message queries and mutations</li>
				<li>OAuth authentication for secure access</li>
				<li>Timeline event generation for client activity tracking</li>
			</ul>
			
			<h2>Status</h2>
			<p>This server is running in minimal mode. The full version is available in the codebase but not running to conserve disk space.</p>
		</body>
		</html>
		`
		
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, html)
	})

	// Add a status endpoint
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"running", "mode":"minimal"}`)
	})

	// Start the server
	serverAddr := "0.0.0.0:" + port
	log.Printf("Starting minimal server on %s...\n", serverAddr)
	err := http.ListenAndServe(serverAddr, nil)
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
}