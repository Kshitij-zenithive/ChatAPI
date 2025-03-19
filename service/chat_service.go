package service

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/your-org/crm-communication-api/database"
	"github.com/your-org/crm-communication-api/graph/model"
	"github.com/your-org/crm-communication-api/util"
)

// ChatService handles chat-related operations
type ChatService struct {
	db     *database.DB
	logger *util.Logger
	
	// Subscriptions
	clientMutex      sync.RWMutex
	clientSubscribers map[string][]chan *model.ChatMessage
	
	mentionMutex      sync.RWMutex
	mentionSubscribers map[string][]chan *model.ChatMessage
}

// NewChatService creates a new chat service
func NewChatService(db *database.DB, logger *util.Logger) *ChatService {
	return &ChatService{
		db:                 db,
		logger:             logger,
		clientSubscribers:  make(map[string][]chan *model.ChatMessage),
		mentionSubscribers: make(map[string][]chan *model.ChatMessage),
	}
}

// SendMessage sends a new chat message
func (s *ChatService) SendMessage(ctx context.Context, sender *model.User, input model.ChatMessageInput) (*model.ChatMessage, error) {
	// Get client
	client, err := s.db.GetClient(ctx, input.ClientID)
	if err != nil {
		s.logger.Error("Failed to get client for chat message", "error", err, "clientId", input.ClientID)
		return nil, err
	}
	
	// Extract mentions from content
	mentions, err := s.extractMentions(ctx, input.Content, input.Mentions)
	if err != nil {
		s.logger.Error("Failed to extract mentions", "error", err)
		return nil, err
	}
	
	// Create message
	message := &model.ChatMessage{
		Client:    client,
		User:      sender,
		Content:   input.Content,
		CreatedAt: time.Now(),
		Type:      model.InteractionTypeChatMessage,
		Mentions:  mentions,
	}
	
	// Save to database
	err = s.db.CreateChatMessage(ctx, message)
	if err != nil {
		s.logger.Error("Failed to create chat message", "error", err)
		return nil, err
	}
	
	return message, nil
}

// extractMentions extracts @mentions from message content and resolves them to users
func (s *ChatService) extractMentions(ctx context.Context, content string, mentionIDs []string) ([]*model.User, error) {
	var mentions []*model.User
	mentionMap := make(map[string]bool)
	
	// If explicit mention IDs are provided, use them
	if len(mentionIDs) > 0 {
		for _, id := range mentionIDs {
			if !mentionMap[id] { // Avoid duplicates
				user, err := s.db.GetUser(ctx, id)
				if err != nil {
					return nil, err
				}
				mentions = append(mentions, user)
				mentionMap[id] = true
			}
		}
	} else {
		// Extract @mentions from content
		re := regexp.MustCompile(`@(\w+)`)
		matches := re.FindAllStringSubmatch(content, -1)
		
		for _, match := range matches {
			if len(match) > 1 {
				username := match[1]
				// In a real implementation, you would look up users by username
				// For simplicity, we'll just log this
				s.logger.Info("Found mention", "username", username)
				// Example lookup (not implemented in the DB service)
				// user, err := s.db.GetUserByUsername(ctx, username)
				// if err == nil && !mentionMap[user.ID] {
				// 	mentions = append(mentions, user)
				// 	mentionMap[user.ID] = true
				// }
			}
		}
	}
	
	return mentions, nil
}

// GetMessagesForClient gets all chat messages for a client
func (s *ChatService) GetMessagesForClient(ctx context.Context, clientID string) ([]*model.ChatMessage, error) {
	return s.db.GetChatMessagesForClient(ctx, clientID)
}

// SubscribeToMessages subscribes to chat messages, optionally filtered by client ID
func (s *ChatService) SubscribeToMessages(ctx context.Context, clientID *string) <-chan *model.ChatMessage {
	msgChan := make(chan *model.ChatMessage, 1)
	
	go func() {
		<-ctx.Done()
		s.unsubscribeFromMessages(clientID, msgChan)
	}()
	
	s.subscribeToMessages(clientID, msgChan)
	
	return msgChan
}

