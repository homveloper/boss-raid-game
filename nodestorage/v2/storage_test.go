package nodestorage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"nodestorage/v2/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestDocument is a test document type that implements Cachable
type TestDocument struct {
	ID          primitive.ObjectID `bson:"_id"`
	Name        string             `bson:"name"`
	Value       int                `bson:"value"`
	VectorClock int64              `bson:"vector_clock"`
}

// Copy creates a deep copy of the document
func (d *TestDocument) Copy() *TestDocument {
	if d == nil {
		return nil
	}
	return &TestDocument{
		ID:          d.ID,
		Name:        d.Name,
		Value:       d.Value,
		VectorClock: d.VectorClock,
	}
}

// setupTestDB sets up a test MongoDB database and collection
func setupTestDB(t *testing.T) (*mongo.Client, *mongo.Collection, func()) {
	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a connection string for a local MongoDB instance
	// In a real test environment, you might use a test container or mock
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	require.NoError(t, err, "Failed to connect to MongoDB")

	// Create a unique collection name for this test
	collectionName := "test_" + primitive.NewObjectID().Hex()
	collection := client.Database("test_db").Collection(collectionName)

	// Return a cleanup function
	cleanup := func() {
		// Drop the collection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := collection.Drop(ctx)
		if err != nil {
			t.Logf("Failed to drop collection: %v", err)
		}

		// Disconnect from MongoDB
		err = client.Disconnect(ctx)
		if err != nil {
			t.Logf("Failed to disconnect from MongoDB: %v", err)
		}
	}

	return client, collection, cleanup
}

// setupTestStorage sets up a test storage with memory cache
func setupTestStorage(t *testing.T) (*StorageImpl[*TestDocument], func()) {
	// Set up test database
	_, collection, dbCleanup := setupTestDB(t)

	// Create memory cache
	memCache := cache.NewMemoryCache[*TestDocument](nil)

	// Create storage options
	options := &Options{
		VersionField: "VectorClock",
		CacheTTL:     time.Hour,
	}

	// Create storage
	ctx := context.Background()
	storage, err := NewStorage[*TestDocument](ctx, collection, memCache, options)
	require.NoError(t, err, "Failed to create storage")

	// Return a cleanup function that cleans up both the storage and the database
	cleanup := func() {
		err := storage.Close()
		if err != nil {
			t.Logf("Failed to close storage: %v", err)
		}
		dbCleanup()
	}

	return storage, cleanup
}

// insertTestDocument inserts a test document directly into the collection
func insertTestDocument(t *testing.T, collection *mongo.Collection) *TestDocument {
	ctx := context.Background()
	doc := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Test Document",
		Value:       42,
		VectorClock: 1,
	}

	_, err := collection.InsertOne(ctx, doc)
	require.NoError(t, err, "Failed to insert test document")

	return doc
}

// TestFindOne tests the FindOne method
func TestFindOne(t *testing.T) {
	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert a test document
	doc := insertTestDocument(t, storage.Collection())

	// Test FindOne
	ctx := context.Background()
	result, err := storage.FindOne(ctx, doc.ID)

	// Verify results
	assert.NoError(t, err, "FindOne should not return an error")
	assert.NotNil(t, result, "FindOne should return a document")
	assert.Equal(t, doc.ID, result.ID, "Document ID should match")
	assert.Equal(t, doc.Name, result.Name, "Document Name should match")
	assert.Equal(t, doc.Value, result.Value, "Document Value should match")
	assert.Equal(t, doc.VectorClock, result.VectorClock, "Document VectorClock should match")

	// Test FindOne with non-existent ID
	nonExistentID := primitive.NewObjectID()
	_, err = storage.FindOne(ctx, nonExistentID)
	assert.Error(t, err, "FindOne with non-existent ID should return an error")
	assert.Equal(t, ErrNotFound, err, "Error should be ErrNotFound")
}

