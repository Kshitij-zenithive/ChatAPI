package resolvers

import (
	"context"
	"log"
	"regexp"
	"time"

	"crm-communication-api/database"
	"crm-communication-api/internal/graphql/model"
	"crm-communication-api/models"

	"github.com/google/uuid"
)

// CreateMessage handles the creation of a new message with @mention support
func (r *mutationResolver) CreateMessage(ctx context.Context, input model.CreateMessageInput) (*model.Message, error) {
	// Get user from context (added by auth middleware)
	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		return nil, ErrUnauthenticated
	}

	db := database.GetDB()

	// Create the message
	message := &models.Message{
		Content:   input.Content,
		SenderID:  userID,
		ClientID:  input.ClientID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Begin transaction
	tx := db.Begin()
	if err := tx.Create(message).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating message: %v", err)
		return nil, err
	}

	// Process mentions if any
	if len(input.Mentions) > 0 {
		for _, mentionID := range input.Mentions {
			mention := &models.MessageMention{
				MessageID: message.ID,
				UserID:    mentionID,
				CreatedAt: time.Now(),
			}
			if err := tx.Create(mention).Error; err != nil {
				tx.Rollback()
				log.Printf("Error creating mention: %v", err)
				return nil, err
			}
		}
	} else {
		// Check for @mentions in the message content
		mentions := extractMentionsFromContent(input.Content)
		if len(mentions) > 0 {
			// Find users by username/email
			var users []models.User
			if err := tx.Where("name IN ?", mentions).Find(&users).Error; err != nil {
				log.Printf("Error finding mentioned users: %v", err)
				// Continue without mentions in case of error
			} else {
				// Create mentions for found users
				for _, user := range users {
					mention := &models.MessageMention{
						MessageID: message.ID,
						UserID:    user.ID,
						CreatedAt: time.Now(),
					}
					if err := tx.Create(mention).Error; err != nil {
						log.Printf("Error creating mention from content: %v", err)
						// Continue with other mentions
					}
				}
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		return nil, err
	}

	// Convert to GraphQL model
	result := &model.Message{
		ID:        message.ID,
		Content:   message.Content,
		ClientID:  message.ClientID,
		SenderID:  message.SenderID,
		CreatedAt: message.CreatedAt,
		UpdatedAt: message.UpdatedAt,
	}

	// Publish to subscription
	PublishMessage(input.ClientID, result)

	return result, nil
}

// DeleteMessage handles deleting a message
func (r *mutationResolver) DeleteMessage(ctx context.Context, id uuid.UUID) (bool, error) {
	// Get user from context (added by auth middleware)
	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		return false, ErrUnauthenticated
	}

	db := database.GetDB()

	// Check if the message exists and belongs to the user
	var message models.Message
	if err := db.Where("id = ? AND sender_id = ?", id, userID).First(&message).Error; err != nil {
		return false, err
	}

	// Begin transaction
	tx := db.Begin()

	// Delete mentions first
	if err := tx.Where("message_id = ?", id).Delete(&models.MessageMention{}).Error; err != nil {
		tx.Rollback()
		return false, err
	}

	// Delete timeline events for this message
	if err := tx.Where("eventable_type = ? AND eventable_id = ?", "Message", id).Delete(&models.TimelineEvent{}).Error; err != nil {
		tx.Rollback()
		return false, err
	}

	// Delete the message
	if err := tx.Delete(&message).Error; err != nil {
		tx.Rollback()
		return false, err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	return true, nil
}

// Messages retrieves messages for a client
func (r *queryResolver) Messages(ctx context.Context, clientID uuid.UUID) ([]*model.Message, error) {
	db := database.GetDB()

	var dbMessages []models.Message
	if err := db.Where("client_id = ?", clientID).
		Order("created_at DESC").
		Preload("Sender").
		Preload("Mentions.User").
		Find(&dbMessages).Error; err != nil {
		return nil, err
	}

	// Convert to GraphQL model
	var result []*model.Message
	for _, m := range dbMessages {
		message := &model.Message{
			ID:        m.ID,
			Content:   m.Content,
			ClientID:  m.ClientID,
			SenderID:  m.SenderID,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		}
		result = append(result, message)
	}

	return result, nil
}

// Message retrieves a single message by ID
func (r *queryResolver) Message(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	db := database.GetDB()

	var dbMessage models.Message
	if err := db.Where("id = ?", id).
		Preload("Sender").
		Preload("Mentions.User").
		First(&dbMessage).Error; err != nil {
		return nil, err
	}

	// Convert to GraphQL model
	result := &model.Message{
		ID:        dbMessage.ID,
		Content:   dbMessage.Content,
		ClientID:  dbMessage.ClientID,
		SenderID:  dbMessage.SenderID,
		CreatedAt: dbMessage.CreatedAt,
		UpdatedAt: dbMessage.UpdatedAt,
	}

	return result, nil
}

// Helper function to extract @mentions from message content
func extractMentionsFromContent(content string) []string {
	mentionRegex := regexp.MustCompile(`@(\w+)`)
	matches := mentionRegex.FindAllStringSubmatch(content, -1)
	
	var mentions []string
	for _, match := range matches {
		if len(match) >= 2 {
			mentions = append(mentions, match[1])
		}
	}
	
	return mentions
}

// Common errors
var (
	ErrUnauthenticated = Errorf("not authenticated")
)

// Errorf creates a formatted error
func Errorf(format string, args ...interface{}) error {
	return &UserError{
		Message: format,
		Args:    args,
	}
}

// UserError represents an error that's meant to be displayed to the user
type UserError struct {
	Message string
	Args    []interface{}
}

// Error implements the error interface
func (e *UserError) Error() string {
	return e.Message
}