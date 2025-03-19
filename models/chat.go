package models

import (
	"time"

	"github.com/google/uuid"
)

// ChatThread represents a conversation thread
type ChatThread struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	Title     string    `json:"title"`
	ClientID  uuid.UUID `json:"client_id" gorm:"type:uuid"`
	CreatedBy uuid.UUID `json:"created_by" gorm:"type:uuid"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ChatMessage represents a single message in a chat thread
type ChatMessage struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	ChatID    uuid.UUID `json:"chat_id" gorm:"type:uuid"`
	SenderID  uuid.UUID `json:"sender_id" gorm:"type:uuid"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Mention represents an @mention in a message
type Mention struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	MessageID uuid.UUID `json:"message_id" gorm:"type:uuid"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid"`
	CreatedAt time.Time `json:"created_at"`
}

// MessageRead tracks when a user reads a message
type MessageRead struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key"`
	MessageID uuid.UUID `json:"message_id" gorm:"type:uuid"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid"`
	ReadAt    time.Time `json:"read_at"`
}

// WSMessage represents a WebSocket message structure
type WSMessage struct {
	Type      string      `json:"type"`
	ChatID    uuid.UUID   `json:"chat_id"`
	MessageID uuid.UUID   `json:"message_id,omitempty"`
	UserID    uuid.UUID   `json:"user_id"`
	Content   string      `json:"content,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Mentions  []uuid.UUID `json:"mentions,omitempty"`
}