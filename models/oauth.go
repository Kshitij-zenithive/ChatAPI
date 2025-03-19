package models

import (
        "time"

        "gorm.io/gorm"
)

// OAuthToken represents an OAuth token for external services (e.g., Gmail)
type OAuthToken struct {
        ID           uint           `gorm:"primaryKey" json:"id"`
        UserID       uint           `gorm:"not null" json:"userId"`
        Provider     string         `gorm:"size:50;not null" json:"provider"`
        AccessToken  string         `gorm:"size:4096;not null" json:"-"` // Not exposed in JSON
        RefreshToken string         `gorm:"size:4096;not null" json:"-"` // Not exposed in JSON
        TokenType    string         `gorm:"size:50;not null" json:"tokenType"`
        Expiry       time.Time      `json:"expiry"`
        CreatedAt    time.Time      `json:"createdAt"`
        UpdatedAt    time.Time      `json:"updatedAt"`
        DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

        // Relationships
        User User `gorm:"foreignKey:UserID" json:"-"`
}