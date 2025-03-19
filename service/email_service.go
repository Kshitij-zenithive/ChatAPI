package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/your-org/crm-communication-api/database"
	"github.com/your-org/crm-communication-api/graph/model"
	"github.com/your-org/crm-communication-api/util"
)

// GmailConfig holds Gmail API configuration
type GmailConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
}

// EmailService handles email-related operations
type EmailService struct {
	db     *database.DB
	config GmailConfig
	logger *util.Logger
	
	// OAuth configuration
	oauthConfig *oauth2.Config
	
	// In-memory state for OAuth flow
	stateMutex sync.Mutex
	stateStore map[string]string // Maps state to user ID
	
	// Subscriptions
	emailMutex      sync.RWMutex
	emailSubscribers []chan *model.EmailInteraction
}

// NewEmailService creates a new email service
func NewEmailService(db *database.DB, config GmailConfig, logger *util.Logger) *EmailService {
	// Set default scopes if not provided
	if len(config.Scopes) == 0 {
		config.Scopes = []string{
			gmail.GmailSendScope,
			gmail.GmailReadonlyScope,
			gmail.GmailModifyScope,
		}
	}
	
	// Create OAuth2 config
	oauthConfig := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURL,
		Scopes:       config.Scopes,
		Endpoint:     google.Endpoint,
	}
	
	return &EmailService{
		db:               db,
		config:           config,
		logger:           logger,
		oauthConfig:      oauthConfig,
		stateStore:       make(map[string]string),
		emailSubscribers: make([]chan *model.EmailInteraction, 0),
	}
}

// GetAuthorizationURL generates a URL to authorize Gmail access
func (s *EmailService) GetAuthorizationURL(ctx context.Context, userID string) (string, error) {
	// Generate random state
	state := util.GenerateRandomString(32)
	
	// Store the state with the user ID
	s.stateMutex.Lock()
	s.stateStore[state] = userID
	s.stateMutex.Unlock()
	
	// Generate authorization URL
	authURL := s.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	
	return authURL, nil
}

// HandleOAuthCallback handles the OAuth callback from Gmail
func (s *EmailService) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// Get the state and code from the request
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	
	if state == "" || code == "" {
		http.Error(w, "Invalid state or code parameter", http.StatusBadRequest)
		return
	}
	
	// Verify state and get the user ID
	s.stateMutex.Lock()
	userID, exists := s.stateStore[state]
	delete(s.stateStore, state)
	s.stateMutex.Unlock()
	
	if !exists {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}
	
	// Complete the OAuth flow
	err := s.CompleteOAuth(r.Context(), userID, code)
	if err != nil {
		s.logger.Error("Failed to complete OAuth flow", "error", err, "userId", userID)
		http.Error(w, "Failed to complete authorization", http.StatusInternalServerError)
		return
	}
	
	// Redirect to success page
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><body><h1>Gmail Authorization Successful</h1><p>You can close this window and return to the CRM.</p></body></html>")
}

// CompleteOAuth completes the OAuth flow and stores the token
func (s *EmailService) CompleteOAuth(ctx context.Context, userID, code string) error {
	// Exchange the code for a token
	token, err := s.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to exchange code for token: %v", err)
	}
	
	// Serialize the token to JSON
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to serialize token: %v", err)
	}
	
	// Store the token in the database
	err = s.db.SaveGmailToken(ctx, userID, string(tokenJSON))
	if err != nil {
		return fmt.Errorf("failed to save token: %v", err)
	}
	
	s.logger.Info("Gmail OAuth flow completed successfully", "userId", userID)
	
	return nil
}

// GetGmailClient gets a Gmail client for a user
func (s *EmailService) GetGmailClient(ctx context.Context, userID string) (*gmail.Service, error) {
	// Get the token from the database
	tokenJSON, err := s.db.GetGmailToken(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Gmail token: %v", err)
	}
	
	// Parse the token
	var token oauth2.Token
	err = json.Unmarshal([]byte(tokenJSON), &token)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}
	
	// Create the OAuth client
	tokenSource := s.oauthConfig.TokenSource(ctx, &token)
	
	// Create the Gmail service
	gmailService, err := gmail.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("failed to create Gmail service: %v", err)
	}
	
	return gmailService, nil
}

