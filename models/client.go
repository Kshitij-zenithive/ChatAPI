package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Client represents a customer in the CRM system
type Client struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	Email     string    `gorm:"type:varchar(100);unique;not null" json:"email"`
	Phone     string    `gorm:"type:varchar(20)" json:"phone"`
	Company   string    `gorm:"type:varchar(100)" json:"company"`
	Notes     string    `gorm:"type:text" json:"notes"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP;autoUpdateTime" json:"updatedAt"`
	
	// Relations
	Messages      []Message       `gorm:"foreignKey:ClientID" json:"messages,omitempty"`
	Emails        []Email         `gorm:"foreignKey:ClientID" json:"emails,omitempty"`
	TimelineEvents []TimelineEvent `gorm:"foreignKey:ClientID" json:"timeline,omitempty"`
}

// BeforeCreate is called before inserting a new client into the database
func (c *Client) BeforeCreate(tx *gorm.DB) error {
	// Generate UUID if not set
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}