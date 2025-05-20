package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ServerResource represents the server-side resource model with full details
// This is the model stored in MongoDB and used by the server for business logic
type ServerResource struct {
	ID               primitive.ObjectID `bson:"_id"`
	OwnerID          string             `bson:"owner_id"`
	ResourceType     string             `bson:"resource_type"`
	Amount           int                `bson:"amount"`
	MaxAmount        int                `bson:"max_amount"`
	RegenerationRate float64            `bson:"regeneration_rate"`
	LastUpdated      time.Time          `bson:"last_updated"`
	LastRegenerated  time.Time          `bson:"last_regenerated"`
	Version          int64              `bson:"version"` // For optimistic concurrency control
	Metadata         map[string]interface{} `bson:"metadata,omitempty"`
	IsLocked         bool               `bson:"is_locked"`
	LockReason       string             `bson:"lock_reason,omitempty"`
	LockExpiry       *time.Time         `bson:"lock_expiry,omitempty"`
}

// Copy implements the nodestorage.Cachable interface
func (r *ServerResource) Copy() *ServerResource {
	if r == nil {
		return nil
	}

	// Create a deep copy of the resource
	copy := &ServerResource{
		ID:               r.ID,
		OwnerID:          r.OwnerID,
		ResourceType:     r.ResourceType,
		Amount:           r.Amount,
		MaxAmount:        r.MaxAmount,
		RegenerationRate: r.RegenerationRate,
		LastUpdated:      r.LastUpdated,
		LastRegenerated:  r.LastRegenerated,
		Version:          r.Version,
		IsLocked:         r.IsLocked,
		LockReason:       r.LockReason,
	}

	// Copy lock expiry if it exists
	if r.LockExpiry != nil {
		expiry := *r.LockExpiry
		copy.LockExpiry = &expiry
	}

	// Deep copy metadata map
	if r.Metadata != nil {
		copy.Metadata = make(map[string]interface{}, len(r.Metadata))
		for k, v := range r.Metadata {
			copy.Metadata[k] = v
		}
	}

	return copy
}

// ClientResource represents the client-side resource model with limited details
// This is the model sent to clients and used for display purposes
type ClientResource struct {
	ID           string    `json:"id"`
	OwnerID      string    `json:"owner_id"`
	ResourceType string    `json:"resource_type"`
	Amount       int       `json:"amount"`
	MaxAmount    int       `json:"max_amount"`
	LastUpdated  time.Time `json:"last_updated"`
	IsLocked     bool      `json:"is_locked"`
}

// ToClientResource converts a ServerResource to a ClientResource
func (r *ServerResource) ToClientResource() *ClientResource {
	return &ClientResource{
		ID:           r.ID.Hex(),
		OwnerID:      r.OwnerID,
		ResourceType: r.ResourceType,
		Amount:       r.Amount,
		MaxAmount:    r.MaxAmount,
		LastUpdated:  r.LastUpdated,
		IsLocked:     r.IsLocked,
	}
}

// ResourceEvent represents an event related to a resource change
type ResourceEvent struct {
	ID            primitive.ObjectID     `bson:"_id"`
	ResourceID    primitive.ObjectID     `bson:"resource_id"`
	EventType     string                 `bson:"event_type"`
	Timestamp     time.Time              `bson:"timestamp"`
	Data          map[string]interface{} `bson:"data"`
	Version       int64                  `bson:"version"`
	PreviousState *ServerResource        `bson:"previous_state,omitempty"`
	NewState      *ServerResource        `bson:"new_state,omitempty"`
}

// ResourceReadModel represents the read model for resources
// This is used for query operations in the CQRS pattern
type ResourceReadModel struct {
	ID               string    `bson:"_id" json:"id"`
	OwnerID          string    `bson:"owner_id" json:"owner_id"`
	ResourceType     string    `bson:"resource_type" json:"resource_type"`
	Amount           int       `bson:"amount" json:"amount"`
	MaxAmount        int       `bson:"max_amount" json:"max_amount"`
	RegenerationRate float64   `bson:"regeneration_rate" json:"regeneration_rate"`
	LastUpdated      time.Time `bson:"last_updated" json:"last_updated"`
	IsLocked         bool      `bson:"is_locked" json:"is_locked"`
}

// FromServerResource creates a ResourceReadModel from a ServerResource
func FromServerResource(r *ServerResource) *ResourceReadModel {
	return &ResourceReadModel{
		ID:               r.ID.Hex(),
		OwnerID:          r.OwnerID,
		ResourceType:     r.ResourceType,
		Amount:           r.Amount,
		MaxAmount:        r.MaxAmount,
		RegenerationRate: r.RegenerationRate,
		LastUpdated:      r.LastUpdated,
		IsLocked:         r.IsLocked,
	}
}
