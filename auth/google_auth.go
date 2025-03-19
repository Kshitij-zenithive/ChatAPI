package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"crm-communication-api/models"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"gorm.io/gorm"
)

// GoogleUserInfo represents the structure of Google user info
type GoogleUserInfo struct {
	ID        string `json:"sub"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Picture   string `json:"picture"`
	Verified  bool   `json:"email_verified"`
}

// GoogleOAuthConfig holds OAuth configuration
type GoogleOAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// GoogleAuthService manages Google authentication
type GoogleAuthService struct {
	DB           *gorm.DB
	Logger       *logrus.Logger
	OAuthConfig  *oauth2.Config
	CookieStore  *sessions.CookieStore
}

// NewGoogleAuthService creates a new Google auth service
func NewGoogleAuthService(db *gorm.DB, logger *logrus.Logger) *GoogleAuthService {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	redirectURL := os.Getenv("GOOGLE_REDIRECT_URL")
	sessionSecret := os.Getenv("SESSION_SECRET")

	// Configure OAuth
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/gmail.readonly",
		},
		Endpoint: google.Endpoint,
	}

	// Configure session store
	store := sessions.NewCookieStore([]byte(sessionSecret))
	store.MaxAge(86400) // 1 day

	return &GoogleAuthService{
		DB:          db,
		Logger:      logger,
		OAuthConfig: config,
		CookieStore: store,
	}
}

// InitGothGoogle initializes Goth for Google OAuth
func (s *GoogleAuthService) InitGothGoogle() {
	gothic.Store = s.CookieStore

	provider := google.New(
		s.OAuthConfig.ClientID,
		s.OAuthConfig.ClientSecret,
		s.OAuthConfig.RedirectURL,
		"email", "profile", "https://www.googleapis.com/auth/gmail.readonly",
	)

	// Force refresh token by adding extra parameters
	provider.SetAccessType("offline")
	provider.SetPrompt("consent")
	
	goth.UseProviders(provider)
}

// GetGoogleAuthURL returns the Google OAuth authorization URL
func (s *GoogleAuthService) GetGoogleAuthURL(state string) string {
	return s.OAuthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// GetGoogleUserInfo fetches user info from Google API
func (s *GoogleAuthService) GetGoogleUserInfo(token *oauth2.Token) (*GoogleUserInfo, error) {
	client := s.OAuthConfig.Client(context.Background(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: status %d", resp.StatusCode)
	}

	var userInfo GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

// HandleGoogleCallback processes OAuth callback and creates/updates user
func (s *GoogleAuthService) HandleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	// Get the OAuth2 token from the callback
	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		s.Logger.WithError(err).Error("Failed to complete user auth")
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Log OAuth details
	s.Logger.WithFields(logrus.Fields{
		"email":         gothUser.Email,
		"has_refresh":   gothUser.RefreshToken != "",
		"provider":      gothUser.Provider,
		"provider_id":   gothUser.UserID,
	}).Info("User authenticated with Google")

	// Check if user exists by email
	var user models.User
	result := s.DB.Where("email = ?", gothUser.Email).First(&user)

	// If user does not exist, create a new one
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		user = models.User{
			ID:       uuid.New(),
			Email:    gothUser.Email,
			Name:     gothUser.Name,
			Avatar:   gothUser.AvatarURL,
			Role:     "user", // Default role for new users
			Password: "",     // No password for OAuth users
		}

		if err := s.DB.Create(&user).Error; err != nil {
			s.Logger.WithError(err).Error("Failed to create user")
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
	} else if result.Error != nil {
		s.Logger.WithError(result.Error).Error("Database error when checking user")
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Store/update OAuth provider details
	var oauthProvider models.OAuthProvider
	providerResult := s.DB.Where("user_id = ? AND provider = ?", user.ID, "google").First(&oauthProvider)

	if errors.Is(providerResult.Error, gorm.ErrRecordNotFound) {
		// Create new OAuth provider record
		oauthProvider = models.OAuthProvider{
			ID:           uuid.New(),
			UserID:       user.ID,
			Provider:     "google",
			ProviderID:   gothUser.UserID,
			AccessToken:  gothUser.AccessToken,
			RefreshToken: gothUser.RefreshToken,
			ExpiresAt:    time.Now().Add(time.Hour), // Approximate, should be from token info
		}
		if err := s.DB.Create(&oauthProvider).Error; err != nil {
			s.Logger.WithError(err).Error("Failed to create OAuth provider record")
		}
	} else if providerResult.Error == nil {
		// Update existing OAuth provider record
		oauthProvider.AccessToken = gothUser.AccessToken
		if gothUser.RefreshToken != "" {
			oauthProvider.RefreshToken = gothUser.RefreshToken
		}
		oauthProvider.ExpiresAt = time.Now().Add(time.Hour)
		if err := s.DB.Save(&oauthProvider).Error; err != nil {
			s.Logger.WithError(err).Error("Failed to update OAuth provider record")
		}
	}

	// Generate JWT tokens for our API
	accessToken, refreshToken, err := GenerateTokens(&user, "google")
	if err != nil {
		s.Logger.WithError(err).Error("Failed to generate tokens")
		http.Error(w, "Failed to generate authentication tokens", http.StatusInternalServerError)
		return
	}

	// Store refresh token
	if err := StoreRefreshToken(s.DB, user.ID.String(), refreshToken); err != nil {
		s.Logger.WithError(err).Error("Failed to store refresh token")
	}

	// In a real application, you would redirect to a frontend with the tokens
	// Here we're just returning JSON with the tokens
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user_id":       user.ID,
		"name":          user.Name,
		"email":         user.Email,
		"avatar":        user.Avatar,
		"role":          user.Role,
		"auth_provider": "google",
	})
}

// GetGmailClient creates a Gmail API client for a user
func (s *GoogleAuthService) GetGmailClient(userID uuid.UUID) (*http.Client, error) {
	var oauthProvider models.OAuthProvider
	if err := s.DB.Where("user_id = ? AND provider = ?", userID, "google").First(&oauthProvider).Error; err != nil {
		return nil, fmt.Errorf("no Google account linked: %w", err)
	}

	token := &oauth2.Token{
		AccessToken:  oauthProvider.AccessToken,
		RefreshToken: oauthProvider.RefreshToken,
		Expiry:       oauthProvider.ExpiresAt,
		TokenType:    "Bearer",
	}

	// This will automatically refresh the token if needed
	return s.OAuthConfig.Client(context.Background(), token), nil
}