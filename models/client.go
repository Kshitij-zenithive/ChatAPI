package models

import (
        "database/sql/driver"
        "encoding/json"
        "errors"
        "time"

        "gorm.io/gorm"
)

// StringSlice is a helper type for working with arrays of strings in PostgreSQL
type StringSlice []string

// Value converts StringSlice to database value
func (ss StringSlice) Value() (driver.Value, error) {
        if len(ss) == 0 {
                return nil, nil
        }
        bytes, err := json.Marshal(ss)
        return string(bytes), err
}

// Scan implements the sql.Scanner interface for StringSlice
func (ss *StringSlice) Scan(value interface{}) error {
        if value == nil {
                *ss = StringSlice{}
                return nil
        }

        var bytes []byte
        switch v := value.(type) {
        case []byte:
                bytes = v
        case string:
                bytes = []byte(v)
        default:
                return errors.New("failed to scan StringSlice")
        }

        return json.Unmarshal(bytes, ss)
}

// Client represents a client in the CRM system
type Client struct {
        ID           uint           `gorm:"primaryKey" json:"id"`
        Name         string         `gorm:"size:100;not null" json:"name"`
        Email        string         `gorm:"size:100;not null;uniqueIndex" json:"email"`
        Phone        string         `gorm:"size:20" json:"phone,omitempty"`
        Company      string         `gorm:"size:100" json:"company,omitempty"`
        AvatarURL    string         `gorm:"size:255" json:"avatarUrl,omitempty"`
        Notes        string         `gorm:"type:text" json:"notes,omitempty"`
        Tags         StringSlice    `gorm:"type:jsonb" json:"tags,omitempty"`
        Status       ClientStatus   `gorm:"size:20;not null;default:ACTIVE" json:"status"`
        Address      string         `gorm:"size:255" json:"address,omitempty"`
        Industry     string         `gorm:"size:100" json:"industry,omitempty"`
        AssignedToID *uint          `json:"assignedToId,omitempty"`
        CreatedAt    time.Time      `json:"createdAt"`
        UpdatedAt    time.Time      `json:"updatedAt"`
        DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

        // Relationships
        AssignedTo     *User          `gorm:"foreignKey:AssignedToID" json:"assignedTo,omitempty"`
        Chats          []Chat         `gorm:"foreignKey:ClientID" json:"chatThreads,omitempty"`
        Emails         []Email        `gorm:"foreignKey:ClientID" json:"emails,omitempty"`
        TimelineEvents []TimelineEvent `gorm:"foreignKey:ClientID" json:"timelineEvents,omitempty"`
}