// subscribeToMessages adds a subscription for chat messages
func (s *ChatService) subscribeToMessages(clientID *string, ch chan *model.ChatMessage) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	
	key := "all"
	if clientID != nil {
		key = *clientID
	}
	
	s.clientSubscribers[key] = append(s.clientSubscribers[key], ch)
}

// unsubscribeFromMessages removes a subscription for chat messages
func (s *ChatService) unsubscribeFromMessages(clientID *string, ch chan *model.ChatMessage) {
	s.clientMutex.Lock()
	defer s.clientMutex.Unlock()
	
	key := "all"
	if clientID != nil {
		key = *clientID
	}
	
	var channels []chan *model.ChatMessage
	for _, c := range s.clientSubscribers[key] {
		if c != ch {
			channels = append(channels, c)
		}
	}
	
	if len(channels) == 0 {
		delete(s.clientSubscribers, key)
	} else {
		s.clientSubscribers[key] = channels
	}
	
	close(ch)
}

// SubscribeToMentions subscribes to chat messages where the user is mentioned
func (s *ChatService) SubscribeToMentions(ctx context.Context, userID string) <-chan *model.ChatMessage {
	msgChan := make(chan *model.ChatMessage, 1)
	
	go func() {
		<-ctx.Done()
		s.unsubscribeFromMentions(userID, msgChan)
	}()
	
	s.subscribeToMentions(userID, msgChan)
	
	return msgChan
}

// subscribeToMentions adds a subscription for mentions
func (s *ChatService) subscribeToMentions(userID string, ch chan *model.ChatMessage) {
	s.mentionMutex.Lock()
	defer s.mentionMutex.Unlock()
	
	s.mentionSubscribers[userID] = append(s.mentionSubscribers[userID], ch)
}

// unsubscribeFromMentions removes a subscription for mentions
func (s *ChatService) unsubscribeFromMentions(userID string, ch chan *model.ChatMessage) {
	s.mentionMutex.Lock()
	defer s.mentionMutex.Unlock()
	
	var channels []chan *model.ChatMessage
	for _, c := range s.mentionSubscribers[userID] {
		if c != ch {
			channels = append(channels, c)
		}
	}
	
	if len(channels) == 0 {
		delete(s.mentionSubscribers, userID)
	} else {
		s.mentionSubscribers[userID] = channels
	}
	
	close(ch)
}

// BroadcastMessage broadcasts a chat message to all relevant subscribers
func (s *ChatService) BroadcastMessage(msg *model.ChatMessage) {
	// Broadcast to "all" subscribers
	s.broadcastToClientSubscribers("all", msg)
	
	// Broadcast to client-specific subscribers
	s.broadcastToClientSubscribers(msg.Client.ID, msg)
	
	// Broadcast to mentioned users
	for _, mention := range msg.Mentions {
		s.broadcastToMentionSubscribers(mention.ID, msg)
	}
}

// broadcastToClientSubscribers sends a message to client subscribers
func (s *ChatService) broadcastToClientSubscribers(clientID string, msg *model.ChatMessage) {
	s.clientMutex.RLock()
	defer s.clientMutex.RUnlock()
	
	for _, ch := range s.clientSubscribers[clientID] {
		select {
		case ch <- msg:
		default:
			// Channel buffer is full, skip
			s.logger.Warn("Skipped message broadcast - channel buffer full", "clientId", clientID)
		}
	}
}

// broadcastToMentionSubscribers sends a message to mention subscribers
func (s *ChatService) broadcastToMentionSubscribers(userID string, msg *model.ChatMessage) {
	s.mentionMutex.RLock()
	defer s.mentionMutex.RUnlock()
	
	for _, ch := range s.mentionSubscribers[userID] {
		select {
		case ch <- msg:
		default:
			// Channel buffer is full, skip
			s.logger.Warn("Skipped mention broadcast - channel buffer full", "userId", userID)
		}
	}
}

// FormatChatContent formats the chat content with highlighted mentions
func (s *ChatService) FormatChatContent(content string) string {
	// Replace @username mentions with styled spans
	re := regexp.MustCompile(`@(\w+)`)
	formattedContent := re.ReplaceAllString(content, `<span class="mention">@$1</span>`)
	
	// Add line breaks for improved readability
	formattedContent = strings.Replace(formattedContent, "\n", "<br>", -1)
	
	return formattedContent
}
