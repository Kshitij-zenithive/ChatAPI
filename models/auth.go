package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RefreshToken represents a refresh token in the database
type RefreshToken struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;index;not null"`
	Token     string    `json:"token" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"type:timestamp;not null;default:now()"`
	ExpiresAt time.Time `json:"expires_at" gorm:"type:timestamp;not null"`
	
	// Relations
	User      *User     `json:"user" gorm:"foreignKey:UserID"`
}

// OAuthProvider represents an OAuth provider
type OAuthProvider struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID      `json:"user_id" gorm:"type:uuid;index;not null"`
	Provider  string         `json:"provider" gorm:"type:varchar(50);not null"`
	ProviderID string        `json:"provider_id" gorm:"type:varchar(255);not null"`
	AccessToken string       `json:"access_token" gorm:"type:text"`
	RefreshToken string      `json:"refresh_token" gorm:"type:text"`
	ExpiresAt time.Time      `json:"expires_at" gorm:"type:timestamp"`
	CreatedAt time.Time      `json:"created_at" gorm:"type:timestamp;not null;default:now()"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"type:timestamp;not null;default:now()"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
	
	// Relations
	User      *User          `json:"user" gorm:"foreignKey:UserID"`
}

// Email represents an email that has been imported via Gmail API
type Email struct {
	ID          uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClientID    uuid.UUID      `json:"client_id" gorm:"type:uuid;index;not null"`
	UserID      uuid.UUID      `json:"user_id" gorm:"type:uuid;index;not null"`
	GoogleID    string         `json:"google_id" gorm:"type:varchar(255);uniqueIndex;not null"`
	Subject     string         `json:"subject" gorm:"type:varchar(255);not null"`
	From        string         `json:"from" gorm:"type:varchar(255);not null"`
	To          string         `json:"to" gorm:"type:varchar(255);not null"`
	Body        string         `json:"body" gorm:"type:text"`
	Snippet     string         `json:"snippet" gorm:"type:text"`
	ThreadID    string         `json:"thread_id" gorm:"type:varchar(255);index"`
	Received    time.Time      `json:"received" gorm:"type:timestamp;not null"`
	CreatedAt   time.Time      `json:"created_at" gorm:"type:timestamp;not null;default:now()"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"type:timestamp;not null;default:now()"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
	
	// Relations
	Client     *Client        `json:"client" gorm:"foreignKey:ClientID"`
	User       *User          `json:"user" gorm:"foreignKey:UserID"`
	Timeline   []TimelineEvent `json:"timeline" gorm:"polymorphic:Eventable"`
}

// TimelineEvent represents an event in a client's timeline (chat, email, or other interaction)
type TimelineEvent struct {
	ID            uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ClientID      uuid.UUID      `json:"client_id" gorm:"type:uuid;index;not null"`
	UserID        uuid.UUID      `json:"user_id" gorm:"type:uuid;index;not null"`
	EventableType string         `json:"eventable_type" gorm:"type:varchar(255);not null"`
	EventableID   uuid.UUID      `json:"eventable_id" gorm:"type:uuid;not null"`
	EventType     string         `json:"event_type" gorm:"type:varchar(50);not null"` // chat_message, email_sent, email_received, note, etc.
	Title         string         `json:"title" gorm:"type:varchar(255);not null"`
	Content       string         `json:"content" gorm:"type:text"`
	EventTime     time.Time      `json:"event_time" gorm:"type:timestamp;not null"`
	CreatedAt     time.Time      `json:"created_at" gorm:"type:timestamp;not null;default:now()"`
	UpdatedAt     time.Time      `json:"updated_at" gorm:"type:timestamp;not null;default:now()"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
	
	// Relations
	Client       *Client        `json:"client" gorm:"foreignKey:ClientID"`
	User         *User          `json:"user" gorm:"foreignKey:UserID"`
}