// TestFindMany tests the FindMany method
func TestFindMany(t *testing.T) {
	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert multiple test documents
	ctx := context.Background()
	doc1 := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Document 1",
		Value:       42,
		VectorClock: 1,
	}
	doc2 := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Document 2",
		Value:       84,
		VectorClock: 1,
	}
	doc3 := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Document 3",
		Value:       126,
		VectorClock: 1,
	}

	_, err := storage.Collection().InsertMany(ctx, []interface{}{doc1, doc2, doc3})
	require.NoError(t, err, "Failed to insert test documents")

	// Test FindMany with empty filter (should return all documents)
	results, err := storage.FindMany(ctx, bson.M{})
	assert.NoError(t, err, "FindMany should not return an error")
	assert.Len(t, results, 3, "FindMany should return 3 documents")

	// Test FindMany with filter
	results, err = storage.FindMany(ctx, bson.M{"value": bson.M{"$gt": 50}})
	assert.NoError(t, err, "FindMany with filter should not return an error")
	assert.Len(t, results, 2, "FindMany with filter should return 2 documents")

	// Test FindMany with non-matching filter
	results, err = storage.FindMany(ctx, bson.M{"name": "Non-existent"})
	assert.NoError(t, err, "FindMany with non-matching filter should not return an error")
	assert.Len(t, results, 0, "FindMany with non-matching filter should return 0 documents")
}

// TestFindOneAndUpsert tests the FindOneAndUpsert method
func TestFindOneAndUpsert(t *testing.T) {
	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Test creating a new document
	ctx := context.Background()
	newDoc := &TestDocument{
		ID:    primitive.NewObjectID(),
		Name:  "New Document",
		Value: 42,
		// Note: VectorClock is not set, should be set by FindOneAndUpsert
	}

	result, err := storage.FindOneAndUpsert(ctx, newDoc)
	assert.NoError(t, err, "FindOneAndUpsert should not return an error")
	assert.NotNil(t, result, "FindOneAndUpsert should return a document")
	assert.Equal(t, newDoc.ID, result.ID, "Document ID should match")
	assert.Equal(t, newDoc.Name, result.Name, "Document Name should match")
	assert.Equal(t, newDoc.Value, result.Value, "Document Value should match")
	assert.Equal(t, int64(1), result.VectorClock, "Document VectorClock should be 1")

	// Test upserting an existing document (should return the existing document)
	existingDoc := result
	existingDoc.Name = "Updated Name" // This change should not be applied

	result, err = storage.FindOneAndUpsert(ctx, existingDoc)
	assert.NoError(t, err, "FindOneAndUpsert with existing document should not return an error")
	assert.NotNil(t, result, "FindOneAndUpsert with existing document should return a document")
	assert.Equal(t, existingDoc.ID, result.ID, "Document ID should match")
	assert.Equal(t, "New Document", result.Name, "Document Name should not be updated")
	assert.Equal(t, existingDoc.Value, result.Value, "Document Value should match")
	assert.Equal(t, int64(1), result.VectorClock, "Document VectorClock should still be 1")
}

// TestFindOneAndUpdate tests the FindOneAndUpdate method
func TestFindOneAndUpdate(t *testing.T) {
	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert a test document
	doc := insertTestDocument(t, storage.Collection())

	// Test updating a document
	ctx := context.Background()
	result, diff, err := storage.FindOneAndUpdate(ctx, doc.ID, func(d *TestDocument) (*TestDocument, error) {
		d.Name = "Updated Document"
		d.Value = 100
		return d, nil
	})

	// Verify results
	assert.NoError(t, err, "FindOneAndUpdate should not return an error")
	assert.NotNil(t, result, "FindOneAndUpdate should return a document")
	assert.Equal(t, doc.ID, result.ID, "Document ID should match")
	assert.Equal(t, "Updated Document", result.Name, "Document Name should be updated")
	assert.Equal(t, 100, result.Value, "Document Value should be updated")
	assert.Equal(t, int64(2), result.VectorClock, "Document VectorClock should be incremented")
	assert.NotNil(t, diff, "Diff should not be nil")

	// Test updating a non-existent document
	nonExistentID := primitive.NewObjectID()
	_, _, err = storage.FindOneAndUpdate(ctx, nonExistentID, func(d *TestDocument) (*TestDocument, error) {
		return d, nil
	})
	assert.Error(t, err, "FindOneAndUpdate with non-existent ID should return an error")
	assert.Equal(t, ErrNotFound, err, "Error should be ErrNotFound")

	// Test update function returning an error
	_, _, err = storage.FindOneAndUpdate(ctx, doc.ID, func(d *TestDocument) (*TestDocument, error) {
		return nil, fmt.Errorf("test error")
	})
	assert.Error(t, err, "FindOneAndUpdate with error in update function should return an error")
	assert.Contains(t, err.Error(), "test error", "Error should contain the message from the update function")
}

