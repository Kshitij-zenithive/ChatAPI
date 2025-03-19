package models

import (
        "time"

        "github.com/google/uuid"
        "gorm.io/gorm"
)

// EmailAttachment represents a file attachment to an email
type EmailAttachment struct {
        ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
        EmailID   uuid.UUID `gorm:"type:uuid;not null" json:"emailId"`
        Filename  string    `gorm:"type:varchar(255);not null" json:"filename"`
        Path      string    `gorm:"type:varchar(255);not null" json:"path"`
        Size      int64     `gorm:"type:bigint;not null" json:"size"`
        CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
        
        // Relations
        Email *Email `gorm:"foreignKey:EmailID" json:"-"`
}

// BeforeCreate is called before inserting a new email attachment into the database
func (ea *EmailAttachment) BeforeCreate(tx *gorm.DB) error {
        // Generate UUID if not set
        if ea.ID == uuid.Nil {
                ea.ID = uuid.New()
        }
        return nil
}

// AfterCreate is called after inserting a new email into the database
// It creates a timeline event for the email
func (e *Email) AfterCreate(tx *gorm.DB) error {
        timelineEvent := TimelineEvent{
                EventType:     "email",
                Title:         "Email sent: " + e.Subject,
                Content:       e.Body,
                ClientID:      e.ClientID,
                UserID:        e.UserID,
                EventableType: "Email",
                EventableID:   e.ID,
                EventTime:     time.Now(),
        }
        
        return tx.Create(&timelineEvent).Error
}