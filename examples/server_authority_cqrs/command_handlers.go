package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// CommandHandler handles commands
type CommandHandler interface {
	Handle(ctx context.Context, command Command) error
}

// ResourceCommandHandler handles resource-related commands
type ResourceCommandHandler struct {
	storage    *nodestorage.StorageImpl[*ServerResource]
	eventStore *EventStore
	logger     *zap.Logger
}

// NewResourceCommandHandler creates a new ResourceCommandHandler
func NewResourceCommandHandler(
	storage *nodestorage.StorageImpl[*ServerResource],
	eventStore *EventStore,
	logger *zap.Logger,
) *ResourceCommandHandler {
	return &ResourceCommandHandler{
		storage:    storage,
		eventStore: eventStore,
		logger:     logger,
	}
}

// Handle handles a command
func (h *ResourceCommandHandler) Handle(ctx context.Context, cmd Command) error {
	switch c := cmd.(type) {
	case *CreateResourceCommand:
		return h.handleCreateResource(ctx, c)
	case *AllocateResourceCommand:
		return h.handleAllocateResource(ctx, c)
	case *AddResourceCommand:
		return h.handleAddResource(ctx, c)
	case *LockResourceCommand:
		return h.handleLockResource(ctx, c)
	case *UnlockResourceCommand:
		return h.handleUnlockResource(ctx, c)
	default:
		return fmt.Errorf("unknown command type: %T", cmd)
	}
}

// handleCreateResource handles the CreateResourceCommand
func (h *ResourceCommandHandler) handleCreateResource(ctx context.Context, cmd *CreateResourceCommand) error {
	// Server-side validation
	if cmd.InitialAmount < 0 {
		return errors.New("initial amount cannot be negative")
	}
	if cmd.MaxAmount <= 0 {
		return errors.New("max amount must be positive")
	}
	if cmd.RegenerationRate < 0 {
		return errors.New("regeneration rate cannot be negative")
	}
	if cmd.InitialAmount > cmd.MaxAmount {
		return errors.New("initial amount cannot exceed max amount")
	}

	// Create new resource
	now := time.Now()
	resource := &ServerResource{
		ID:               primitive.NewObjectID(),
		OwnerID:          cmd.OwnerID,
		ResourceType:     cmd.ResourceType,
		Amount:           cmd.InitialAmount,
		MaxAmount:        cmd.MaxAmount,
		RegenerationRate: cmd.RegenerationRate,
		LastUpdated:      now,
		LastRegenerated:  now,
		Version:          1,
		Metadata:         make(map[string]interface{}),
		IsLocked:         false,
	}

	// Save to storage
	savedResource, err := h.storage.FindOneAndUpsert(ctx, resource)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create and store event
	event := NewResourceCreatedEvent(
		savedResource.ID,
		savedResource.OwnerID,
		savedResource.ResourceType,
		savedResource.Amount,
		savedResource.MaxAmount,
		savedResource.RegenerationRate,
		savedResource.Version,
	)

	if err := h.eventStore.StoreEvent(ctx, event); err != nil {
		h.logger.Error("Failed to store resource created event",
			zap.Error(err),
			zap.String("resource_id", savedResource.ID.Hex()))
		// Continue even if event storage fails
	}

	h.logger.Info("Resource created",
		zap.String("resource_id", savedResource.ID.Hex()),
		zap.String("owner_id", savedResource.OwnerID),
		zap.String("resource_type", savedResource.ResourceType))

	return nil
}

