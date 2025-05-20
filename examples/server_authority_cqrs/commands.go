package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Command is the interface for all commands
type Command interface {
	Type() string
}

// BaseCommand provides common fields for all commands
type BaseCommand struct {
	ID          string    `json:"id"`
	CommandType string    `json:"command_type"`
	Timestamp   time.Time `json:"timestamp"`
	ClientID    string    `json:"client_id"`
}

func (c *BaseCommand) Type() string {
	return c.CommandType
}

// CreateResourceCommand is used to create a new resource
type CreateResourceCommand struct {
	BaseCommand
	OwnerID          string  `json:"owner_id"`
	ResourceType     string  `json:"resource_type"`
	InitialAmount    int     `json:"initial_amount"`
	MaxAmount        int     `json:"max_amount"`
	RegenerationRate float64 `json:"regeneration_rate"`
}

// NewCreateResourceCommand creates a new CreateResourceCommand
func NewCreateResourceCommand(clientID, ownerID, resourceType string, initialAmount, maxAmount int, regenerationRate float64) *CreateResourceCommand {
	return &CreateResourceCommand{
		BaseCommand: BaseCommand{
			ID:          primitive.NewObjectID().Hex(),
			CommandType: "CreateResource",
			Timestamp:   time.Now(),
			ClientID:    clientID,
		},
		OwnerID:          ownerID,
		ResourceType:     resourceType,
		InitialAmount:    initialAmount,
		MaxAmount:        maxAmount,
		RegenerationRate: regenerationRate,
	}
}

// AllocateResourceCommand is used to allocate (consume) resources
type AllocateResourceCommand struct {
	BaseCommand
	ResourceID primitive.ObjectID `json:"resource_id"`
	Amount     int                `json:"amount"`
	Reason     string             `json:"reason"`
}

// NewAllocateResourceCommand creates a new AllocateResourceCommand
func NewAllocateResourceCommand(clientID string, resourceID primitive.ObjectID, amount int, reason string) *AllocateResourceCommand {
	return &AllocateResourceCommand{
		BaseCommand: BaseCommand{
			ID:          primitive.NewObjectID().Hex(),
			CommandType: "AllocateResource",
			Timestamp:   time.Now(),
			ClientID:    clientID,
		},
		ResourceID: resourceID,
		Amount:     amount,
		Reason:     reason,
	}
}

// AddResourceCommand is used to add resources
type AddResourceCommand struct {
	BaseCommand
	ResourceID primitive.ObjectID `json:"resource_id"`
	Amount     int                `json:"amount"`
	Reason     string             `json:"reason"`
}

// NewAddResourceCommand creates a new AddResourceCommand
func NewAddResourceCommand(clientID string, resourceID primitive.ObjectID, amount int, reason string) *AddResourceCommand {
	return &AddResourceCommand{
		BaseCommand: BaseCommand{
			ID:          primitive.NewObjectID().Hex(),
			CommandType: "AddResource",
			Timestamp:   time.Now(),
			ClientID:    clientID,
		},
		ResourceID: resourceID,
		Amount:     amount,
		Reason:     reason,
	}
}

// LockResourceCommand is used to lock a resource
type LockResourceCommand struct {
	BaseCommand
	ResourceID primitive.ObjectID `json:"resource_id"`
	Reason     string             `json:"reason"`
	Duration   time.Duration      `json:"duration"`
}

// NewLockResourceCommand creates a new LockResourceCommand
func NewLockResourceCommand(clientID string, resourceID primitive.ObjectID, reason string, duration time.Duration) *LockResourceCommand {
	return &LockResourceCommand{
		BaseCommand: BaseCommand{
			ID:          primitive.NewObjectID().Hex(),
			CommandType: "LockResource",
			Timestamp:   time.Now(),
			ClientID:    clientID,
		},
		ResourceID: resourceID,
		Reason:     reason,
		Duration:   duration,
	}
}

// UnlockResourceCommand is used to unlock a resource
type UnlockResourceCommand struct {
	BaseCommand
	ResourceID primitive.ObjectID `json:"resource_id"`
}

// NewUnlockResourceCommand creates a new UnlockResourceCommand
func NewUnlockResourceCommand(clientID string, resourceID primitive.ObjectID) *UnlockResourceCommand {
	return &UnlockResourceCommand{
		BaseCommand: BaseCommand{
			ID:          primitive.NewObjectID().Hex(),
			CommandType: "UnlockResource",
			Timestamp:   time.Now(),
			ClientID:    clientID,
		},
		ResourceID: resourceID,
	}
}
