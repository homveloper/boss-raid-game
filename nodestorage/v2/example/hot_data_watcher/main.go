package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"nodestorage/v2"
	"nodestorage/v2/cache"
	"nodestorage/v2/core"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Document is a sample document type
type Document struct {
	ID          primitive.ObjectID `bson:"_id"`
	Name        string             `bson:"name"`
	Value       int                `bson:"value"`
	VectorClock int64              `bson:"vector_clock"`
	UpdatedAt   time.Time          `bson:"updated_at"`
}

// Copy creates a deep copy of the document
func (d *Document) Copy() *Document {
	if d == nil {
		return nil
	}
	return &Document{
		ID:          d.ID,
		Name:        d.Name,
		Value:       d.Value,
		VectorClock: d.VectorClock,
		UpdatedAt:   d.UpdatedAt,
	}
}

func main() {
	// Configure logger
	err := core.ConfigureLogger(true, "debug", "stdout")
	if err != nil {
		log.Fatalf("Failed to configure logger: %v", err)
	}

	// Connect to MongoDB
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// Create a collection for this example
	collection := client.Database("hot_data_example").Collection("documents")

	// Create a memory cache
	memCache := cache.NewMemoryCache[*Document](nil)
	defer memCache.Close()

	// Create storage with hot data watcher enabled
	storageOptions := &nodestorage.Options{
		VersionField:          "vector_clock",
		CacheTTL:              time.Minute * 10,
		HotDataWatcherEnabled: true,
		HotDataMaxItems:       10,
		HotDataWatchInterval:  time.Second * 10, // Short interval for demo purposes
		HotDataDecayInterval:  time.Minute,
	}

	storage, err := nodestorage.NewStorage[*Document](ctx, collection, memCache, storageOptions)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create some test documents
	docIDs := createTestDocuments(ctx, storage, 20)

	// Simulate access patterns
	simulateAccessPatterns(ctx, storage, docIDs)

	// Wait for hot data watcher to update
	time.Sleep(time.Second * 15)

	// Update a hot document in MongoDB directly (bypassing the storage)
	updateDocumentDirectly(ctx, collection, docIDs[0])

	// Wait for hot data watcher to detect the change
	time.Sleep(time.Second * 5)

	// Verify that the cache was updated
	doc, err := storage.FindOne(ctx, docIDs[0])
	if err != nil {
		log.Fatalf("Failed to get document: %v", err)
	}

	fmt.Printf("Document after direct update: %+v\n", doc)

	// Keep the program running to observe hot data watcher behavior
	fmt.Println("Press Ctrl+C to exit")
	select {}
}

// createTestDocuments creates test documents
func createTestDocuments(ctx context.Context, storage nodestorage.Storage[*Document], count int) []primitive.ObjectID {
	var docIDs []primitive.ObjectID

	for i := 0; i < count; i++ {
		doc := &Document{
			ID:        primitive.NewObjectID(),
			Name:      fmt.Sprintf("Document %d", i),
			Value:     i * 10,
			UpdatedAt: time.Now(),
		}

		result, err := storage.FindOneAndUpsert(ctx, doc)
		if err != nil {
			core.Error("Failed to create document", zap.Error(err), zap.Int("index", i))
			continue
		}

		docIDs = append(docIDs, result.ID)
		core.Info("Created document", zap.String("id", result.ID.Hex()), zap.String("name", result.Name))
	}

	return docIDs
}

// simulateAccessPatterns simulates different access patterns
func simulateAccessPatterns(ctx context.Context, storage nodestorage.Storage[*Document], docIDs []primitive.ObjectID) {
	// Simulate frequent access to some documents
	go func() {
		for i := 0; i < 50; i++ {
			// Access first 5 documents frequently
			for j := 0; j < 5 && j < len(docIDs); j++ {
				_, err := storage.FindOne(ctx, docIDs[j])
				if err != nil {
					core.Error("Failed to get document", zap.Error(err), zap.Int("index", j))
				}
			}
			time.Sleep(time.Millisecond * 100)
		}
	}()

	// Simulate less frequent access to other documents
	go func() {
		for i := 0; i < 10; i++ {
			// Access other documents less frequently
			for j := 5; j < len(docIDs); j++ {
				_, err := storage.FindOne(ctx, docIDs[j])
				if err != nil {
					core.Error("Failed to get document", zap.Error(err), zap.Int("index", j))
				}
			}
			time.Sleep(time.Second)
		}
	}()
}

// updateDocumentDirectly updates a document directly in MongoDB
func updateDocumentDirectly(ctx context.Context, collection *mongo.Collection, id primitive.ObjectID) {
	// Update the document directly in MongoDB
	_, err := collection.UpdateOne(
		ctx,
		map[string]interface{}{"_id": id},
		map[string]interface{}{
			"$set": map[string]interface{}{
				"name":         "Updated Directly",
				"value":        999,
				"updated_at":   time.Now(),
				"vector_clock": 100, // Increment version
			},
		},
	)

	if err != nil {
		core.Error("Failed to update document directly", zap.Error(err), zap.String("id", id.Hex()))
		return
	}

	core.Info("Updated document directly", zap.String("id", id.Hex()))
}