// handleAllocateResource handles the AllocateResourceCommand
func (h *ResourceCommandHandler) handleAllocateResource(ctx context.Context, cmd *AllocateResourceCommand) error {
	// Server-side validation
	if cmd.Amount <= 0 {
		return errors.New("allocation amount must be positive")
	}

	var updatedResource *ServerResource
	var allocatedAmount int

	// Use optimistic concurrency control with FindOneAndUpdate
	_, _, err := h.storage.FindOneAndUpdate(ctx, cmd.ResourceID, func(resource *ServerResource) (*ServerResource, error) {
		// Check if resource is locked
		if resource.IsLocked {
			if resource.LockExpiry != nil && time.Now().After(*resource.LockExpiry) {
				// Lock has expired, automatically unlock
				resource.IsLocked = false
				resource.LockReason = ""
				resource.LockExpiry = nil
			} else {
				return nil, fmt.Errorf("resource is locked: %s", resource.LockReason)
			}
		}

		// Apply regeneration if needed
		if resource.RegenerationRate > 0 {
			elapsed := time.Since(resource.LastRegenerated).Seconds()
			regenAmount := int(elapsed * resource.RegenerationRate)
			if regenAmount > 0 {
				resource.Amount = min(resource.Amount+regenAmount, resource.MaxAmount)
				resource.LastRegenerated = time.Now()
			}
		}

		// Check if there are enough resources
		if cmd.Amount > resource.Amount {
			return nil, fmt.Errorf("insufficient resources: requested %d, available %d", cmd.Amount, resource.Amount)
		}

		// Allocate resources
		resource.Amount -= cmd.Amount
		resource.LastUpdated = time.Now()
		allocatedAmount = cmd.Amount
		updatedResource = resource

		return resource, nil
	})

	if err != nil {
		return fmt.Errorf("failed to allocate resource: %w", err)
	}

	// Create and store event
	event := NewResourceAllocatedEvent(
		cmd.ResourceID,
		allocatedAmount,
		updatedResource.Amount,
		cmd.Reason,
		cmd.ClientID,
		updatedResource.Version,
	)

	if err := h.eventStore.StoreEvent(ctx, event); err != nil {
		h.logger.Error("Failed to store resource allocated event",
			zap.Error(err),
			zap.String("resource_id", cmd.ResourceID.Hex()))
		// Continue even if event storage fails
	}

	h.logger.Info("Resource allocated",
		zap.String("resource_id", cmd.ResourceID.Hex()),
		zap.Int("amount", allocatedAmount),
		zap.Int("new_amount", updatedResource.Amount),
		zap.String("reason", cmd.Reason))

	return nil
}

// handleAddResource handles the AddResourceCommand
func (h *ResourceCommandHandler) handleAddResource(ctx context.Context, cmd *AddResourceCommand) error {
	// Server-side validation
	if cmd.Amount <= 0 {
		return errors.New("add amount must be positive")
	}

	var updatedResource *ServerResource
	var addedAmount int

	// Use optimistic concurrency control with FindOneAndUpdate
	_, _, err := h.storage.FindOneAndUpdate(ctx, cmd.ResourceID, func(resource *ServerResource) (*ServerResource, error) {
		// Check if resource is locked
		if resource.IsLocked {
			if resource.LockExpiry != nil && time.Now().After(*resource.LockExpiry) {
				// Lock has expired, automatically unlock
				resource.IsLocked = false
				resource.LockReason = ""
				resource.LockExpiry = nil
			} else {
				return nil, fmt.Errorf("resource is locked: %s", resource.LockReason)
			}
		}

		// Apply regeneration if needed
		if resource.RegenerationRate > 0 {
			elapsed := time.Since(resource.LastRegenerated).Seconds()
			regenAmount := int(elapsed * resource.RegenerationRate)
			if regenAmount > 0 {
				resource.Amount = min(resource.Amount+regenAmount, resource.MaxAmount)
				resource.LastRegenerated = time.Now()
			}
		}

		// Add resources (capped at max amount)
		oldAmount := resource.Amount
		resource.Amount = min(resource.Amount+cmd.Amount, resource.MaxAmount)
		addedAmount = resource.Amount - oldAmount // Actual amount added (may be less than requested if capped)
		resource.LastUpdated = time.Now()
		updatedResource = resource

		return resource, nil
	})

	if err != nil {
		return fmt.Errorf("failed to add resource: %w", err)
	}

	// Create and store event
	event := NewResourceAddedEvent(
		cmd.ResourceID,
		addedAmount,
		updatedResource.Amount,
		cmd.Reason,
		cmd.ClientID,
		updatedResource.Version,
	)

	if err := h.eventStore.StoreEvent(ctx, event); err != nil {
		h.logger.Error("Failed to store resource added event",
			zap.Error(err),
			zap.String("resource_id", cmd.ResourceID.Hex()))
		// Continue even if event storage fails
	}

	h.logger.Info("Resource added",
		zap.String("resource_id", cmd.ResourceID.Hex()),
		zap.Int("amount", addedAmount),
		zap.Int("new_amount", updatedResource.Amount),
		zap.String("reason", cmd.Reason))

	return nil
}

