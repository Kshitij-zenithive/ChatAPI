package models

import (
        "time"

        "gorm.io/gorm"
)

// EventType represents the type of timeline event
type EventType string

// Event types
const (
        EventTypeClientCreated EventType = "CLIENT_CREATED"
        EventTypeEmailSent     EventType = "EMAIL_SENT"
        EventTypeEmailReceived EventType = "EMAIL_RECEIVED"
        EventTypeChatStarted   EventType = "CHAT_STARTED"
        EventTypeChatMessage   EventType = "CHAT_MESSAGE"
        EventTypeNote          EventType = "NOTE"
        EventTypeTask          EventType = "TASK"
)

// TimelineEvent represents a chronological record of client interactions
type TimelineEvent struct {
        ID            uint           `gorm:"primaryKey" json:"id"`
        ClientID      uint           `gorm:"not null" json:"clientId"`
        UserID        uint           `json:"userId,omitempty"`
        Type          EventType      `gorm:"size:20;not null" json:"type"`
        Title         string         `gorm:"size:100;not null" json:"title"`
        Description   string         `gorm:"type:text" json:"description,omitempty"`
        ReferenceID   uint           `json:"referenceId,omitempty"`
        ReferenceType string         `gorm:"size:50" json:"referenceType,omitempty"`
        Timestamp     time.Time      `json:"timestamp"`
        CreatedAt     time.Time      `json:"createdAt"`
        UpdatedAt     time.Time      `json:"updatedAt"`
        DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

        // Relationships
        Client Client `gorm:"foreignKey:ClientID" json:"client"`
        User   User   `gorm:"foreignKey:UserID" json:"user,omitempty"`
}