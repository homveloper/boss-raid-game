package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Event is the interface for all events
type Event interface {
	EventType() string
	AggregateID() primitive.ObjectID
	EventedAt() time.Time
	Version() int64
}

// BaseEvent provides common fields for all events
type BaseEvent struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Type        string             `bson:"type" json:"type"`
	AggregateId primitive.ObjectID `bson:"aggregate_id" json:"aggregate_id"`
	TimeStamp   time.Time          `bson:"timestamp" json:"timestamp"`
	VersionNum  int64              `bson:"version" json:"version"`
}

// EventType returns the type of the event
func (e *BaseEvent) EventType() string {
	return e.Type
}

// AggregateID returns the ID of the aggregate this event applies to
func (e *BaseEvent) AggregateID() primitive.ObjectID {
	return e.AggregateId
}

// Timestamp returns when the event occurred
func (e *BaseEvent) EventedAt() time.Time {
	return e.TimeStamp
}

// Version returns the version of the aggregate after this event
func (e *BaseEvent) Version() int64 {
	return e.VersionNum
}

// ResourceCreatedEvent is emitted when a resource is created
type ResourceCreatedEvent struct {
	BaseEvent
	OwnerID          string  `bson:"owner_id" json:"owner_id"`
	ResourceType     string  `bson:"resource_type" json:"resource_type"`
	InitialAmount    int     `bson:"initial_amount" json:"initial_amount"`
	MaxAmount        int     `bson:"max_amount" json:"max_amount"`
	RegenerationRate float64 `bson:"regeneration_rate" json:"regeneration_rate"`
}

// NewResourceCreatedEvent creates a new ResourceCreatedEvent
func NewResourceCreatedEvent(resourceID primitive.ObjectID, ownerID, resourceType string, initialAmount, maxAmount int, regenerationRate float64, version int64) *ResourceCreatedEvent {
	return &ResourceCreatedEvent{
		BaseEvent: BaseEvent{
			ID:          primitive.NewObjectID(),
			Type:        "ResourceCreated",
			AggregateId: resourceID,
			TimeStamp:   time.Now(),
			VersionNum:  version,
		},
		OwnerID:          ownerID,
		ResourceType:     resourceType,
		InitialAmount:    initialAmount,
		MaxAmount:        maxAmount,
		RegenerationRate: regenerationRate,
	}
}

// ResourceAllocatedEvent is emitted when resources are allocated (consumed)
type ResourceAllocatedEvent struct {
	BaseEvent
	Amount    int    `bson:"amount" json:"amount"`
	NewAmount int    `bson:"new_amount" json:"new_amount"`
	Reason    string `bson:"reason" json:"reason"`
	ClientID  string `bson:"client_id" json:"client_id"`
}

// NewResourceAllocatedEvent creates a new ResourceAllocatedEvent
func NewResourceAllocatedEvent(resourceID primitive.ObjectID, amount, newAmount int, reason, clientID string, version int64) *ResourceAllocatedEvent {
	return &ResourceAllocatedEvent{
		BaseEvent: BaseEvent{
			ID:          primitive.NewObjectID(),
			Type:        "ResourceAllocated",
			AggregateId: resourceID,
			TimeStamp:   time.Now(),
			VersionNum:  version,
		},
		Amount:    amount,
		NewAmount: newAmount,
		Reason:    reason,
		ClientID:  clientID,
	}
}

// ResourceAddedEvent is emitted when resources are added
type ResourceAddedEvent struct {
	BaseEvent
	Amount    int    `bson:"amount" json:"amount"`
	NewAmount int    `bson:"new_amount" json:"new_amount"`
	Reason    string `bson:"reason" json:"reason"`
	ClientID  string `bson:"client_id" json:"client_id"`
}

// NewResourceAddedEvent creates a new ResourceAddedEvent
func NewResourceAddedEvent(resourceID primitive.ObjectID, amount, newAmount int, reason, clientID string, version int64) *ResourceAddedEvent {
	return &ResourceAddedEvent{
		BaseEvent: BaseEvent{
			ID:          primitive.NewObjectID(),
			Type:        "ResourceAdded",
			AggregateId: resourceID,
			TimeStamp:   time.Now(),
			VersionNum:  version,
		},
		Amount:    amount,
		NewAmount: newAmount,
		Reason:    reason,
		ClientID:  clientID,
	}
}

// ResourceRegeneratedEvent is emitted when resources regenerate
type ResourceRegeneratedEvent struct {
	BaseEvent
	Amount    int       `bson:"amount" json:"amount"`
	NewAmount int       `bson:"new_amount" json:"new_amount"`
	Timestamp time.Time `bson:"timestamp" json:"timestamp"`
}

// NewResourceRegeneratedEvent creates a new ResourceRegeneratedEvent
func NewResourceRegeneratedEvent(resourceID primitive.ObjectID, amount, newAmount int, version int64) *ResourceRegeneratedEvent {
	now := time.Now()
	return &ResourceRegeneratedEvent{
		BaseEvent: BaseEvent{
			ID:          primitive.NewObjectID(),
			Type:        "ResourceRegenerated",
			AggregateId: resourceID,
			TimeStamp:   now,
			VersionNum:  version,
		},
		Amount:    amount,
		NewAmount: newAmount,
		Timestamp: now,
	}
}

// ResourceLockedEvent is emitted when a resource is locked
type ResourceLockedEvent struct {
	BaseEvent
	Reason   string     `bson:"reason" json:"reason"`
	Duration string     `bson:"duration" json:"duration"`
	Expiry   *time.Time `bson:"expiry" json:"expiry"`
	ClientID string     `bson:"client_id" json:"client_id"`
}

// ResourceUnlockedEvent is emitted when a resource is unlocked
type ResourceUnlockedEvent struct {
	BaseEvent
	ClientID string `bson:"client_id" json:"client_id"`
}
