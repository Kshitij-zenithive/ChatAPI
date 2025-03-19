package models

import (
        "time"

        "github.com/google/uuid"
        "gorm.io/gorm"
)

// TimelineEvent represents a timeline event in the system
type TimelineEvent struct {
        ID            uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
        EventType     string     `gorm:"type:varchar(50);not null" json:"eventType"`
        Title         string     `gorm:"type:varchar(255);not null" json:"title"`
        Content       string     `gorm:"type:text" json:"content"`
        ClientID      uuid.UUID  `gorm:"type:uuid;not null" json:"clientId"`
        UserID        uuid.UUID  `gorm:"type:uuid;not null" json:"userId"`
        EventableType string     `gorm:"type:varchar(50);not null" json:"eventableType"`
        EventableID   uuid.UUID  `gorm:"type:uuid;not null" json:"eventableId"`
        EventTime     time.Time  `gorm:"not null" json:"eventTime"`
        CreatedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
        UpdatedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updatedAt"`
        
        // Relations
        Client        Client     `gorm:"foreignKey:ClientID" json:"client"`
        User          User       `gorm:"foreignKey:UserID" json:"user"`
}

// BeforeCreate is called before inserting a new timeline event into the database
func (t *TimelineEvent) BeforeCreate(tx *gorm.DB) error {
        // Generate UUID if not set
        if t.ID == uuid.Nil {
                t.ID = uuid.New()
        }
        return nil
}