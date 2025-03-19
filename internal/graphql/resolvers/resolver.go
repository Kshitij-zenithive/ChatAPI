package resolvers

import (
        "sync"

        "crm-communication-api/database"
        "gorm.io/gorm"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

// Resolver is the resolver root.
type Resolver struct {
        DB           *gorm.DB
        mutex        sync.Mutex
        subscriptions map[string][]chan interface{}
}

// NewResolver creates a new resolver with database connection
func NewResolver() *Resolver {
        return &Resolver{
                DB:           database.GetDB(),
                subscriptions: make(map[string][]chan interface{}),
        }
}
