package main

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// EventHandler handles events
type EventHandler interface {
	Handle(ctx context.Context, event Event) error
}

// ResourceReadModelUpdater updates the read model based on events
type ResourceReadModelUpdater struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

// NewResourceReadModelUpdater creates a new ResourceReadModelUpdater
func NewResourceReadModelUpdater(collection *mongo.Collection, logger *zap.Logger) *ResourceReadModelUpdater {
	return &ResourceReadModelUpdater{
		collection: collection,
		logger:     logger,
	}
}

// Handle handles an event
func (h *ResourceReadModelUpdater) Handle(ctx context.Context, event Event) error {
	switch e := event.(type) {
	case *ResourceCreatedEvent:
		return h.handleResourceCreated(ctx, e)
	case *ResourceAllocatedEvent:
		return h.handleResourceAllocated(ctx, e)
	case *ResourceAddedEvent:
		return h.handleResourceAdded(ctx, e)
	case *ResourceRegeneratedEvent:
		return h.handleResourceRegenerated(ctx, e)
	case *ResourceLockedEvent:
		return h.handleResourceLocked(ctx, e)
	case *ResourceUnlockedEvent:
		return h.handleResourceUnlocked(ctx, e)
	default:
		return fmt.Errorf("unknown event type: %T", event)
	}
}

// handleResourceCreated handles a ResourceCreatedEvent
func (h *ResourceReadModelUpdater) handleResourceCreated(ctx context.Context, event *ResourceCreatedEvent) error {
	// Create read model
	readModel := &ResourceReadModel{
		ID:               event.AggregateID().Hex(),
		OwnerID:          event.OwnerID,
		ResourceType:     event.ResourceType,
		Amount:           event.InitialAmount,
		MaxAmount:        event.MaxAmount,
		RegenerationRate: event.RegenerationRate,
		LastUpdated:      event.EventedAt(),
		IsLocked:         false,
	}

	// Insert read model
	_, err := h.collection.InsertOne(ctx, readModel)
	if err != nil {
		return fmt.Errorf("failed to insert read model: %w", err)
	}

	h.logger.Info("Read model created",
		zap.String("resource_id", readModel.ID),
		zap.String("owner_id", readModel.OwnerID),
		zap.String("resource_type", readModel.ResourceType))

	return nil
}

// handleResourceAllocated handles a ResourceAllocatedEvent
func (h *ResourceReadModelUpdater) handleResourceAllocated(ctx context.Context, event *ResourceAllocatedEvent) error {
	// Update read model
	filter := bson.M{"_id": event.AggregateID().Hex()}
	update := bson.M{
		"$set": bson.M{
			"amount":       event.NewAmount,
			"last_updated": event.EventedAt(),
		},
	}
	opts := options.Update().SetUpsert(false)

	result, err := h.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update read model: %w", err)
	}

	if result.MatchedCount == 0 {
		h.logger.Warn("Read model not found for update",
			zap.String("resource_id", event.AggregateID().Hex()),
			zap.String("event_type", event.EventType()))
	}

	h.logger.Info("Read model updated (resource allocated)",
		zap.String("resource_id", event.AggregateID().Hex()),
		zap.Int("amount", event.Amount),
		zap.Int("new_amount", event.NewAmount))

	return nil
}

// handleResourceAdded handles a ResourceAddedEvent
func (h *ResourceReadModelUpdater) handleResourceAdded(ctx context.Context, event *ResourceAddedEvent) error {
	// Update read model
	filter := bson.M{"_id": event.AggregateID().Hex()}
	update := bson.M{
		"$set": bson.M{
			"amount":       event.NewAmount,
			"last_updated": event.EventedAt(),
		},
	}
	opts := options.Update().SetUpsert(false)

	result, err := h.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update read model: %w", err)
	}

	if result.MatchedCount == 0 {
		h.logger.Warn("Read model not found for update",
			zap.String("resource_id", event.AggregateID().Hex()),
			zap.String("event_type", event.EventType()))
	}

	h.logger.Info("Read model updated (resource added)",
		zap.String("resource_id", event.AggregateID().Hex()),
		zap.Int("amount", event.Amount),
		zap.Int("new_amount", event.NewAmount))

	return nil
}

// handleResourceRegenerated handles a ResourceRegeneratedEvent
func (h *ResourceReadModelUpdater) handleResourceRegenerated(ctx context.Context, event *ResourceRegeneratedEvent) error {
	// Update read model
	filter := bson.M{"_id": event.AggregateID().Hex()}
	update := bson.M{
		"$set": bson.M{
			"amount":       event.NewAmount,
			"last_updated": event.EventedAt(),
		},
	}
	opts := options.Update().SetUpsert(false)

	result, err := h.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update read model: %w", err)
	}

	if result.MatchedCount == 0 {
		h.logger.Warn("Read model not found for update",
			zap.String("resource_id", event.AggregateID().Hex()),
			zap.String("event_type", event.EventType()))
	}

	h.logger.Info("Read model updated (resource regenerated)",
		zap.String("resource_id", event.AggregateID().Hex()),
		zap.Int("amount", event.Amount),
		zap.Int("new_amount", event.NewAmount))

	return nil
}

// handleResourceLocked handles a ResourceLockedEvent
func (h *ResourceReadModelUpdater) handleResourceLocked(ctx context.Context, event *ResourceLockedEvent) error {
	// Update read model
	filter := bson.M{"_id": event.AggregateID().Hex()}
	update := bson.M{
		"$set": bson.M{
			"is_locked":    true,
			"last_updated": event.EventedAt(),
		},
	}
	opts := options.Update().SetUpsert(false)

	result, err := h.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update read model: %w", err)
	}

	if result.MatchedCount == 0 {
		h.logger.Warn("Read model not found for update",
			zap.String("resource_id", event.AggregateID().Hex()),
			zap.String("event_type", event.EventType()))
	}

	h.logger.Info("Read model updated (resource locked)",
		zap.String("resource_id", event.AggregateID().Hex()),
		zap.String("reason", event.Reason))

	return nil
}

// handleResourceUnlocked handles a ResourceUnlockedEvent
func (h *ResourceReadModelUpdater) handleResourceUnlocked(ctx context.Context, event *ResourceUnlockedEvent) error {
	// Update read model
	filter := bson.M{"_id": event.AggregateID().Hex()}
	update := bson.M{
		"$set": bson.M{
			"is_locked":    false,
			"last_updated": event.EventedAt(),
		},
	}
	opts := options.Update().SetUpsert(false)

	result, err := h.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update read model: %w", err)
	}

	if result.MatchedCount == 0 {
		h.logger.Warn("Read model not found for update",
			zap.String("resource_id", event.AggregateID().Hex()),
			zap.String("event_type", event.EventType()))
	}

	h.logger.Info("Read model updated (resource unlocked)",
		zap.String("resource_id", event.AggregateID().Hex()))

	return nil
}
