package v2

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"nodestorage/v2/cache"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// BenchDocument is a document type for benchmarking
type BenchDocument struct {
	ID          primitive.ObjectID `bson:"_id"`
	Name        string             `bson:"name"`
	Value       int                `bson:"value"`
	Tags        []string           `bson:"tags"`
	Data        []byte             `bson:"data"` // Variable size data for testing different document sizes
	VectorClock int64              `bson:"vector_clock"`
}

// Copy creates a deep copy of the document
func (d *BenchDocument) Copy() *BenchDocument {
	if d == nil {
		return nil
	}

	// Copy tags
	tagsCopy := make([]string, len(d.Tags))
	copy(tagsCopy, d.Tags)

	// Copy data
	dataCopy := make([]byte, len(d.Data))
	copy(dataCopy, d.Data)

	return &BenchDocument{
		ID:          d.ID,
		Name:        d.Name,
		Value:       d.Value,
		Tags:        tagsCopy,
		Data:        dataCopy,
		VectorClock: d.VectorClock,
	}
}

// setupBenchmarkDB sets up a MongoDB database for benchmarking
func setupBenchmarkDB(b *testing.B) (*mongo.Client, *mongo.Collection, func()) {
	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a connection string for a local MongoDB instance
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		b.Skipf("MongoDB not available: %v", err)
		return nil, nil, func() {}
	}

	// Ping MongoDB to ensure it's responsive
	err = client.Ping(ctx, nil)
	if err != nil {
		b.Skipf("MongoDB not responsive: %v", err)
		return nil, nil, func() {}
	}

	// Create a unique collection name for this benchmark
	collectionName := "bench_" + primitive.NewObjectID().Hex()
	collection := client.Database("test_db").Collection(collectionName)

	// Return a cleanup function
	cleanup := func() {
		// Drop the collection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		collection.Drop(ctx)

		// Disconnect from MongoDB
		client.Disconnect(ctx)
	}

	return client, collection, cleanup
}

// setupBenchmarkStorage sets up a storage instance for benchmarking
func setupBenchmarkStorage(b *testing.B, cacheType string, docSize int) (*StorageImpl[*BenchDocument], func()) {
	// Set up MongoDB
	client, collection, dbCleanup := setupBenchmarkDB(b)
	if client == nil {
		return nil, func() {}
	}

	// Create cache based on type
	var cacheImpl cache.Cache[*BenchDocument]
	var cacheCleanup func()

	switch cacheType {
	case "none":
		// No cache
		cacheImpl = nil
		cacheCleanup = func() {}

	case "memory":
		// Memory cache
		cacheImpl = cache.NewMemoryCache[*BenchDocument](nil)
		cacheCleanup = func() {
			cacheImpl.Close()
		}

	case "badger":
		// BadgerDB cache
		tempDir, err := os.MkdirTemp("", "badger-bench-*")
		if err != nil {
			b.Fatalf("Failed to create temporary directory: %v", err)
		}

		badgerCache, err := cache.NewBadgerCache[*BenchDocument](tempDir, nil)
		if err != nil {
			os.RemoveAll(tempDir)
			b.Fatalf("Failed to create BadgerDB cache: %v", err)
		}

		cacheImpl = badgerCache
		cacheCleanup = func() {
			badgerCache.Close()
			os.RemoveAll(tempDir)
		}

	case "redis":
		// Redis cache
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "localhost:6379"
		}

		redisCache, err := cache.NewRedisCache[*BenchDocument](redisAddr, nil)
		if err != nil {
			b.Skipf("Redis not available: %v", err)
			return nil, func() {}
		}

		// We can't set prefix directly as it's unexported
		// Use a unique key prefix for benchmarking

		cacheImpl = redisCache
		cacheCleanup = func() {
			ctx := context.Background()
			redisCache.Clear(ctx)
			redisCache.Close()
		}

	default:
		b.Fatalf("Unknown cache type: %s", cacheType)
		return nil, nil
	}

	// Create storage options
	options := &Options{
		VersionField: "vector_clock",
		CacheTTL:     time.Hour,
	}

	// Create storage
	ctx := context.Background()
	storage, err := NewStorage[*BenchDocument](ctx, client, collection, cacheImpl, options)
	if err != nil {
		if cacheImpl != nil {
			cacheCleanup()
		}
		dbCleanup()
		b.Fatalf("Failed to create storage: %v", err)
	}

	// Create cleanup function
	cleanup := func() {
		storage.Close()
		if cacheImpl != nil {
			cacheCleanup()
		}
		dbCleanup()
	}

	return storage, cleanup
}

