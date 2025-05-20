package main

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// QueryHandler handles queries
type QueryHandler interface {
	Handle(ctx context.Context, query interface{}) (interface{}, error)
}

// ResourceQueryHandler handles resource-related queries
type ResourceQueryHandler struct {
	storage       *nodestorage.StorageImpl[*ServerResource]
	readModelColl *mongo.Collection
	logger        *zap.Logger
}

// NewResourceQueryHandler creates a new ResourceQueryHandler
func NewResourceQueryHandler(
	storage *nodestorage.StorageImpl[*ServerResource],
	readModelColl *mongo.Collection,
	logger *zap.Logger,
) *ResourceQueryHandler {
	return &ResourceQueryHandler{
		storage:       storage,
		readModelColl: readModelColl,
		logger:        logger,
	}
}

// GetResourceByID retrieves a resource by ID
func (h *ResourceQueryHandler) GetResourceByID(ctx context.Context, id primitive.ObjectID) (*ClientResource, error) {
	// Get resource from storage
	resource, err := h.storage.FindOne(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	// Apply regeneration if needed (read-only, not persisted)
	if resource.RegenerationRate > 0 {
		elapsed := time.Since(resource.LastRegenerated).Seconds()
		regenAmount := int(elapsed * resource.RegenerationRate)
		if regenAmount > 0 {
			// Create a copy to avoid modifying the cached version
			resourceCopy := resource.Copy()
			resourceCopy.Amount = min(resourceCopy.Amount+regenAmount, resourceCopy.MaxAmount)
			resource = resourceCopy
		}
	}

	// Convert to client resource
	clientResource := resource.ToClientResource()

	return clientResource, nil
}

// GetResourcesByOwner retrieves all resources for a specific owner
func (h *ResourceQueryHandler) GetResourcesByOwner(ctx context.Context, ownerID string) ([]*ClientResource, error) {
	// Query resources by owner ID
	filter := bson.M{"owner_id": ownerID}
	cursor, err := h.storage.FindMany(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query resources: %w", err)
	}

	// Convert to client resources
	var clientResources []*ClientResource
	for _, resource := range cursor {
		// Apply regeneration if needed (read-only, not persisted)
		if resource.RegenerationRate > 0 {
			elapsed := time.Since(resource.LastRegenerated).Seconds()
			regenAmount := int(elapsed * resource.RegenerationRate)
			if regenAmount > 0 {
				// Create a copy to avoid modifying the cached version
				resourceCopy := resource.Copy()
				resourceCopy.Amount = min(resourceCopy.Amount+regenAmount, resourceCopy.MaxAmount)
				resource = resourceCopy
			}
		}

		clientResources = append(clientResources, resource.ToClientResource())
	}

	return clientResources, nil
}

// GetResourcesByType retrieves all resources of a specific type
func (h *ResourceQueryHandler) GetResourcesByType(ctx context.Context, resourceType string) ([]*ClientResource, error) {
	// Query resources by type
	filter := bson.M{"resource_type": resourceType}
	cursor, err := h.storage.FindMany(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query resources: %w", err)
	}

	// Convert to client resources
	var clientResources []*ClientResource
	for _, resource := range cursor {
		// Apply regeneration if needed (read-only, not persisted)
		if resource.RegenerationRate > 0 {
			elapsed := time.Since(resource.LastRegenerated).Seconds()
			regenAmount := int(elapsed * resource.RegenerationRate)
			if regenAmount > 0 {
				// Create a copy to avoid modifying the cached version
				resourceCopy := resource.Copy()
				resourceCopy.Amount = min(resourceCopy.Amount+regenAmount, resourceCopy.MaxAmount)
				resource = resourceCopy
			}
		}

		clientResources = append(clientResources, resource.ToClientResource())
	}

	return clientResources, nil
}

// GetResourceReadModel retrieves a resource read model by ID
func (h *ResourceQueryHandler) GetResourceReadModel(ctx context.Context, id string) (*ResourceReadModel, error) {
	// Convert string ID to ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("invalid resource ID: %w", err)
	}

	// Query read model
	var readModel ResourceReadModel
	err = h.readModelColl.FindOne(ctx, bson.M{"_id": id}).Decode(&readModel)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// If read model doesn't exist, try to get from source and create it
			resource, err := h.storage.FindOne(ctx, objectID)
			if err != nil {
				return nil, fmt.Errorf("failed to get resource: %w", err)
			}

			// Create read model
			readModel = *FromServerResource(resource)

			// Save read model
			_, err = h.readModelColl.InsertOne(ctx, readModel)
			if err != nil {
				h.logger.Warn("Failed to save read model",
					zap.Error(err),
					zap.String("resource_id", id))
				// Continue even if save fails
			}

			return &readModel, nil
		}
		return nil, fmt.Errorf("failed to get read model: %w", err)
	}

	return &readModel, nil
}

// GetResourceReadModels retrieves resource read models with pagination
func (h *ResourceQueryHandler) GetResourceReadModels(ctx context.Context, page, pageSize int64) ([]*ResourceReadModel, error) {
	// Set up pagination options
	opts := options.Find().
		SetSkip((page - 1) * pageSize).
		SetLimit(pageSize).
		SetSort(bson.D{{Key: "last_updated", Value: -1}})

	// Query read models
	cursor, err := h.readModelColl.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query read models: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	var readModels []*ResourceReadModel
	for cursor.Next(ctx) {
		var readModel ResourceReadModel
		if err := cursor.Decode(&readModel); err != nil {
			return nil, fmt.Errorf("failed to decode read model: %w", err)
		}
		readModels = append(readModels, &readModel)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return readModels, nil
}

// GetResourceReadModelsByOwner retrieves resource read models for a specific owner
func (h *ResourceQueryHandler) GetResourceReadModelsByOwner(ctx context.Context, ownerID string) ([]*ResourceReadModel, error) {
	// Query read models by owner ID
	cursor, err := h.readModelColl.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, fmt.Errorf("failed to query read models: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	var readModels []*ResourceReadModel
	for cursor.Next(ctx) {
		var readModel ResourceReadModel
		if err := cursor.Decode(&readModel); err != nil {
			return nil, fmt.Errorf("failed to decode read model: %w", err)
		}
		readModels = append(readModels, &readModel)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return readModels, nil
}
