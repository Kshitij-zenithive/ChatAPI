package graphql

import (
	"log"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/websocket"

	"crm-communication-api/auth"
	"crm-communication-api/internal/graphql/generated"
	"crm-communication-api/internal/graphql/resolvers"
)

// Configure the GraphQL handler with WebSocket support
func NewHandler() *handler.Server {
	// Create a new GraphQL handler
	srv := handler.New(generated.NewExecutableSchema(generated.Config{
		Resolvers: &resolvers.Resolver{},
	}))

	// Set up cors and WebSocket configuration
	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins in development
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	})

	// Add HTTP transports
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	// Enable Apollo GraphQL tracing in development mode
	srv.Use(extension.Introspection{})

	// Add query cache to improve performance
	srv.SetQueryCache(lru.New(1000))

	return srv
}

// RegisterRoutes sets up the GraphQL routes
func RegisterRoutes(mux *http.ServeMux) {
	// Create GraphQL handler with WebSocket support
	graphqlHandler := NewHandler()

	// Create the GraphQL playground handler
	playgroundHandler := playground.Handler("GraphQL Playground", "/graphql")

	// Apply authentication middleware to GraphQL endpoint
	authMiddleware := auth.Middleware()

	// Register routes
	mux.Handle("/playground", playgroundHandler)
	mux.Handle("/graphql", authMiddleware(graphqlHandler))

	// WebSocket specific endpoint for subscriptions
	mux.Handle("/ws", authMiddleware(graphqlHandler))

	log.Println("GraphQL endpoint registered at /graphql")
	log.Println("GraphQL playground registered at /playground")
	log.Println("WebSocket endpoint registered at /ws")
}