// createBenchDocument creates a document with the specified size
func createBenchDocument(size int) *BenchDocument {
	// Create tags
	tags := []string{"benchmark", "test", "performance"}

	// Create data
	data := make([]byte, size)
	for i := 0; i < size; i++ {
		data[i] = byte(i % 256)
	}

	// Create document
	doc := &BenchDocument{
		ID:          primitive.NewObjectID(),
		Name:        "Benchmark Document",
		Value:       42,
		Tags:        tags,
		Data:        data,
		VectorClock: 0, // Will be set by storage
	}

	return doc
}

// runStorageBenchmark runs a benchmark for the specified storage configuration and operation
func runStorageBenchmark(b *testing.B, cacheType, operation string, docSize int) {
	storage, cleanup := setupBenchmarkStorage(b, cacheType, docSize)
	defer cleanup()

	// Skip if storage setup failed
	if storage == nil {
		return
	}

	// Create context
	ctx := context.Background()

	// Create a document with specified size
	doc := createBenchDocument(docSize)

	// Reset timer before the benchmark loop
	b.ResetTimer()

	switch operation {
	case "find-one":
		// Prepare a document for FindOne
		result, err := storage.FindOneAndUpsert(ctx, doc)
		if err != nil {
			b.Fatalf("Failed to prepare document for FindOne: %v", err)
		}
		id := result.ID

		// Benchmark FindOne operation
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := storage.FindOne(ctx, id)
				if err != nil {
					b.Fatalf("Failed to find document: %v", err)
				}
			}
		})

	case "find-many":
		// Prepare documents for FindMany
		numDocs := 100
		for i := 0; i < numDocs; i++ {
			doc := createBenchDocument(docSize / 10) // Smaller documents for FindMany
			doc.Value = i
			_, err := storage.FindOneAndUpsert(ctx, doc)
			if err != nil {
				b.Fatalf("Failed to prepare document for FindMany: %v", err)
			}
		}

		// Benchmark FindMany operation
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := storage.FindMany(ctx, bson.M{"value": bson.M{"$gte": 0}})
				if err != nil {
					b.Fatalf("Failed to find documents: %v", err)
				}
			}
		})

	case "find-one-and-upsert":
		// Benchmark FindOneAndUpsert operation
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				newDoc := createBenchDocument(docSize)
				_, err := storage.FindOneAndUpsert(ctx, newDoc)
				if err != nil {
					b.Fatalf("Failed to upsert document: %v", err)
				}
			}
		})

	case "find-one-and-update":
		// Prepare a document for FindOneAndUpdate
		result, err := storage.FindOneAndUpsert(ctx, doc)
		if err != nil {
			b.Fatalf("Failed to prepare document for FindOneAndUpdate: %v", err)
		}
		id := result.ID

		// Benchmark FindOneAndUpdate operation
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _, err := storage.FindOneAndUpdate(ctx, id, func(d *BenchDocument) (*BenchDocument, error) {
					d.Value++
					d.Name = "Updated " + d.Name
					return d, nil
				})
				if err != nil {
					b.Fatalf("Failed to update document: %v", err)
				}
			}
		})

	case "update-one":
		// Prepare a document for UpdateOne
		result, err := storage.FindOneAndUpsert(ctx, doc)
		if err != nil {
			b.Fatalf("Failed to prepare document for UpdateOne: %v", err)
		}
		id := result.ID

		// Benchmark UpdateOne operation
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				i++
				_, err := storage.UpdateOne(ctx, id, bson.M{
					"$set": bson.M{
						"name":  fmt.Sprintf("Updated %d", i),
						"value": 42 + i,
					},
				})
				if err != nil {
					b.Fatalf("Failed to update document: %v", err)
				}
			}
		})

	case "update-section":
		// Prepare a document with a section for UpdateSection
		sectionDoc := &BenchDocument{
			ID:    primitive.NewObjectID(),
			Name:  "Section Test",
			Value: 42,
			Tags:  []string{"section", "test"},
			Data:  make([]byte, docSize/2),
		}

		// Insert document with metadata section
		_, err := storage.Collection().InsertOne(ctx, bson.M{
			"_id":          sectionDoc.ID,
			"name":         sectionDoc.Name,
			"value":        sectionDoc.Value,
			"tags":         sectionDoc.Tags,
			"data":         sectionDoc.Data,
			"vector_clock": 1,
			"metadata": bson.M{
				"vector_clock": 1,
				"created_at":   time.Now(),
				"updated_at":   time.Now(),
				"version":      "1.0",
				"status":       "active",
			},
		})
		if err != nil {
			b.Fatalf("Failed to prepare document for UpdateSection: %v", err)
		}

		// Benchmark UpdateSection operation
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				i++
				_, err := storage.UpdateSection(ctx, sectionDoc.ID, "metadata", func(section interface{}) (interface{}, error) {
					metadata := section.(bson.M)
					metadata["updated_at"] = time.Now()
					metadata["version"] = fmt.Sprintf("1.%d", i)
					metadata["status"] = fmt.Sprintf("active-%d", i)
					return metadata, nil
				})
				if err != nil {
					b.Fatalf("Failed to update section: %v", err)
				}
			}
		})

	case "delete-one":
		// Benchmark DeleteOne operation
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Create a document to delete
				newDoc := createBenchDocument(docSize)
				result, err := storage.FindOneAndUpsert(ctx, newDoc)
				if err != nil {
					b.Fatalf("Failed to create document for DeleteOne: %v", err)
				}

				// Delete the document
				err = storage.DeleteOne(ctx, result.ID)
				if err != nil {
					b.Fatalf("Failed to delete document: %v", err)
				}
			}
		})

	default:
		b.Fatalf("Unknown operation: %s", operation)
	}
}

