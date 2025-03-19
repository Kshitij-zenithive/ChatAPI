package resolvers

import (
	"context"
	"log"
	"time"

	"crm-communication-api/database"
	"crm-communication-api/internal/graphql/model"
	"crm-communication-api/models"

	"github.com/google/uuid"
)

// CreateEmail handles the creation of a new email record
func (r *mutationResolver) CreateEmail(ctx context.Context, input model.CreateEmailInput) (*model.Email, error) {
	// Get user from context (added by auth middleware)
	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		return nil, ErrUnauthenticated
	}

	db := database.GetDB()

	// Create the email record
	email := &models.Email{
		Subject:     input.Subject,
		Body:        input.Body,
		SenderID:    userID,
		ClientID:    input.ClientID,
		ToAddresses: input.ToAddresses,
		CcAddresses: input.CcAddresses,
		BccAddresses: input.BccAddresses,
		ExternalID:  input.ExternalID,
		Status:      "sent", // Default status
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Begin transaction
	tx := db.Begin()
	if err := tx.Create(email).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating email: %v", err)
		return nil, err
	}

	// Create timeline event for the email
	timelineEvent := &models.TimelineEvent{
		ClientID:      input.ClientID,
		ActorID:       userID,
		EventableType: "Email",
		EventableID:   email.ID,
		EventType:     "EmailSent",
		Metadata:      map[string]interface{}{"subject": input.Subject},
		CreatedAt:     time.Now(),
	}

	if err := tx.Create(timelineEvent).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating timeline event: %v", err)
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		return nil, err
	}

	// Convert to GraphQL model
	result := &model.Email{
		ID:          email.ID,
		Subject:     email.Subject,
		Body:        email.Body,
		ClientID:    email.ClientID,
		SenderID:    email.SenderID,
		ToAddresses: email.ToAddresses,
		CcAddresses: email.CcAddresses,
		BccAddresses: email.BccAddresses,
		ExternalID:  email.ExternalID,
		Status:      email.Status,
		CreatedAt:   email.CreatedAt,
		UpdatedAt:   email.UpdatedAt,
	}

	// Publish to subscription
	PublishEmail(input.ClientID, result)

	// Create and publish timeline event
	gqlTimelineEvent := &model.TimelineEvent{
		ID:           timelineEvent.ID,
		ClientID:     timelineEvent.ClientID,
		ActorID:      timelineEvent.ActorID,
		EventableType: timelineEvent.EventableType,
		EventableID:  timelineEvent.EventableID,
		EventType:    timelineEvent.EventType,
		Metadata:     timelineEvent.Metadata,
		CreatedAt:    timelineEvent.CreatedAt,
	}
	PublishTimelineEvent(input.ClientID, gqlTimelineEvent)

	return result, nil
}

// Emails retrieves emails for a client
func (r *queryResolver) Emails(ctx context.Context, clientID uuid.UUID) ([]*model.Email, error) {
	db := database.GetDB()

	var dbEmails []models.Email
	if err := db.Where("client_id = ?", clientID).
		Order("created_at DESC").
		Preload("Sender").
		Find(&dbEmails).Error; err != nil {
		return nil, err
	}

	// Convert to GraphQL model
	var result []*model.Email
	for _, e := range dbEmails {
		email := &model.Email{
			ID:          e.ID,
			Subject:     e.Subject,
			Body:        e.Body,
			ClientID:    e.ClientID,
			SenderID:    e.SenderID,
			ToAddresses: e.ToAddresses,
			CcAddresses: e.CcAddresses,
			BccAddresses: e.BccAddresses,
			ExternalID:  e.ExternalID,
			Status:      e.Status,
			CreatedAt:   e.CreatedAt,
			UpdatedAt:   e.UpdatedAt,
		}
		result = append(result, email)
	}

	return result, nil
}

// Email retrieves a single email by ID
func (r *queryResolver) Email(ctx context.Context, id uuid.UUID) (*model.Email, error) {
	db := database.GetDB()

	var dbEmail models.Email
	if err := db.Where("id = ?", id).
		Preload("Sender").
		First(&dbEmail).Error; err != nil {
		return nil, err
	}

	// Convert to GraphQL model
	result := &model.Email{
		ID:          dbEmail.ID,
		Subject:     dbEmail.Subject,
		Body:        dbEmail.Body,
		ClientID:    dbEmail.ClientID,
		SenderID:    dbEmail.SenderID,
		ToAddresses: dbEmail.ToAddresses,
		CcAddresses: dbEmail.CcAddresses,
		BccAddresses: dbEmail.BccAddresses,
		ExternalID:  dbEmail.ExternalID,
		Status:      dbEmail.Status,
		CreatedAt:   dbEmail.CreatedAt,
		UpdatedAt:   dbEmail.UpdatedAt,
	}

	return result, nil
}