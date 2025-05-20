package main

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// EventStore handles storing and retrieving events
type EventStore struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

// NewEventStore creates a new EventStore
func NewEventStore(collection *mongo.Collection, logger *zap.Logger) *EventStore {
	return &EventStore{
		collection: collection,
		logger:     logger,
	}
}

// StoreEvent stores an event in the event store
func (s *EventStore) StoreEvent(ctx context.Context, event Event) error {
	// Convert event to BSON document
	doc, err := bson.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Insert event into collection
	_, err = s.collection.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	return nil
}

// GetEvents retrieves all events for a specific aggregate
func (s *EventStore) GetEvents(ctx context.Context, aggregateID primitive.ObjectID) ([]Event, error) {
	// Create filter for the aggregate ID
	filter := bson.M{"aggregate_id": aggregateID}

	// Set up options to sort by version
	opts := options.Find().SetSort(bson.D{{Key: "version", Value: 1}})

	// Execute query
	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	var events []Event
	for cursor.Next(ctx) {
		// Decode document into a map
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}

		// Create appropriate event type based on the event type field
		eventType, ok := doc["type"].(string)
		if !ok {
			return nil, fmt.Errorf("event missing type field")
		}

		var event Event
		switch eventType {
		case "ResourceCreated":
			var e ResourceCreatedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceAllocated":
			var e ResourceAllocatedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceAdded":
			var e ResourceAddedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceRegenerated":
			var e ResourceRegeneratedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceLocked":
			var e ResourceLockedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceUnlocked":
			var e ResourceUnlockedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		default:
			return nil, fmt.Errorf("unknown event type: %s", eventType)
		}

		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return events, nil
}

// GetEventsSince retrieves all events for a specific aggregate since a specific version
func (s *EventStore) GetEventsSince(ctx context.Context, aggregateID primitive.ObjectID, sinceVersion int64) ([]Event, error) {
	// Create filter for the aggregate ID and version greater than sinceVersion
	filter := bson.M{
		"aggregate_id": aggregateID,
		"version":      bson.M{"$gt": sinceVersion},
	}

	// Set up options to sort by version
	opts := options.Find().SetSort(bson.D{{Key: "version", Value: 1}})

	// Execute query
	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	var events []Event
	for cursor.Next(ctx) {
		// Decode document into a map
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}

		// Create appropriate event type based on the event type field
		eventType, ok := doc["type"].(string)
		if !ok {
			return nil, fmt.Errorf("event missing type field")
		}

		var event Event
		switch eventType {
		case "ResourceCreated":
			var e ResourceCreatedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceAllocated":
			var e ResourceAllocatedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceAdded":
			var e ResourceAddedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceRegenerated":
			var e ResourceRegeneratedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceLocked":
			var e ResourceLockedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceUnlocked":
			var e ResourceUnlockedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		default:
			return nil, fmt.Errorf("unknown event type: %s", eventType)
		}

		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return events, nil
}

// remarshalEvent converts a bson.M to a specific event type
func remarshalEvent(doc bson.M, event interface{}) error {
	data, err := bson.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	if err := bson.Unmarshal(data, event); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	return nil
}

// GetLatestEvents retrieves the latest events across all aggregates
func (s *EventStore) GetLatestEvents(ctx context.Context, limit int64) ([]Event, error) {
	// Set up options to sort by timestamp in descending order and limit results
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetLimit(limit)

	// Execute query
	cursor, err := s.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest events: %w", err)
	}
	defer cursor.Close(ctx)

	// Process results
	var events []Event
	for cursor.Next(ctx) {
		// Decode document into a map
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode event: %w", err)
		}

		// Create appropriate event type based on the event type field
		eventType, ok := doc["type"].(string)
		if !ok {
			return nil, fmt.Errorf("event missing type field")
		}

		var event Event
		switch eventType {
		case "ResourceCreated":
			var e ResourceCreatedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceAllocated":
			var e ResourceAllocatedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceAdded":
			var e ResourceAddedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceRegenerated":
			var e ResourceRegeneratedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceLocked":
			var e ResourceLockedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		case "ResourceUnlocked":
			var e ResourceUnlockedEvent
			if err := remarshalEvent(doc, &e); err != nil {
				return nil, err
			}
			event = &e
		default:
			return nil, fmt.Errorf("unknown event type: %s", eventType)
		}

		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return events, nil
}
