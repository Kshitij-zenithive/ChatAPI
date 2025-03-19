package models

import (
        "time"

        "gorm.io/gorm"
)

// Chat represents a chat thread in the system
type Chat struct {
        ID        uint           `gorm:"primaryKey" json:"id"`
        ClientID  uint           `gorm:"not null" json:"clientId"`
        Title     string         `gorm:"size:100" json:"title,omitempty"`
        CreatedAt time.Time      `json:"createdAt"`
        UpdatedAt time.Time      `json:"updatedAt"`
        DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

        // Relationships
        Client   Client    `gorm:"foreignKey:ClientID" json:"client"`
        Messages []Message `gorm:"foreignKey:ChatID" json:"messages,omitempty"`
}

// UnreadCount returns the count of unread messages in the chat thread
func (c *Chat) UnreadCount(userID uint) int {
        var count int64
        // Count messages not read by this user
        // This would typically be implemented as a database query
        // But for simplicity, we'll use a placeholder implementation
        return int(count)
}

// Message represents a chat message
type Message struct {
        ID        uint           `gorm:"primaryKey" json:"id"`
        ChatID    uint           `gorm:"not null" json:"threadId"`
        SenderID  uint           `gorm:"not null" json:"senderId"`
        Content   string         `gorm:"type:text;not null" json:"content"`
        CreatedAt time.Time      `json:"createdAt"`
        UpdatedAt time.Time      `json:"updatedAt"`
        DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

        // Relationships
        Chat     Chat   `gorm:"foreignKey:ChatID" json:"-"`
        Sender   User   `gorm:"foreignKey:SenderID" json:"user"`
        ReadBy   []User `gorm:"many2many:message_reads;" json:"readBy,omitempty"`
        Mentions []User `gorm:"many2many:message_mentions;" json:"mentions,omitempty"`
}