package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
	"nodestorage/v2/cache"
	"nodestorage/v2/core"
)

func main() {
	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Set global logger for nodestorage
	core.SetLogger(logger)

	// Connect to MongoDB
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer client.Disconnect(ctx)

	// Ping MongoDB to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		logger.Fatal("Failed to ping MongoDB", zap.Error(err))
	}
	logger.Info("Connected to MongoDB")

	// Set up database and collections
	db := client.Database("server_authority_cqrs_example")
	resourceCollection := db.Collection("resources")
	eventCollection := db.Collection("events")
	readModelCollection := db.Collection("resource_read_models")

	// Create memory cache for resources
	memCache := cache.NewMemoryCache[*ServerResource](nil)
	defer memCache.Close()

	// Create storage options
	storageOptions := &nodestorage.Options{
		VersionField:      "version",
		CacheTTL:          time.Minute * 10,
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
	}

	// Create storage
	storage, err := nodestorage.NewStorage[*ServerResource](ctx, resourceCollection, memCache, storageOptions)
	if err != nil {
		logger.Fatal("Failed to create storage", zap.Error(err))
	}
	defer storage.Close()

	// Create event store
	eventStore := NewEventStore(eventCollection, logger)

	// Create command handler
	commandHandler := NewResourceCommandHandler(storage, eventStore, logger)

	// Create query handler
	queryHandler := NewResourceQueryHandler(storage, readModelCollection, logger)

	// Create event handler for updating read models
	eventHandler := NewResourceReadModelUpdater(readModelCollection, logger)

	// Set up event subscription
	watchCh, err := storage.Watch(ctx, mongo.Pipeline{}, nil)
	if err != nil {
		logger.Fatal("Failed to set up watch", zap.Error(err))
	}

	// Start event processing goroutine
	go func() {
		for event := range watchCh {
			logger.Info("Received storage event",
				zap.String("operation", event.Operation),
				zap.String("resource_id", event.ID.Hex()))

			// Process events for read model updates
			// In a real application, this would be handled by a proper event bus
			if event.Operation == "create" || event.Operation == "update" {
				// Simulate events based on storage operations
				// In a real application, events would come from the command handlers
				// This is just for demonstration purposes
				if event.Diff != nil && event.Diff.HasChanges {
					// Create a simple event based on the operation
					var simulatedEvent Event
					if event.Operation == "create" {
						simulatedEvent = &ResourceCreatedEvent{
							BaseEvent: BaseEvent{
								ID:          primitive.NewObjectID(),
								Type:        "ResourceCreated",
								AggregateId: event.ID,
								TimeStamp:   time.Now(),
								VersionNum:  event.Diff.Version,
							},
							OwnerID:          event.Data.OwnerID,
							ResourceType:     event.Data.ResourceType,
							InitialAmount:    event.Data.Amount,
							MaxAmount:        event.Data.MaxAmount,
							RegenerationRate: event.Data.RegenerationRate,
						}
					} else {
						// For simplicity, treat all updates as resource added events
						simulatedEvent = &ResourceAddedEvent{
							BaseEvent: BaseEvent{
								ID:          primitive.NewObjectID(),
								Type:        "ResourceAdded",
								AggregateId: event.ID,
								TimeStamp:   time.Now(),
								VersionNum:  event.Diff.Version,
							},
							Amount:    0, // We don't know the actual amount changed
							NewAmount: event.Data.Amount,
							Reason:    "Storage update",
							ClientID:  "system",
						}
					}

					// Handle the event
					if err := eventHandler.Handle(ctx, simulatedEvent); err != nil {
						logger.Error("Failed to handle simulated event",
							zap.Error(err),
							zap.String("event_type", simulatedEvent.EventType()))
					}
				}
			}
		}
	}()

	// Demonstrate server authority model with example commands
	logger.Info("Starting server authority CQRS example")

	// Example 1: Create a resource
	createCmd := NewCreateResourceCommand(
		"client1",
		"player123",
		"gold",
		1000,
		5000,
		0.5, // 0.5 gold per second regeneration rate
	)

	logger.Info("Executing CreateResourceCommand",
		zap.String("client_id", createCmd.ClientID),
		zap.String("owner_id", createCmd.OwnerID),
		zap.String("resource_type", createCmd.ResourceType))

	if err := commandHandler.Handle(ctx, createCmd); err != nil {
		logger.Error("Failed to create resource", zap.Error(err))
	} else {
		logger.Info("Resource created successfully")
	}

	// Wait a moment for the read model to be updated
	time.Sleep(time.Second)

	// Query resources by owner
	resources, err := queryHandler.GetResourcesByOwner(ctx, "player123")
	if err != nil {
		logger.Error("Failed to query resources", zap.Error(err))
	} else {
		logger.Info("Found resources", zap.Int("count", len(resources)))
		for i, resource := range resources {
			logger.Info(fmt.Sprintf("Resource %d", i+1),
				zap.String("id", resource.ID),
				zap.String("type", resource.ResourceType),
				zap.Int("amount", resource.Amount),
				zap.Int("max_amount", resource.MaxAmount))

			// Example 2: Allocate resources
			resourceID, _ := primitive.ObjectIDFromHex(resource.ID)
			allocateCmd := NewAllocateResourceCommand(
				"client1",
				resourceID,
				200,
				"Purchase item",
			)

			logger.Info("Executing AllocateResourceCommand",
				zap.String("client_id", allocateCmd.ClientID),
				zap.String("resource_id", allocateCmd.ResourceID.Hex()),
				zap.Int("amount", allocateCmd.Amount))

			if err := commandHandler.Handle(ctx, allocateCmd); err != nil {
				logger.Error("Failed to allocate resource", zap.Error(err))
			} else {
				logger.Info("Resource allocated successfully")
			}

			// Example 3: Add resources
			addCmd := NewAddResourceCommand(
				"client2",
				resourceID,
				500,
				"Quest reward",
			)

			logger.Info("Executing AddResourceCommand",
				zap.String("client_id", addCmd.ClientID),
				zap.String("resource_id", addCmd.ResourceID.Hex()),
				zap.Int("amount", addCmd.Amount))

			if err := commandHandler.Handle(ctx, addCmd); err != nil {
				logger.Error("Failed to add resource", zap.Error(err))
			} else {
				logger.Info("Resource added successfully")
			}

			// Example 4: Lock resource
			lockCmd := NewLockResourceCommand(
				"client3",
				resourceID,
				"Maintenance",
				time.Minute*5,
			)

			logger.Info("Executing LockResourceCommand",
				zap.String("client_id", lockCmd.ClientID),
				zap.String("resource_id", lockCmd.ResourceID.Hex()),
				zap.String("reason", lockCmd.Reason))

			if err := commandHandler.Handle(ctx, lockCmd); err != nil {
				logger.Error("Failed to lock resource", zap.Error(err))
			} else {
				logger.Info("Resource locked successfully")
			}

			// Example 5: Try to allocate locked resource (should fail)
			allocateLockedCmd := NewAllocateResourceCommand(
				"client1",
				resourceID,
				100,
				"Purchase item (should fail)",
			)

			logger.Info("Executing AllocateResourceCommand on locked resource",
				zap.String("client_id", allocateLockedCmd.ClientID),
				zap.String("resource_id", allocateLockedCmd.ResourceID.Hex()),
				zap.Int("amount", allocateLockedCmd.Amount))

			if err := commandHandler.Handle(ctx, allocateLockedCmd); err != nil {
				logger.Info("Expected failure: resource is locked", zap.Error(err))
			} else {
				logger.Error("Resource allocation succeeded unexpectedly")
			}

			// Example 6: Unlock resource
			unlockCmd := NewUnlockResourceCommand(
				"client3",
				resourceID,
			)

			logger.Info("Executing UnlockResourceCommand",
				zap.String("client_id", unlockCmd.ClientID),
				zap.String("resource_id", unlockCmd.ResourceID.Hex()))

			if err := commandHandler.Handle(ctx, unlockCmd); err != nil {
				logger.Error("Failed to unlock resource", zap.Error(err))
			} else {
				logger.Info("Resource unlocked successfully")
			}

			// Example 7: Allocate resource after unlock (should succeed)
			allocateAfterUnlockCmd := NewAllocateResourceCommand(
				"client1",
				resourceID,
				100,
				"Purchase item after unlock",
			)

			logger.Info("Executing AllocateResourceCommand after unlock",
				zap.String("client_id", allocateAfterUnlockCmd.ClientID),
				zap.String("resource_id", allocateAfterUnlockCmd.ResourceID.Hex()),
				zap.Int("amount", allocateAfterUnlockCmd.Amount))

			if err := commandHandler.Handle(ctx, allocateAfterUnlockCmd); err != nil {
				logger.Error("Failed to allocate resource after unlock", zap.Error(err))
			} else {
				logger.Info("Resource allocated successfully after unlock")
			}
		}
	}

	// Wait a moment for the read model to be updated
	time.Sleep(time.Second)

	// Query read models
	readModels, err := queryHandler.GetResourceReadModels(ctx, 1, 10)
	if err != nil {
		logger.Error("Failed to query read models", zap.Error(err))
	} else {
		logger.Info("Found read models", zap.Int("count", len(readModels)))
		for i, readModel := range readModels {
			logger.Info(fmt.Sprintf("Read Model %d", i+1),
				zap.String("id", readModel.ID),
				zap.String("owner_id", readModel.OwnerID),
				zap.String("type", readModel.ResourceType),
				zap.Int("amount", readModel.Amount),
				zap.Int("max_amount", readModel.MaxAmount),
				zap.Bool("is_locked", readModel.IsLocked))
		}
	}

	logger.Info("Server authority CQRS example completed")
}