// TestDeleteOne tests the DeleteOne method
func TestDeleteOne(t *testing.T) {

	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert a test document
	doc := insertTestDocument(t, storage.Collection())

	// Test deleting a document
	ctx := context.Background()
	err := storage.DeleteOne(ctx, doc.ID)
	assert.NoError(t, err, "DeleteOne should not return an error")

	// Verify the document is deleted
	_, err = storage.FindOne(ctx, doc.ID)
	assert.Error(t, err, "FindOne after DeleteOne should return an error")
	assert.Equal(t, ErrNotFound, err, "Error should be ErrNotFound")

	// Test deleting a non-existent document
	nonExistentID := primitive.NewObjectID()
	err = storage.DeleteOne(ctx, nonExistentID)
	assert.NoError(t, err, "DeleteOne with non-existent ID should not return an error")
}

// TestUpdateOne tests the UpdateOne method
func TestUpdateOne(t *testing.T) {

	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert a test document
	doc := insertTestDocument(t, storage.Collection())

	// Test updating a document with MongoDB update operators
	ctx := context.Background()
	update := bson.M{
		"$set": bson.M{
			"name": "Updated with Operators",
		},
		"$inc": bson.M{
			"value": 10,
		},
	}

	result, err := storage.UpdateOne(ctx, doc.ID, update)
	assert.NoError(t, err, "UpdateOne should not return an error")
	assert.NotNil(t, result, "UpdateOne should return a document")
	assert.Equal(t, doc.ID, result.ID, "Document ID should match")
	assert.Equal(t, "Updated with Operators", result.Name, "Document Name should be updated")
	assert.Equal(t, 52, result.Value, "Document Value should be incremented")
	assert.Equal(t, int64(2), result.VectorClock, "Document VectorClock should be incremented")

	// Test updating a non-existent document
	nonExistentID := primitive.NewObjectID()
	_, err = storage.UpdateOne(ctx, nonExistentID, update)
	assert.Error(t, err, "UpdateOne with non-existent ID should return an error")
	assert.Equal(t, ErrNotFound, err, "Error should be ErrNotFound")

	// Test version conflict
	// First, get the current document to ensure it's in cache
	_, err = storage.FindOne(ctx, doc.ID)
	assert.NoError(t, err, "FindOne should not return an error")

	// Modify the document directly to simulate a concurrent update
	_, err = storage.Collection().UpdateOne(
		ctx,
		bson.M{"_id": doc.ID},
		bson.M{"$inc": bson.M{"vector_clock": 1}},
	)
	assert.NoError(t, err, "Direct update should not return an error")

	// Now try to update with the old version
	_, err = storage.UpdateOne(ctx, doc.ID, update)
	assert.Error(t, err, "UpdateOne with version conflict should return an error")
	assert.Equal(t, ErrVersionMismatch, err, "Error should be ErrVersionMismatch")
}

// TestUpdateOneWithPipeline tests the UpdateOneWithPipeline method
func TestUpdateOneWithPipeline(t *testing.T) {

	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert a test document
	doc := insertTestDocument(t, storage.Collection())

	// Test updating a document with MongoDB aggregation pipeline
	ctx := context.Background()
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			{Key: "name", Value: "Updated with Pipeline"},
			{Key: "value", Value: bson.D{{Key: "$add", Value: bson.A{"$value", 20}}}},
		}}},
	}

	result, err := storage.UpdateOneWithPipeline(ctx, doc.ID, pipeline)
	assert.NoError(t, err, "UpdateOneWithPipeline should not return an error")
	assert.NotNil(t, result, "UpdateOneWithPipeline should return a document")
	assert.Equal(t, doc.ID, result.ID, "Document ID should match")
	assert.Equal(t, "Updated with Pipeline", result.Name, "Document Name should be updated")
	assert.Equal(t, 62, result.Value, "Document Value should be incremented")
	assert.Equal(t, int64(2), result.VectorClock, "Document VectorClock should be incremented")

	// Test updating a non-existent document
	nonExistentID := primitive.NewObjectID()
	_, err = storage.UpdateOneWithPipeline(ctx, nonExistentID, pipeline)
	assert.Error(t, err, "UpdateOneWithPipeline with non-existent ID should return an error")
	assert.Equal(t, ErrNotFound, err, "Error should be ErrNotFound")

	// Test version conflict
	// First, get the current document to ensure it's in cache
	_, err = storage.FindOne(ctx, doc.ID)
	assert.NoError(t, err, "FindOne should not return an error")

	// Modify the document directly to simulate a concurrent update
	_, err = storage.Collection().UpdateOne(
		ctx,
		bson.M{"_id": doc.ID},
		bson.M{"$inc": bson.M{"vector_clock": 1}},
	)
	assert.NoError(t, err, "Direct update should not return an error")

	// Now try to update with the old version
	_, err = storage.UpdateOneWithPipeline(ctx, doc.ID, pipeline)
	assert.Error(t, err, "UpdateOneWithPipeline with version conflict should return an error")
	assert.Equal(t, ErrVersionMismatch, err, "Error should be ErrVersionMismatch")
}

