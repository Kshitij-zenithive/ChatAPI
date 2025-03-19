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

// CreateTimelineEvent handles the creation of a new timeline event
func (r *mutationResolver) CreateTimelineEvent(ctx context.Context, input model.CreateTimelineEventInput) (*model.TimelineEvent, error) {
	// Get user from context (added by auth middleware)
	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		return nil, ErrUnauthenticated
	}

	db := database.GetDB()

	// Create the timeline event
	timelineEvent := &models.TimelineEvent{
		ClientID:      input.ClientID,
		ActorID:       userID,
		EventableType: input.EventableType,
		EventableID:   input.EventableID,
		EventType:     input.EventType,
		Metadata:      input.Metadata,
		CreatedAt:     time.Now(),
	}

	if err := db.Create(timelineEvent).Error; err != nil {
		log.Printf("Error creating timeline event: %v", err)
		return nil, err
	}

	// Convert to GraphQL model
	result := &model.TimelineEvent{
		ID:            timelineEvent.ID,
		ClientID:      timelineEvent.ClientID,
		ActorID:       timelineEvent.ActorID,
		EventableType: timelineEvent.EventableType,
		EventableID:   timelineEvent.EventableID,
		EventType:     timelineEvent.EventType,
		Metadata:      timelineEvent.Metadata,
		CreatedAt:     timelineEvent.CreatedAt,
	}

	// Publish to subscription
	PublishTimelineEvent(input.ClientID, result)

	return result, nil
}

// DeleteTimelineEvent handles deleting a timeline event
func (r *mutationResolver) DeleteTimelineEvent(ctx context.Context, id uuid.UUID) (bool, error) {
	// Get user from context (added by auth middleware)
	userID, ok := ctx.Value("user_id").(uuid.UUID)
	if !ok {
		return false, ErrUnauthenticated
	}

	db := database.GetDB()

	// Check if the timeline event exists and belongs to the user
	var timelineEvent models.TimelineEvent
	if err := db.Where("id = ? AND actor_id = ?", id, userID).First(&timelineEvent).Error; err != nil {
		return false, err
	}

	// Delete the timeline event
	if err := db.Delete(&timelineEvent).Error; err != nil {
		return false, err
	}

	return true, nil
}

// TimelineEvents retrieves timeline events for a client
func (r *queryResolver) TimelineEvents(ctx context.Context, clientID uuid.UUID) ([]*model.TimelineEvent, error) {
	db := database.GetDB()

	var dbTimelineEvents []models.TimelineEvent
	if err := db.Where("client_id = ?", clientID).
		Order("created_at DESC").
		Preload("Actor").
		Find(&dbTimelineEvents).Error; err != nil {
		return nil, err
	}

	// Convert to GraphQL model
	var result []*model.TimelineEvent
	for _, e := range dbTimelineEvents {
		timelineEvent := &model.TimelineEvent{
			ID:            e.ID,
			ClientID:      e.ClientID,
			ActorID:       e.ActorID,
			EventableType: e.EventableType,
			EventableID:   e.EventableID,
			EventType:     e.EventType,
			Metadata:      e.Metadata,
			CreatedAt:     e.CreatedAt,
		}
		result = append(result, timelineEvent)
	}

	return result, nil
}

// TimelineEvent retrieves a single timeline event by ID
func (r *queryResolver) TimelineEvent(ctx context.Context, id uuid.UUID) (*model.TimelineEvent, error) {
	db := database.GetDB()

	var dbTimelineEvent models.TimelineEvent
	if err := db.Where("id = ?", id).
		Preload("Actor").
		First(&dbTimelineEvent).Error; err != nil {
		return nil, err
	}

	// Convert to GraphQL model
	result := &model.TimelineEvent{
		ID:            dbTimelineEvent.ID,
		ClientID:      dbTimelineEvent.ClientID,
		ActorID:       dbTimelineEvent.ActorID,
		EventableType: dbTimelineEvent.EventableType,
		EventableID:   dbTimelineEvent.EventableID,
		EventType:     dbTimelineEvent.EventType,
		Metadata:      dbTimelineEvent.Metadata,
		CreatedAt:     dbTimelineEvent.CreatedAt,
	}

	return result, nil
}