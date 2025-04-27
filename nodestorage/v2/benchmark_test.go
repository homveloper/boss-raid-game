package v2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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
	// Add debug log
	fmt.Println("Setting up MongoDB database for benchmarking...")

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use a connection string for a local MongoDB instance
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	fmt.Printf("Using MongoDB URI: %s\n", mongoURI)

	clientOptions := options.Client().ApplyURI(mongoURI)
	fmt.Println("Connecting to MongoDB...")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		fmt.Printf("MongoDB connection failed: %v\n", err)
		b.Skipf("MongoDB not available: %v", err)
		return nil, nil, func() {}
	}
	fmt.Println("MongoDB connection established, pinging server...")

	// Ping MongoDB to ensure it's responsive
	err = client.Ping(ctx, nil)
	if err != nil {
		fmt.Printf("MongoDB ping failed: %v\n", err)
		b.Skipf("MongoDB not responsive: %v", err)
		return nil, nil, func() {}
	}
	fmt.Println("MongoDB ping successful")

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
	return setupBenchmarkStorageWithOptions(b, cacheType, docSize, false)
}

// setupBenchmarkStorageWithOptions sets up a storage instance for benchmarking with additional options
func setupBenchmarkStorageWithOptions(b *testing.B, cacheType string, docSize int, hotDataWatcherEnabled bool) (*StorageImpl[*BenchDocument], func()) {
	// Add debug log
	fmt.Printf("Setting up benchmark storage with cache type: %s, docSize: %d, hotDataWatcherEnabled: %v\n",
		cacheType, docSize, hotDataWatcherEnabled)

	// Set up MongoDB
	client, collection, dbCleanup := setupBenchmarkDB(b)
	if client == nil {
		fmt.Println("Failed to set up MongoDB client")
		return nil, func() {}
	}
	fmt.Println("MongoDB client set up successfully")

	// Create cache based on type
	var cacheImpl cache.Cache[*BenchDocument]
	var cacheCleanup func()

	switch cacheType {
	case "none":
		// No cache
		cacheImpl = cache.NewMemoryCache[*BenchDocument](nil)
		cacheCleanup = func() {
			cacheImpl.Close()
		}

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
		VersionField:          "VectorClock",
		CacheTTL:              time.Hour,
		HotDataWatcherEnabled: hotDataWatcherEnabled,
		HotDataMaxItems:       100,
		HotDataWatchInterval:  time.Second * 5, // Shorter interval for benchmarks
		HotDataDecayInterval:  time.Minute,
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
				// Use a longer timeout for FindOneAndUpdate in benchmarks
				_, _, err := storage.FindOneAndUpdate(ctx, id, func(d *BenchDocument) (*BenchDocument, error) {
					d.Value++
					d.Name = "Updated " + d.Name
					return d, nil
				}, WithTimeout(time.Minute), WithMaxRetries(3))
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
				time.Sleep(time.Millisecond * 100)

				i++
				// Use a longer timeout for UpdateOne in benchmarks
				_, err := storage.UpdateOne(ctx, id, bson.M{
					"$set": bson.M{
						"name":  fmt.Sprintf("Updated %d", i),
						"value": 42 + i,
					},
				}, WithTimeout(time.Minute), WithMaxRetries(3))
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
				// Use a longer timeout for UpdateSection in benchmarks
				_, err := storage.UpdateSection(ctx, sectionDoc.ID, "metadata", func(section interface{}) (interface{}, error) {
					metadata := section.(bson.M)
					metadata["updated_at"] = time.Now()
					metadata["version"] = fmt.Sprintf("1.%d", i)
					metadata["status"] = fmt.Sprintf("active-%d", i)
					return metadata, nil
				}, WithTimeout(time.Minute), WithMaxRetries(3))
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

// BenchmarkOptimalConcurrencyControl benchmarks FindOneAndUpdate with different timeout and retry settings
// to find the optimal configuration for optimistic concurrency control
func BenchmarkOptimalConcurrencyControl(b *testing.B) {
	// Setup MongoDB and storage
	storage, cleanup := setupBenchmarkStorage(b, "memory", 1000)
	defer cleanup()

	// Skip if storage setup failed
	if storage == nil {
		return
	}

	// Create context
	ctx := context.Background()

	// Create a document to update
	doc := createBenchDocument(1000)
	result, err := storage.FindOneAndUpsert(ctx, doc)
	if err != nil {
		b.Fatalf("Failed to prepare document: %v", err)
	}
	id := result.ID

	// Define different concurrency levels to test
	concurrencyLevels := []int{1, 2, 4, 8, 16, 32}

	// Define different timeout settings to test (in milliseconds)
	timeouts := []int{100, 500, 1000, 5000, 10000}

	// Define different retry settings to test
	retries := []int{1, 3, 5, 10, 0} // 0 means unlimited retries

	// Define different retry delay settings to test (in milliseconds)
	retryDelays := []int{5, 10, 50, 100}

	// Track success rates and execution times
	type Result struct {
		Concurrency    int
		Timeout        int
		MaxRetries     int
		RetryDelay     int
		SuccessRate    float64
		AvgExecTimeMs  float64
		MaxExecTimeMs  float64
		FailureReasons map[string]int
	}

	var results []Result

	// Run benchmarks for different combinations
	for _, concurrency := range concurrencyLevels {
		for _, timeout := range timeouts {
			for _, maxRetries := range retries {
				for _, retryDelay := range retryDelays {
					// Skip some combinations to reduce test time
					if concurrency > 8 && timeout > 1000 && maxRetries > 3 {
						continue
					}

					// Create a descriptive name for this test case
					testName := fmt.Sprintf("C=%d/T=%dms/R=%d/D=%dms",
						concurrency, timeout, maxRetries, retryDelay)

					b.Run(testName, func(b *testing.B) {
						// Set parallelism level
						b.SetParallelism(concurrency)

						// Reset the document to a known state before each test
						_, err := storage.UpdateOne(ctx, id, bson.M{
							"$set": bson.M{
								"name":  "Benchmark Document",
								"value": 42,
							},
						}, WithTimeout(time.Second*10))
						if err != nil {
							b.Fatalf("Failed to reset document: %v", err)
						}

						// Create channels to collect results
						successCh := make(chan time.Duration, b.N)
						failureCh := make(chan error, b.N)

						// Reset timer before the benchmark loop
						b.ResetTimer()

						// Run the benchmark
						b.RunParallel(func(pb *testing.PB) {
							for pb.Next() {
								// Create edit options for this test case
								editOpts := []EditOption{
									WithTimeout(time.Millisecond * time.Duration(timeout)),
									WithMaxRetries(maxRetries),
									WithRetryDelay(time.Millisecond * time.Duration(retryDelay)),
									WithMaxRetryDelay(time.Millisecond * time.Duration(retryDelay*10)),
									WithRetryJitter(0.1),
								}

								// Measure execution time
								startTime := time.Now()

								// Perform update
								_, _, err := storage.FindOneAndUpdate(ctx, id, func(d *BenchDocument) (*BenchDocument, error) {
									d.Value++
									d.Name = "Updated " + d.Name
									return d, nil
								}, editOpts...)

								execTime := time.Since(startTime)

								// Record result
								if err != nil {
									failureCh <- err
								} else {
									successCh <- execTime
								}
							}
						})

						// Stop timer and collect results
						b.StopTimer()

						// Calculate statistics
						totalOps := len(successCh) + len(failureCh)
						successRate := float64(len(successCh)) / float64(totalOps)

						// Calculate average and max execution times
						var totalExecTime time.Duration
						var maxExecTime time.Duration
						for execTime := range successCh {
							totalExecTime += execTime
							if execTime > maxExecTime {
								maxExecTime = execTime
							}
							if len(successCh) == 0 {
								break
							}
						}

						var avgExecTimeMs float64
						if len(successCh) > 0 {
							avgExecTimeMs = float64(totalExecTime.Milliseconds()) / float64(len(successCh))
						}

						// Categorize failure reasons
						failureReasons := make(map[string]int)
						for err := range failureCh {
							reason := "unknown"
							if err != nil {
								if errors.Is(err, ErrVersionMismatch) {
									reason = "version_mismatch"
								} else if strings.Contains(err.Error(), "timeout") {
									reason = "timeout"
								} else if strings.Contains(err.Error(), "exceeded maximum retries") {
									reason = "max_retries"
								} else {
									reason = "other"
								}
							}
							failureReasons[reason]++
							if len(failureCh) == 0 {
								break
							}
						}

						// Store result
						result := Result{
							Concurrency:    concurrency,
							Timeout:        timeout,
							MaxRetries:     maxRetries,
							RetryDelay:     retryDelay,
							SuccessRate:    successRate,
							AvgExecTimeMs:  avgExecTimeMs,
							MaxExecTimeMs:  float64(maxExecTime.Milliseconds()),
							FailureReasons: failureReasons,
						}
						results = append(results, result)

						// Log result
						b.Logf("Success rate: %.2f%%, Avg exec time: %.2f ms, Max exec time: %.2f ms",
							successRate*100, avgExecTimeMs, float64(maxExecTime.Milliseconds()))

						for reason, count := range failureReasons {
							b.Logf("Failure reason '%s': %d (%.2f%%)",
								reason, count, float64(count)/float64(totalOps)*100)
						}
					})
				}
			}
		}
	}

	// Find and report the optimal configuration
	var optimalResult Result
	var optimalScore float64

	for _, result := range results {
		// Calculate a score based on success rate and execution time
		// Higher success rate is better, lower execution time is better
		score := result.SuccessRate*100 - (result.AvgExecTimeMs / 100)

		if score > optimalScore {
			optimalScore = score
			optimalResult = result
		}
	}

	if optimalScore > 0 {
		b.Logf("Optimal configuration: Concurrency=%d, Timeout=%dms, MaxRetries=%d, RetryDelay=%dms",
			optimalResult.Concurrency, optimalResult.Timeout,
			optimalResult.MaxRetries, optimalResult.RetryDelay)
		b.Logf("Optimal configuration stats: Success rate=%.2f%%, Avg exec time=%.2f ms",
			optimalResult.SuccessRate*100, optimalResult.AvgExecTimeMs)
	}
}