// TestUpdateSection tests the UpdateSection method
func TestUpdateSection(t *testing.T) {

	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert a test document with a section
	ctx := context.Background()
	doc := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Document with Section",
		Value:       42,
		VectorClock: 1,
	}

	// Add a section to the document
	_, err := storage.Collection().InsertOne(ctx, bson.M{
		"_id":          doc.ID,
		"name":         doc.Name,
		"value":        doc.Value,
		"vector_clock": doc.VectorClock,
		"metadata": bson.M{
			"vector_clock": 1,
			"created_at":   time.Now(),
			"tags":         []string{"test", "section"},
		},
	})
	require.NoError(t, err, "Failed to insert test document with section")

	// Test updating a section
	result, err := storage.UpdateSection(ctx, doc.ID, "metadata", func(section interface{}) (interface{}, error) {
		sectionMap := section.(bson.M)
		sectionMap["updated_at"] = time.Now()
		sectionMap["tags"] = append(sectionMap["tags"].([]string), "updated")
		return sectionMap, nil
	})

	// Verify results
	assert.NoError(t, err, "UpdateSection should not return an error")
	assert.NotNil(t, result, "UpdateSection should return a document")
	assert.Equal(t, doc.ID, result.ID, "Document ID should match")

	// Get the updated document to verify the section
	updatedDoc, err := storage.FindOne(ctx, doc.ID)
	assert.NoError(t, err, "FindOne should not return an error")

	// Convert to bson.M to access the section
	docBytes, err := bson.Marshal(updatedDoc)
	assert.NoError(t, err, "Marshal should not return an error")

	var docMap bson.M
	err = bson.Unmarshal(docBytes, &docMap)
	assert.NoError(t, err, "Unmarshal should not return an error")

	// Verify the section was updated
	metadata, ok := docMap["metadata"].(bson.M)
	assert.True(t, ok, "metadata should be a bson.M")
	assert.Equal(t, int64(2), metadata["vector_clock"], "Section vector_clock should be incremented")
	assert.NotNil(t, metadata["updated_at"], "updated_at should be set")

	tags, ok := metadata["tags"].(bson.A)
	assert.True(t, ok, "tags should be a bson.A")
	assert.Len(t, tags, 3, "tags should have 3 elements")
	assert.Equal(t, "updated", tags[2], "Last tag should be 'updated'")

	// Test updating a non-existent document
	nonExistentID := primitive.NewObjectID()
	_, err = storage.UpdateSection(ctx, nonExistentID, "metadata", func(section interface{}) (interface{}, error) {
		return section, nil
	})
	assert.Error(t, err, "UpdateSection with non-existent ID should return an error")
	assert.Equal(t, ErrNotFound, err, "Error should be ErrNotFound")

	// Test update function returning an error
	_, err = storage.UpdateSection(ctx, doc.ID, "metadata", func(section interface{}) (interface{}, error) {
		return nil, fmt.Errorf("test error")
	})
	assert.Error(t, err, "UpdateSection with error in update function should return an error")
	assert.Contains(t, err.Error(), "test error", "Error should contain the message from the update function")
}

