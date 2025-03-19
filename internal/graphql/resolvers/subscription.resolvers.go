package resolvers

import (
	"context"
	"fmt"
	"log"
	"sync"

	"crm-communication-api/internal/graphql/model"
	"github.com/google/uuid"
)

type Observer struct {
	id      string
	events  chan interface{}
	closeCh chan struct{}
}

type EventManager struct {
	observers map[string][]*Observer
	mu        sync.RWMutex
}

var (
	// Global event manager instance
	eventManager = &EventManager{
		observers: make(map[string][]*Observer),
	}
)

// Register adds a new observer for a specific client
func (m *EventManager) Register(clientID uuid.UUID, observer *Observer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := clientID.String()
	m.observers[key] = append(m.observers[key], observer)
	log.Printf("Observer %s registered for client %s", observer.id, key)
}

// Unregister removes an observer
func (m *EventManager) Unregister(clientID uuid.UUID, observer *Observer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := clientID.String()
	
	observers := m.observers[key]
	for i, obs := range observers {
		if obs.id == observer.id {
			m.observers[key] = append(observers[:i], observers[i+1:]...)
			close(obs.events)
			close(obs.closeCh)
			log.Printf("Observer %s unregistered from client %s", observer.id, key)
			break
		}
	}
	
	// Clean up if no observers left for this client
	if len(m.observers[key]) == 0 {
		delete(m.observers, key)
	}
}

// Broadcast sends an event to all observers for a specific client
func (m *EventManager) Broadcast(clientID uuid.UUID, event interface{}, eventType string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := clientID.String()
	
	for _, observer := range m.observers[key] {
		select {
		case observer.events <- event:
			log.Printf("Event %s sent to observer %s", eventType, observer.id)
		default:
			log.Printf("Observer %s channel full, dropping event", observer.id)
		}
	}
}

// NewObserver creates a new observer
func NewObserver() *Observer {
	return &Observer{
		id:      uuid.New().String(),
		events:  make(chan interface{}, 10), // Buffer for 10 events
		closeCh: make(chan struct{}),
	}
}

// MessageCreated subscription resolver
func (r *subscriptionResolver) MessageCreated(ctx context.Context, clientID uuid.UUID) (<-chan *model.Message, error) {
	observer := NewObserver()
	eventManager.Register(clientID, observer)
	
	messageChan := make(chan *model.Message, 1)
	
	// Handle cleanup when subscription is closed
	go func() {
		<-ctx.Done()
		eventManager.Unregister(clientID, observer)
		close(messageChan)
		log.Printf("MessageCreated subscription closed for client %s", clientID.String())
	}()
	
	// Forward events to the typed channel
	go func() {
		for {
			select {
			case event := <-observer.events:
				if message, ok := event.(*model.Message); ok {
					messageChan <- message
				}
			case <-observer.closeCh:
				return
			}
		}
	}()
	
	return messageChan, nil
}

// EmailCreated subscription resolver
func (r *subscriptionResolver) EmailCreated(ctx context.Context, clientID uuid.UUID) (<-chan *model.Email, error) {
	observer := NewObserver()
	eventManager.Register(clientID, observer)
	
	emailChan := make(chan *model.Email, 1)
	
	// Handle cleanup when subscription is closed
	go func() {
		<-ctx.Done()
		eventManager.Unregister(clientID, observer)
		close(emailChan)
		log.Printf("EmailCreated subscription closed for client %s", clientID.String())
	}()
	
	// Forward events to the typed channel
	go func() {
		for {
			select {
			case event := <-observer.events:
				if email, ok := event.(*model.Email); ok {
					emailChan <- email
				}
			case <-observer.closeCh:
				return
			}
		}
	}()
	
	return emailChan, nil
}

// TimelineEventCreated subscription resolver
func (r *subscriptionResolver) TimelineEventCreated(ctx context.Context, clientID uuid.UUID) (<-chan *model.TimelineEvent, error) {
	observer := NewObserver()
	eventManager.Register(clientID, observer)
	
	timelineEventChan := make(chan *model.TimelineEvent, 1)
	
	// Handle cleanup when subscription is closed
	go func() {
		<-ctx.Done()
		eventManager.Unregister(clientID, observer)
		close(timelineEventChan)
		log.Printf("TimelineEventCreated subscription closed for client %s", clientID.String())
	}()
	
	// Forward events to the typed channel
	go func() {
		for {
			select {
			case event := <-observer.events:
				if timelineEvent, ok := event.(*model.TimelineEvent); ok {
					timelineEventChan <- timelineEvent
				}
			case <-observer.closeCh:
				return
			}
		}
	}()
	
	return timelineEventChan, nil
}

// PublishMessage publishes a message to all subscribers
func PublishMessage(clientID uuid.UUID, message *model.Message) {
	eventManager.Broadcast(clientID, message, "MessageCreated")
}

// PublishEmail publishes an email to all subscribers
func PublishEmail(clientID uuid.UUID, email *model.Email) {
	eventManager.Broadcast(clientID, email, "EmailCreated")
}

// PublishTimelineEvent publishes a timeline event to all subscribers
func PublishTimelineEvent(clientID uuid.UUID, event *model.TimelineEvent) {
	eventManager.Broadcast(clientID, event, "TimelineEventCreated")
}