// SendEmail sends an email to a client
func (s *EmailService) SendEmail(ctx context.Context, sender *model.User, client *model.Client, input model.EmailSendInput) (*model.EmailInteraction, error) {
	// Check if using a template
	var emailContent string
	var emailSubject string
	
	if input.TemplateID != nil {
		// Get the template
		template, err := s.db.GetEmailTemplate(ctx, *input.TemplateID)
		if err != nil {
			return nil, fmt.Errorf("failed to get email template: %v", err)
		}
		
		// Use template content and subject
		emailContent = template.Body
		emailSubject = template.Subject
		
		// Replace placeholders with client data
		emailContent = strings.Replace(emailContent, "{{client_name}}", client.Name, -1)
		emailContent = strings.Replace(emailContent, "{{client_email}}", client.Email, -1)
		if client.Company != nil {
			emailContent = strings.Replace(emailContent, "{{client_company}}", *client.Company, -1)
		}
		
		// Replace placeholders with sender data
		emailContent = strings.Replace(emailContent, "{{sender_name}}", sender.Name, -1)
		emailContent = strings.Replace(emailContent, "{{sender_email}}", sender.Email, -1)
	} else {
		// Use provided content and subject
		emailContent = input.Content
		emailSubject = input.Subject
	}
	
	// Get Gmail client
	gmailService, err := s.GetGmailClient(ctx, sender.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get Gmail client: %v", err)
	}
	
	// Create the email message
	messageStr := fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n\r\n"+
		"%s", sender.Name, sender.Email, client.Name, client.Email, emailSubject, emailContent)
	
	// Encode the message
	message := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString([]byte(messageStr)),
	}
	
	// Send the email
	message, err = gmailService.Users.Messages.Send("me", message).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %v", err)
	}
	
	// Create email interaction record
	emailInteraction := &model.EmailInteraction{
		Client:    client,
		User:      sender,
		Content:   emailContent,
		CreatedAt: time.Now(),
		Type:      model.InteractionTypeEmailSent,
		Subject:   emailSubject,
		EmailID:   message.Id,
		ThreadID:  &message.ThreadId,
	}
	
	// Save to database
	err = s.db.CreateEmailInteraction(ctx, emailInteraction)
	if err != nil {
		s.logger.Error("Failed to save email interaction", "error", err)
		// Don't return error here, as the email was already sent
	}
	
	// Broadcast to subscribers
	s.broadcastEmail(emailInteraction)
	
	return emailInteraction, nil
}

// GetEmailsForClient gets all email interactions for a client
func (s *EmailService) GetEmailsForClient(ctx context.Context, clientID string) ([]*model.EmailInteraction, error) {
	return s.db.GetEmailInteractionsForClient(ctx, clientID)
}

// StartEmailSyncWorker starts a background worker to sync emails
func (s *EmailService) StartEmailSyncWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			s.SyncEmails(ctx)
		}
	}
}

// SyncEmails synchronizes emails from Gmail
func (s *EmailService) SyncEmails(ctx context.Context) {
	// In a real implementation, you would:
	// 1. Get all users with Gmail OAuth tokens
	// 2. For each user, fetch recent emails
	// 3. For each email, check if it matches a client
	// 4. If it does, create an email interaction record
	
	s.logger.Info("Syncing emails from Gmail")
	
	// This is a mock implementation
	// In a real app, you would implement the full sync logic
}

// SubscribeToEmails subscribes to email updates
func (s *EmailService) SubscribeToEmails(ctx context.Context) <-chan *model.EmailInteraction {
	emailChan := make(chan *model.EmailInteraction, 1)
	
	go func() {
		<-ctx.Done()
		s.unsubscribeFromEmails(emailChan)
	}()
	
	s.subscribeToEmails(emailChan)
	
	return emailChan
}

// subscribeToEmails adds a subscription for emails
func (s *EmailService) subscribeToEmails(ch chan *model.EmailInteraction) {
	s.emailMutex.Lock()
	defer s.emailMutex.Unlock()
	
	s.emailSubscribers = append(s.emailSubscribers, ch)
}

// unsubscribeFromEmails removes a subscription for emails
func (s *EmailService) unsubscribeFromEmails(ch chan *model.EmailInteraction) {
	s.emailMutex.Lock()
	defer s.emailMutex.Unlock()
	
	var subscribers []chan *model.EmailInteraction
	for _, c := range s.emailSubscribers {
		if c != ch {
			subscribers = append(subscribers, c)
		}
	}
	
	s.emailSubscribers = subscribers
	close(ch)
}

// broadcastEmail broadcasts an email to all subscribers
func (s *EmailService) broadcastEmail(email *model.EmailInteraction) {
	s.emailMutex.RLock()
	defer s.emailMutex.RUnlock()
	
	for _, ch := range s.emailSubscribers {
		select {
		case ch <- email:
		default:
			// Channel buffer is full, skip
			s.logger.Warn("Skipped email broadcast - channel buffer full")
		}
	}
}