// TestWithTransaction tests the WithTransaction method
func TestWithTransaction(t *testing.T) {

	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Insert test documents
	ctx := context.Background()
	doc1 := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Document 1",
		Value:       100,
		VectorClock: 1,
	}
	doc2 := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Document 2",
		Value:       200,
		VectorClock: 1,
	}

	_, err := storage.Collection().InsertMany(ctx, []interface{}{doc1, doc2})
	require.NoError(t, err, "Failed to insert test documents")

	// Test successful transaction
	err = storage.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		// Update first document
		update1 := bson.M{
			"$set": bson.M{
				"name": "Updated Document 1",
			},
			"$inc": bson.M{
				"value": 50,
			},
		}
		_, err := storage.UpdateOne(sessCtx, doc1.ID, update1)
		if err != nil {
			return err
		}

		// Update second document
		update2 := bson.M{
			"$set": bson.M{
				"name": "Updated Document 2",
			},
			"$inc": bson.M{
				"value": -50,
			},
		}
		_, err = storage.UpdateOne(sessCtx, doc2.ID, update2)
		return err
	})

	assert.NoError(t, err, "WithTransaction should not return an error")

	// Verify both documents were updated
	updatedDoc1, err := storage.FindOne(ctx, doc1.ID)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, "Updated Document 1", updatedDoc1.Name, "Document 1 Name should be updated")
	assert.Equal(t, 150, updatedDoc1.Value, "Document 1 Value should be incremented")
	assert.Equal(t, int64(2), updatedDoc1.VectorClock, "Document 1 VectorClock should be incremented")

	updatedDoc2, err := storage.FindOne(ctx, doc2.ID)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, "Updated Document 2", updatedDoc2.Name, "Document 2 Name should be updated")
	assert.Equal(t, 150, updatedDoc2.Value, "Document 2 Value should be decremented")
	assert.Equal(t, int64(2), updatedDoc2.VectorClock, "Document 2 VectorClock should be incremented")

	// Test failed transaction
	err = storage.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
		// Update first document
		update1 := bson.M{
			"$set": bson.M{
				"name": "Should Not Be Updated",
			},
		}
		_, err := storage.UpdateOne(sessCtx, doc1.ID, update1)
		if err != nil {
			return err
		}

		// Return an error to abort the transaction
		return fmt.Errorf("test error to abort transaction")
	})

	assert.Error(t, err, "WithTransaction with error should return an error")
	assert.Contains(t, err.Error(), "test error to abort transaction", "Error should contain the message from the transaction function")

	// Verify the first document was not updated (transaction was aborted)
	updatedDoc1, err = storage.FindOne(ctx, doc1.ID)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, "Updated Document 1", updatedDoc1.Name, "Document 1 Name should not be changed")
}

// TestWatch tests the Watch method
func TestWatch(t *testing.T) {
	// Set up test storage
	storage, cleanup := setupTestStorage(t)
	defer cleanup()

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start watching for changes
	events, err := storage.Watch(ctx, nil)
	assert.NoError(t, err, "Watch should not return an error")
	assert.NotNil(t, events, "Watch should return a channel")

	// Create a channel to collect events
	receivedEvents := make(chan WatchEvent[*TestDocument], 10)

	// Start a goroutine to collect events
	go func() {
		for event := range events {
			receivedEvents <- event
		}
	}()

	// Insert a document
	doc := &TestDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Watch Test Document",
		Value:       42,
		VectorClock: 1,
	}

	_, err = storage.Collection().InsertOne(ctx, doc)
	assert.NoError(t, err, "InsertOne should not return an error")

	// Update the document
	_, err = storage.Collection().UpdateOne(
		ctx,
		bson.M{"_id": doc.ID},
		bson.M{
			"$set": bson.M{"name": "Updated Watch Test Document"},
			"$inc": bson.M{"vector_clock": 1},
		},
	)
	assert.NoError(t, err, "UpdateOne should not return an error")

	// Delete the document
	_, err = storage.Collection().DeleteOne(ctx, bson.M{"_id": doc.ID})
	assert.NoError(t, err, "DeleteOne should not return an error")

	// Wait for events
	timeout := time.After(5 * time.Second)
	eventCount := 0

	for eventCount < 3 {
		select {
		case event := <-receivedEvents:
			eventCount++
			assert.Equal(t, doc.ID, event.ID, "Event ID should match")

			switch eventCount {
			case 1:
				assert.Equal(t, "create", event.Operation, "First event should be create")
			case 2:
				assert.Equal(t, "update", event.Operation, "Second event should be update")
			case 3:
				assert.Equal(t, "delete", event.Operation, "Third event should be delete")
			}
		case <-timeout:
			t.Fatalf("Timed out waiting for events, received %d events", eventCount)
		}
	}
}
