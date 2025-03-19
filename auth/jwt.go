package auth

import (
	"crm-communication-api/models"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

// Secret keys for signing JWT tokens
var (
	AccessTokenSecretKey  = getEnvOrDefault("JWT_SECRET_KEY", "default_access_token_secret_key")
	RefreshTokenSecretKey = getEnvOrDefault("REFRESH_TOKEN_SECRET_KEY", "default_refresh_token_secret_key")
)

// Claims represents the JWT claims structure
type Claims struct {
	UserID       string `json:"user_id"`
	Name         string `json:"name"`
	Role         string `json:"role"`
	AuthProvider string `json:"auth_provider"`
	jwt.RegisteredClaims
}

// GenerateJWT creates a JWT token for a user
func GenerateJWT(user *models.User, authProvider string, expiryHours int) (string, error) {
	// Set expiration time
	expirationTime := time.Now().Add(time.Duration(expiryHours) * time.Hour)

	// Create JWT claims
	claims := &Claims{
		UserID:       user.ID.String(),
		Name:         user.Name,
		Role:         user.Role,
		AuthProvider: authProvider,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "crm-communication-api",
			Subject:   user.ID.String(),
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with secret key
	return token.SignedString([]byte(AccessTokenSecretKey))
}

// GenerateRefreshToken creates a refresh token for a user
func GenerateRefreshToken(user *models.User, authProvider string) (string, error) {
	// Set a longer expiration time for refresh token (e.g., 7 days)
	refreshExpiryHours, _ := strconv.Atoi(getEnvOrDefault("REFRESH_TOKEN_EXPIRY", "168")) // Default: 7 days
	
	// Create JWT claims with longer expiration
	expirationTime := time.Now().Add(time.Duration(refreshExpiryHours) * time.Hour)
	claims := &Claims{
		UserID:       user.ID.String(),
		Name:         user.Name,
		Role:         user.Role,
		AuthProvider: authProvider,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "crm-communication-api",
			Subject:   user.ID.String(),
		},
	}

	// Create and sign the refresh token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(RefreshTokenSecretKey))
}

// ValidateJWT validates a JWT token and returns the claims
func ValidateJWT(tokenString string) (*Claims, error) {
	// Parse the token with claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(AccessTokenSecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	// Validate the token and return claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ValidateRefreshToken validates a refresh token and returns the claims
func ValidateRefreshToken(tokenString string) (*Claims, error) {
	// Parse the refresh token with claims
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the alg is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(RefreshTokenSecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	// Validate the token and return claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid refresh token")
}

// GetUserIDFromToken extracts the user ID from a token
func GetUserIDFromToken(claims *Claims) (uuid.UUID, error) {
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil, errors.New("invalid user ID in token")
	}
	return userID, nil
}

// Helper function to get environment variable with default fallback
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}