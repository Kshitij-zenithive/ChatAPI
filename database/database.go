package database

import (
        "log"
        "os"
        "time"

        "gorm.io/driver/postgres"
        "gorm.io/gorm"
)

var DB *gorm.DB

// User represents a user in the system
type User struct {
        ID        uint      `gorm:"primaryKey" json:"id"`
        Username  string    `gorm:"unique;not null" json:"username"`
        Email     string    `gorm:"unique;not null" json:"email"`
        CreatedAt time.Time `json:"created_at"`
        UpdatedAt time.Time `json:"updated_at"`
}

// Client represents a client entity in the CRM
type Client struct {
        ID        uint      `gorm:"primaryKey" json:"id"`
        Name      string    `gorm:"not null" json:"name"`
        Email     string    `gorm:"unique;not null" json:"email"`
        Phone     string    `json:"phone"`
        CreatedAt time.Time `json:"created_at"`
        UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a chat message
type Message struct {
        ID        uint      `gorm:"primaryKey" json:"id"`
        SenderID  uint      `gorm:"not null" json:"sender_id"`
        Sender    User      `gorm:"foreignKey:SenderID" json:"sender"`
        Content   string    `gorm:"not null" json:"content"`
        Mentions  string    `json:"mentions"` // Comma-separated list of mentioned user IDs
        CreatedAt time.Time `json:"created_at"`
}

// Email represents an email message in the system
type Email struct {
        ID          uint      `gorm:"primaryKey" json:"id"`
        Subject     string    `gorm:"not null" json:"subject"`
        Content     string    `gorm:"not null" json:"content"`
        SenderID    uint      `gorm:"not null" json:"sender_id"`
        Sender      User      `gorm:"foreignKey:SenderID" json:"sender"`
        RecipientID uint      `gorm:"not null" json:"recipient_id"`
        Recipient   Client    `gorm:"foreignKey:RecipientID" json:"recipient"`
        CreatedAt   time.Time `json:"created_at"`
}

// TimelineEvent represents an event in a client's timeline
type TimelineEvent struct {
        ID        uint      `gorm:"primaryKey" json:"id"`
        ClientID  uint      `gorm:"not null" json:"client_id"`
        Client    Client    `gorm:"foreignKey:ClientID" json:"client"`
        EventType string    `gorm:"not null" json:"event_type"` // e.g., "message", "email", etc.
        Details   string    `json:"details"`                    // JSON string with event details
        CreatedAt time.Time `json:"created_at"`
}

// InitDB initializes the database connection
func InitDB() {
        var err error
        dsn := os.Getenv("DATABASE_URL")
        
        DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
        if err != nil {
                log.Fatalf("Failed to connect to database: %v", err)
        }
        
        log.Println("Database connected successfully")
}