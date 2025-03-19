package models

import (
        "time"

        "github.com/google/uuid"
        "gorm.io/gorm"
)

// Message represents a chat message in the system
type Message struct {
        ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
        Content   string    `gorm:"type:text;not null" json:"content"`
        SenderID  uuid.UUID `gorm:"type:uuid;not null" json:"senderId"`
        ClientID  uuid.UUID `gorm:"type:uuid;not null" json:"clientId"`
        CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
        UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updatedAt"`
        
        // Relations
        Sender   User          `gorm:"foreignKey:SenderID" json:"sender"`
        Client   Client        `gorm:"foreignKey:ClientID" json:"client"`
        Mentions []MessageMention `gorm:"foreignKey:MessageID" json:"mentions,omitempty"`
}

// MessageMention represents a mention of a user in a message
type MessageMention struct {
        ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
        MessageID uuid.UUID `gorm:"type:uuid;not null" json:"messageId"`
        UserID    uuid.UUID `gorm:"type:uuid;not null" json:"userId"`
        CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
        
        // Relations
        Message Message `gorm:"foreignKey:MessageID" json:"-"`
        User    User    `gorm:"foreignKey:UserID" json:"user"`
}

// BeforeCreate is called before inserting a new message into the database
func (m *Message) BeforeCreate(tx *gorm.DB) error {
        // Generate UUID if not set
        if m.ID == uuid.Nil {
                m.ID = uuid.New()
        }
        return nil
}

// BeforeCreate is called before inserting a new message mention into the database
func (mm *MessageMention) BeforeCreate(tx *gorm.DB) error {
        // Generate UUID if not set
        if mm.ID == uuid.Nil {
                mm.ID = uuid.New()
        }
        return nil
}

// AfterCreate is called after inserting a new message into the database
// It creates a timeline event for the message
func (m *Message) AfterCreate(tx *gorm.DB) error {
        timelineEvent := TimelineEvent{
                EventType:     "message",
                Title:         "New message sent",
                Content:       m.Content,
                ClientID:      m.ClientID,
                UserID:        m.SenderID,
                EventableType: "Message",
                EventableID:   m.ID,
                EventTime:     time.Now(),
        }
        
        return tx.Create(&timelineEvent).Error
}