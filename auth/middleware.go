package auth

import (
	"context"
	"crm-communication-api/database"
	"crm-communication-api/models"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Key for user claims in context
type contextKey string

const UserCtxKey contextKey = "user"

// Middleware handles JWT authentication
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for playground in development
		if r.URL.Path == "/playground" {
			next.ServeHTTP(w, r)
			return
		}

		// Read the request body to extract GraphQL operation
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		// Reset the body so it can be read again
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))

		// Try to parse the request as GraphQL
		var graphqlReq struct {
			OperationName string `json:"operationName"`
			Query         string `json:"query"`
		}
		if err := json.Unmarshal(bodyBytes, &graphqlReq); err == nil {
			// Allow login mutation without a token
			if strings.Contains(graphqlReq.Query, "login") ||
				(graphqlReq.OperationName != "" && strings.Contains(strings.ToLower(graphqlReq.OperationName), "login")) {
				log.Println("Login operation detected, skipping auth check")
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check the authorization header for all other requests
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: Missing token", http.StatusUnauthorized)
			return
		}

		// Extract the token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		
		// Validate the token
		claims, err := ValidateJWT(tokenString)
		if err != nil {
			// If token is expired, try to refresh
			if isTokenExpiredError(err) {
				claims, err = handleTokenRefresh(w, tokenString)
				if err != nil {
					http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
					return
				}
			} else {
				http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
				return
			}
		}

		// Set claims in context and proceed
		ctx := context.WithValue(r.Context(), UserCtxKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// handleTokenRefresh attempts to refresh an expired token
func handleTokenRefresh(w http.ResponseWriter, tokenString string) (*Claims, error) {
	// Parse token without validation to extract claims
	parsedToken, _, err := new(jwt.Parser).ParseUnverified(tokenString, &Claims{})
	if err != nil {
		return nil, errors.New("unable to parse expired token")
	}

	// Extract user ID from parsed token
	claims, ok := parsedToken.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, errors.New("invalid user ID in token")
	}

	// Get user from database
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, errors.New("user not found")
	}

	// Generate new access token
	accessExpiry, _ := strconv.Atoi(getEnvOrDefault("JWT_EXPIRY_TIME", "15")) // Default: 15 minutes
	newToken, err := GenerateJWT(&user, claims.AuthProvider, accessExpiry)
	if err != nil {
		return nil, errors.New("failed to generate new token")
	}

	// Set the new token in response header
	w.Header().Set("New-Access-Token", newToken)

	// Get claims from new token
	newClaims, _ := ValidateJWT(newToken)
	return newClaims, nil
}

// isTokenExpiredError checks if the error is due to an expired token
func isTokenExpiredError(err error) bool {
	return strings.Contains(err.Error(), "token is expired")
}

// GetUserFromContext retrieves the user claims from context
func GetUserFromContext(ctx context.Context) (*Claims, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	claims, ok := ctx.Value(UserCtxKey).(*Claims)
	if !ok || claims == nil {
		return nil, errors.New("no user found in context")
	}
	return claims, nil
}