// handleLockResource handles the LockResourceCommand
func (h *ResourceCommandHandler) handleLockResource(ctx context.Context, cmd *LockResourceCommand) error {
	// Server-side validation
	if cmd.Duration <= 0 {
		return errors.New("lock duration must be positive")
	}

	var updatedResource *ServerResource

	// Use optimistic concurrency control with FindOneAndUpdate
	_, _, err := h.storage.FindOneAndUpdate(ctx, cmd.ResourceID, func(resource *ServerResource) (*ServerResource, error) {
		// Check if resource is already locked
		if resource.IsLocked {
			if resource.LockExpiry != nil && time.Now().After(*resource.LockExpiry) {
				// Lock has expired, automatically unlock
				resource.IsLocked = false
				resource.LockReason = ""
				resource.LockExpiry = nil
			} else {
				return nil, fmt.Errorf("resource is already locked: %s", resource.LockReason)
			}
		}

		// Apply lock
		resource.IsLocked = true
		resource.LockReason = cmd.Reason
		expiry := time.Now().Add(cmd.Duration)
		resource.LockExpiry = &expiry
		resource.LastUpdated = time.Now()
		updatedResource = resource

		return resource, nil
	})

	if err != nil {
		return fmt.Errorf("failed to lock resource: %w", err)
	}

	// Create and store event
	event := &ResourceLockedEvent{
		BaseEvent: BaseEvent{
			ID:          primitive.NewObjectID(),
			Type:        "ResourceLocked",
			AggregateId: cmd.ResourceID,
			TimeStamp:   time.Now(),
			VersionNum:  updatedResource.Version,
		},
		Reason:   cmd.Reason,
		Duration: cmd.Duration.String(),
		Expiry:   updatedResource.LockExpiry,
		ClientID: cmd.ClientID,
	}

	if err := h.eventStore.StoreEvent(ctx, event); err != nil {
		h.logger.Error("Failed to store resource locked event",
			zap.Error(err),
			zap.String("resource_id", cmd.ResourceID.Hex()))
		// Continue even if event storage fails
	}

	h.logger.Info("Resource locked",
		zap.String("resource_id", cmd.ResourceID.Hex()),
		zap.String("reason", cmd.Reason),
		zap.Duration("duration", cmd.Duration),
		zap.Time("expiry", *updatedResource.LockExpiry))

	return nil
}

// handleUnlockResource handles the UnlockResourceCommand
func (h *ResourceCommandHandler) handleUnlockResource(ctx context.Context, cmd *UnlockResourceCommand) error {
	var updatedResource *ServerResource

	// Use optimistic concurrency control with FindOneAndUpdate
	_, _, err := h.storage.FindOneAndUpdate(ctx, cmd.ResourceID, func(resource *ServerResource) (*ServerResource, error) {
		// Check if resource is locked
		if !resource.IsLocked {
			return nil, errors.New("resource is not locked")
		}

		// Remove lock
		resource.IsLocked = false
		resource.LockReason = ""
		resource.LockExpiry = nil
		resource.LastUpdated = time.Now()
		updatedResource = resource

		return resource, nil
	})

	if err != nil {
		return fmt.Errorf("failed to unlock resource: %w", err)
	}

	// Create and store event
	event := &ResourceUnlockedEvent{
		BaseEvent: BaseEvent{
			ID:          primitive.NewObjectID(),
			Type:        "ResourceUnlocked",
			AggregateId: cmd.ResourceID,
			TimeStamp:   time.Now(),
			VersionNum:  updatedResource.Version,
		},
		ClientID: cmd.ClientID,
	}

	if err := h.eventStore.StoreEvent(ctx, event); err != nil {
		h.logger.Error("Failed to store resource unlocked event",
			zap.Error(err),
			zap.String("resource_id", cmd.ResourceID.Hex()))
		// Continue even if event storage fails
	}

	h.logger.Info("Resource unlocked",
		zap.String("resource_id", cmd.ResourceID.Hex()))

	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
