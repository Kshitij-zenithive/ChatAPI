package models

import (
        "time"

        "gorm.io/gorm"
)

// UserRole is an enum for user role types
type UserRole string

// UserRole values
const (
        RoleAdmin   UserRole = "ADMIN"
        RoleManager UserRole = "MANAGER"
        RoleAgent   UserRole = "AGENT"
)

// User represents a system user
type User struct {
        ID        uint           `gorm:"primaryKey" json:"id"`
        Name      string         `gorm:"size:100;not null" json:"name"`
        Email     string         `gorm:"size:100;not null;unique" json:"email"`
        Password  string         `gorm:"size:255;not null" json:"-"` // Password not exposed in JSON
        Role      UserRole       `gorm:"type:varchar(20);default:AGENT;not null" json:"role"`
        AvatarURL string         `gorm:"size:255" json:"avatarUrl"`
        CreatedAt time.Time      `json:"createdAt"`
        UpdatedAt time.Time      `json:"updatedAt"`
        DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

        // Relationships
        Clients        []Client        `gorm:"foreignKey:AssignedToID" json:"-"`
        SentMessages   []Message       `gorm:"foreignKey:SenderID" json:"-"`
        ReadMessages   []Message       `gorm:"many2many:message_reads;" json:"-"`
        Mentions       []Message       `gorm:"many2many:message_mentions;" json:"-"`
        Emails         []Email         `gorm:"foreignKey:UserID" json:"-"`
        TimelineEvents []TimelineEvent `gorm:"foreignKey:UserID" json:"-"`
}