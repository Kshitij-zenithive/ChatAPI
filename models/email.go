package models

import (
        "time"

        "gorm.io/gorm"
)

// EmailDirection represents the direction of an email
type EmailDirection string

// Email directions
const (
        EmailInbound  EmailDirection = "INBOUND"
        EmailOutbound EmailDirection = "OUTBOUND"
)

// EmailStatus represents the status of an email
type EmailStatus string

// Email statuses
const (
        EmailStatusDraft     EmailStatus = "DRAFT"
        EmailStatusSent      EmailStatus = "SENT"
        EmailStatusDelivered EmailStatus = "DELIVERED"
        EmailStatusReceived  EmailStatus = "RECEIVED"
        EmailStatusFailed    EmailStatus = "FAILED"
)

// Email represents an email in the system
type Email struct {
        ID           uint           `gorm:"primaryKey" json:"id"`
        ClientID     uint           `gorm:"not null" json:"clientId"`
        UserID       uint           `gorm:"not null" json:"userId"`
        Subject      string         `gorm:"size:255;not null" json:"subject"`
        Body         string         `gorm:"type:text;not null" json:"body"`
        Direction    EmailDirection `gorm:"size:20;not null" json:"direction"`
        Status       EmailStatus    `gorm:"size:20;not null" json:"status"`
        ExternalID   string         `gorm:"size:255" json:"externalId,omitempty"`
        FromEmail    string         `gorm:"size:255;not null" json:"fromEmail"`
        ToEmail      string         `gorm:"size:255;not null" json:"toEmail"`
        SentAt       time.Time      `json:"sentAt,omitempty"`
        ReceivedAt   *time.Time     `json:"receivedAt,omitempty"`
        CreatedAt    time.Time      `json:"createdAt"`
        UpdatedAt    time.Time      `json:"updatedAt"`
        DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

        // Relationships
        Client Client `gorm:"foreignKey:ClientID" json:"client"`
        User   User   `gorm:"foreignKey:UserID" json:"user"`
}