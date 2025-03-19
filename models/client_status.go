package models

// ClientStatus represents the status of a client
type ClientStatus string

// Client statuses
const (
	StatusActive   ClientStatus = "ACTIVE"
	StatusInactive ClientStatus = "INACTIVE"
	StatusLead     ClientStatus = "LEAD"
)