// BenchmarkStorageWithNoCache benchmarks storage without cache
func BenchmarkStorageWithNoCache(b *testing.B) {
	// Document sizes to test
	sizes := []int{100, 1000, 10000}

	// Operations to test
	operations := []string{
		"find-one",
		"find-many",
		"find-one-and-upsert",
		"find-one-and-update",
		"update-one",
		"update-section",
		"delete-one",
	}

	for _, size := range sizes {
		for _, op := range operations {
			b.Run(fmt.Sprintf("Size=%d/Op=%s", size, op), func(b *testing.B) {
				runStorageBenchmark(b, "none", op, size)
			})
		}
	}
}

// BenchmarkStorageWithMemoryCache benchmarks storage with memory cache
func BenchmarkStorageWithMemoryCache(b *testing.B) {
	// Document sizes to test
	sizes := []int{100, 1000, 10000}

	// Operations to test
	operations := []string{
		"find-one",
		"find-many",
		"find-one-and-upsert",
		"find-one-and-update",
		"update-one",
		"update-section",
		"delete-one",
	}

	for _, size := range sizes {
		for _, op := range operations {
			b.Run(fmt.Sprintf("Size=%d/Op=%s", size, op), func(b *testing.B) {
				runStorageBenchmark(b, "memory", op, size)
			})
		}
	}
}

// BenchmarkStorageWithBadgerCache benchmarks storage with BadgerDB cache
func BenchmarkStorageWithBadgerCache(b *testing.B) {
	// Document sizes to test
	sizes := []int{100, 1000, 10000}

	// Operations to test
	operations := []string{
		"find-one",
		"find-many",
		"find-one-and-upsert",
		"find-one-and-update",
		"update-one",
		"update-section",
		"delete-one",
	}

	for _, size := range sizes {
		for _, op := range operations {
			b.Run(fmt.Sprintf("Size=%d/Op=%s", size, op), func(b *testing.B) {
				runStorageBenchmark(b, "badger", op, size)
			})
		}
	}
}

// BenchmarkStorageWithRedisCache benchmarks storage with Redis cache
func BenchmarkStorageWithRedisCache(b *testing.B) {
	// Check if Redis is available
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisCache, err := cache.NewRedisCache[*BenchDocument](redisAddr, nil)
	if err != nil {
		b.Skipf("Redis not available: %v", err)
		return
	}
	redisCache.Close()

	// Document sizes to test
	sizes := []int{100, 1000, 10000}

	// Operations to test
	operations := []string{
		"find-one",
		"find-many",
		"find-one-and-upsert",
		"find-one-and-update",
		"update-one",
		"update-section",
		"delete-one",
	}

	for _, size := range sizes {
		for _, op := range operations {
			b.Run(fmt.Sprintf("Size=%d/Op=%s", size, op), func(b *testing.B) {
				runStorageBenchmark(b, "redis", op, size)
			})
		}
	}
}

// BenchmarkStorageCacheComparison compares storage with different cache implementations
func BenchmarkStorageCacheComparison(b *testing.B) {
	// Cache types to test
	cacheTypes := []string{"none", "memory", "badger"}

	// Check if Redis is available
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisCache, err := cache.NewRedisCache[*BenchDocument](redisAddr, nil)
	if err == nil {
		redisCache.Close()
		cacheTypes = append(cacheTypes, "redis")
	}

	// Document size for comparison
	size := 1000

	// Operations to test
	operations := []string{
		"find-one",
		"find-one-and-update",
		"update-one",
	}

	for _, op := range operations {
		for _, cacheType := range cacheTypes {
			b.Run(fmt.Sprintf("Cache=%s/Op=%s", cacheType, op), func(b *testing.B) {
				runStorageBenchmark(b, cacheType, op, size)
			})
		}
	}
}
