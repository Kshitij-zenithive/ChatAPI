package service

import (
	"context"
	"fmt"

	"github.com/your-org/crm-communication-api/database"
	"github.com/your-org/crm-communication-api/graph/model"
	"github.com/your-org/crm-communication-api/util"
)

// InteractionService handles interaction-related operations
type InteractionService struct {
	db     *database.DB
	logger *util.Logger
}

// NewInteractionService creates a new interaction service
func NewInteractionService(db *database.DB, logger *util.Logger) *InteractionService {
	return &InteractionService{
		db:     db,
		logger: logger,
	}
}

// GetInteractions gets all interactions for a client
func (s *InteractionService) GetInteractions(ctx context.Context, clientID string) ([]model.Interaction, error) {
	return s.db.GetInteractionsForClient(ctx, clientID)
}

// LogInteraction logs an interaction
func (s *InteractionService) LogInteraction(ctx context.Context, interaction model.Interaction) error {
	// This method serves as a central logging point for all interactions
	// It could be extended with additional functionality like:
	// - Analytics
	// - Notification triggers
	// - AI processing of interactions
	
	interactionType := "unknown"
	switch interaction.GetType() {
	case model.InteractionTypeChatMessage:
		interactionType = "chat"
	case model.InteractionTypeEmailSent:
		interactionType = "email_sent"
	case model.InteractionTypeEmailReceived:
		interactionType = "email_received"
	}
	
	s.logger.Info("Interaction logged",
		"type", interactionType,
		"id", interaction.GetID(),
		"clientId", interaction.GetClient().ID,
		"userId", interaction.GetUser().ID)
	
	return nil
}

// AnalyzeInteractions performs analysis on client interactions
func (s *InteractionService) AnalyzeInteractions(ctx context.Context, clientID string) (map[string]interface{}, error) {
	// Get all interactions
	interactions, err := s.db.GetInteractionsForClient(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get interactions: %v", err)
	}
	
	// Initialize counters
	chatCount := 0
	emailSentCount := 0
	emailReceivedCount := 0
	
	// Count interaction types
	for _, interaction := range interactions {
		switch interaction.GetType() {
		case model.InteractionTypeChatMessage:
			chatCount++
		case model.InteractionTypeEmailSent:
			emailSentCount++
		case model.InteractionTypeEmailReceived:
			emailReceivedCount++
		}
	}
	
	// Calculate overall statistics
	totalInteractions := len(interactions)
	var averageResponseTime float64 = 0 // Would calculate from timestamps
	
	// Return analysis results
	analysis := map[string]interface{}{
		"totalInteractions":    totalInteractions,
		"chatCount":            chatCount,
		"emailSentCount":       emailSentCount,
		"emailReceivedCount":   emailReceivedCount,
		"averageResponseTime":  averageResponseTime,
	}
	
	return analysis, nil
}

// GenerateClientTimeline generates a timeline of client interactions
func (s *InteractionService) GenerateClientTimeline(ctx context.Context, clientID string) ([]map[string]interface{}, error) {
	// Get all interactions
	interactions, err := s.db.GetInteractionsForClient(ctx, clientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get interactions: %v", err)
	}
	
	// Format interactions for timeline
	timeline := make([]map[string]interface{}, 0, len(interactions))
	
	for _, interaction := range interactions {
		entry := map[string]interface{}{
			"id":        interaction.GetID(),
			"timestamp": interaction.GetCreatedAt(),
			"type":      interaction.GetType(),
			"user": map[string]interface{}{
				"id":   interaction.GetUser().ID,
				"name": interaction.GetUser().Name,
			},
		}
		
		// Add type-specific data
		switch i := interaction.(type) {
		case *model.ChatMessage:
			entry["content"] = i.Content
			entry["mentions"] = i.Mentions
		case *model.EmailInteraction:
			entry["subject"] = i.Subject
			entry["content"] = i.Content
			entry["emailId"] = i.EmailID
			if i.ThreadID != nil {
				entry["threadId"] = *i.ThreadID
			}
		}
		
		timeline = append(timeline, entry)
	}
	
	return timeline, nil
}
