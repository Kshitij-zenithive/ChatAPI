package googleAuth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	initializers "crm-api/Initializers"
	"crm-api/auth"
	"crm-api/models"

	"github.com/google/uuid"
	"github.com/gorilla/sessions"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
	"gorm.io/gorm"
)

// GoogleResponse represents the structure of Google user info
type GoogleResponse struct {
	ID      string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
	token   string `json:"token"`
}

func InitGoogleStore() {
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		log.Fatal("SESSION_SECRET is not set")
	}
	var store *sessions.CookieStore

	fmt.Println("Session Key:", sessionSecret)

	store = sessions.NewCookieStore([]byte(sessionSecret))
	store.MaxAge(86400) // 1 day
	gothic.Store = store

	fmt.Println("GOOGLE_CLIENT_ID", os.Getenv("GOOGLE_CLIENT_ID"))
	fmt.Println("GOOGLE_CLIENT_SECRET", os.Getenv("GOOGLE_CLIENT_SECRET"))
	fmt.Println("GOOGLE_REDIRECT_URL", os.Getenv("GOOGLE_REDIRECT_URL"))
	provider := google.New(
		os.Getenv("GOOGLE_CLIENT_ID"),
		os.Getenv("GOOGLE_CLIENT_SECRET"),
		os.Getenv("GOOGLE_REDIRECT_URL"),
		"https://www.googleapis.com/auth/calendar.events.readonly",
		"email",
		"profile",
	)

	// Force refresh token by adding extra auth parameters
	provider.SetAccessType("offline") // Ensures refresh token is received

	goth.UseProviders(provider)

}

// VerifyGoogleToken decodes and verifies the Google token
func VerifyGoogleToken(token string) (*GoogleResponse, error) {
	url := "https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + token
	resp, err := http.Get(url)
	if err != nil {
		return nil, errors.New("failed to verify Google token")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid Google token")
	}

	var googleData GoogleResponse
	if err := json.NewDecoder(resp.Body).Decode(&googleData); err != nil {
		return nil, errors.New("failed to parse Google response")
	}

	return &googleData, nil
}

func OauthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// Get user from Gothic
	log.Println("OAuth callback reached")
	gothUser, err := gothic.CompleteUserAuth(w, r)

	if err != nil {
		log.Println("OAuth error:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("User authenticated:", gothUser.Email)
	log.Println("Access Token:", gothUser.AccessToken)
	log.Println("Refresh Token:", gothUser.RefreshToken) // ✅ Log Refresh Token
	if gothUser.RefreshToken == "" {
		log.Println("⚠️ No refresh token received. Make sure user re-consents.")
	}
	// Check if user exists by email
	var user models.UserDemo
	result := initializers.DB.Where("email = ?", gothUser.Email).First(&user)

	if result.Error == gorm.ErrRecordNotFound {
		// Create new user
		log.Println("User not found, creating new user")
		log.Println("gothUser.token =", gothUser.RefreshToken)
		user = models.UserDemo{
			ID:                  uuid.New(),
			Email:               gothUser.Email,
			Name:                gothUser.Name,
			Password:            "", // No password for Google login
			GoogleId:            gothUser.UserID,
			GoogleRefreshToken:  gothUser.RefreshToken,
			GoogleAccessToken:   gothUser.AccessToken,
			Provider:            gothUser.Provider,
			BackendRefreshToken: "",
			BackendTokenExpiry:  time.Time{},       //left to set expiration time
			Role:                "SALES_EXECUTIVE", // Default role
		}

		result = initializers.DB.Create(&user)
		if result.Error != nil {
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}
	} else if result.Error != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	log.Println("User :", &user)
	_, refreshToken, err := auth.GenerateTokensDemo(&user, "Google")
	if err != nil {
		http.Error(w, "Failed to generate tokens", http.StatusInternalServerError)
		return
	}

	// Store refresh token in DB
	user.BackendRefreshToken = refreshToken
	initializers.DB.Save(&user)

	// Redirect with tokens or return JSON
	// For example, redirect to frontend with tokens as query parameters
	http.Redirect(w, r, fmt.Sprintf("http://localhost:3000/oauth-success?access_token=%s&refresh_token=%s&user_id=%s&auth_provider=%s",
		gothUser.AccessToken, refreshToken, user.ID.String(), user.Provider), http.StatusTemporaryRedirect)
}
