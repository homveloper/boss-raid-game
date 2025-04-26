package cache

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BenchDocument is a document type for benchmarking
type BenchDocument struct {
	ID    primitive.ObjectID
	Name  string
	Value int
	Data  []byte // Variable size data for testing different document sizes
}

// setupBenchmarkCache prepares cache implementations for benchmarking
func setupBenchmarkCache(b *testing.B, cacheType string, docSize int) (Cache[*BenchDocument], func()) {
	// Create context
	ctx := context.Background()

	// Create a document with specified size
	doc := createBenchDocument(docSize)

	switch cacheType {
	case "memory":
		// Create memory cache
		cache := NewMemoryCache[*BenchDocument](nil)

		// Warm up cache
		for i := 0; i < 1000; i++ {
			id := primitive.NewObjectID()
			doc.ID = id
			err := cache.Set(ctx, id, doc, 0)
			if err != nil {
				b.Fatalf("Failed to warm up memory cache: %v", err)
			}
		}

		cleanup := func() {
			cache.Close()
		}

		return cache, cleanup

	case "badger":
		// Create temporary directory for BadgerDB
		tempDir, err := os.MkdirTemp("", "badger-bench-*")
		if err != nil {
			b.Fatalf("Failed to create temporary directory: %v", err)
		}

		// Open BadgerDB with benchmark-optimized options
		opts := badger.DefaultOptions(tempDir)
		opts.Logger = nil
		opts.SyncWrites = false
		opts.NumVersionsToKeep = 1

		cache, err := NewBadgerCache[*BenchDocument](tempDir, nil)
		if err != nil {
			os.RemoveAll(tempDir)
			b.Fatalf("Failed to create BadgerDB cache: %v", err)
		}

		// Warm up cache
		for i := 0; i < 1000; i++ {
			id := primitive.NewObjectID()
			doc.ID = id
			err := cache.Set(ctx, id, doc, 0)
			if err != nil {
				cache.Close()
				os.RemoveAll(tempDir)
				b.Fatalf("Failed to warm up BadgerDB cache: %v", err)
			}
		}

		cleanup := func() {
			cache.Close()
			os.RemoveAll(tempDir)
		}

		return cache, cleanup

	case "redis":
		// Check if Redis is available
		redisAddr := os.Getenv("REDIS_ADDR")
		if redisAddr == "" {
			redisAddr = "localhost:6379"
		}

		// Create Redis cache
		cache, err := NewRedisCache[*BenchDocument](redisAddr, nil)
		if err != nil {
			b.Skipf("Redis not available: %v", err)
			return nil, func() {}
		}

		// Set unique prefix for this benchmark
		cache.prefix = "bench:" + primitive.NewObjectID().Hex() + ":"

		// Warm up cache
		for i := 0; i < 1000; i++ {
			id := primitive.NewObjectID()
			doc.ID = id
			err := cache.Set(ctx, id, doc, 0)
			if err != nil {
				cache.Close()
				b.Fatalf("Failed to warm up Redis cache: %v", err)
			}
		}

		cleanup := func() {
			cache.Clear(ctx)
			cache.Close()
		}

		return cache, cleanup

	default:
		b.Fatalf("Unknown cache type: %s", cacheType)
		return nil, nil
	}
}

// createBenchDocument creates a document with the specified size
func createBenchDocument(size int) *BenchDocument {
	doc := &BenchDocument{
		ID:    primitive.NewObjectID(),
		Name:  "Benchmark Document",
		Value: 42,
		Data:  make([]byte, size),
	}

	// Fill data with some pattern
	for i := 0; i < size; i++ {
		doc.Data[i] = byte(i % 256)
	}

	return doc
}

// runCacheBenchmark runs a benchmark for the specified cache type and operation
func runCacheBenchmark(b *testing.B, cacheType, operation string, docSize int) {
	cache, cleanup := setupBenchmarkCache(b, cacheType, docSize)
	defer cleanup()

	// Skip if cache setup failed (e.g., Redis not available)
	if cache == nil {
		return
	}

	// Create context
	ctx := context.Background()

	// Create a document with specified size
	doc := createBenchDocument(docSize)

	// Reset timer before the benchmark loop
	b.ResetTimer()

	switch operation {
	case "set":
		// Benchmark Set operation
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				id := primitive.NewObjectID()
				doc.ID = id
				err := cache.Set(ctx, id, doc, 0)
				if err != nil {
					b.Fatalf("Failed to set document: %v", err)
				}
			}
		})

	case "get-hit":
		// Prepare a document for Get hit
		id := primitive.NewObjectID()
		doc.ID = id
		err := cache.Set(ctx, id, doc, 0)
		if err != nil {
			b.Fatalf("Failed to prepare document for Get hit: %v", err)
		}

		// Benchmark Get operation (cache hit)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, err := cache.Get(ctx, id)
				if err != nil {
					b.Fatalf("Failed to get document: %v", err)
				}
			}
		})

	case "get-miss":
		// Benchmark Get operation (cache miss)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				id := primitive.NewObjectID() // Non-existent ID
				_, err := cache.Get(ctx, id)
				if err == nil {
					b.Fatalf("Expected cache miss, got hit")
				}
			}
		})

	case "delete":
		// Benchmark Delete operation
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Prepare a document to delete
				id := primitive.NewObjectID()
				doc.ID = id
				err := cache.Set(ctx, id, doc, 0)
				if err != nil {
					b.Fatalf("Failed to prepare document for Delete: %v", err)
				}

				// Delete the document
				err = cache.Delete(ctx, id)
				if err != nil {
					b.Fatalf("Failed to delete document: %v", err)
				}
			}
		})

	case "set-get":
		// Benchmark Set followed by Get
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				id := primitive.NewObjectID()
				doc.ID = id

				// Set the document
				err := cache.Set(ctx, id, doc, 0)
				if err != nil {
					b.Fatalf("Failed to set document: %v", err)
				}

				// Get the document
				_, err = cache.Get(ctx, id)
				if err != nil {
					b.Fatalf("Failed to get document: %v", err)
				}
			}
		})

	default:
		b.Fatalf("Unknown operation: %s", operation)
	}
}

// BenchmarkMemoryCache benchmarks the memory cache
func BenchmarkMemoryCache(b *testing.B) {
	// Document sizes to test
	sizes := []int{100, 1000, 10000}

	// Operations to test
	operations := []string{"set", "get-hit", "get-miss", "delete", "set-get"}

	for _, size := range sizes {
		for _, op := range operations {
			b.Run(fmt.Sprintf("Size=%d/Op=%s", size, op), func(b *testing.B) {
				runCacheBenchmark(b, "memory", op, size)
			})
		}
	}
}

// BenchmarkBadgerCache benchmarks the BadgerDB cache
func BenchmarkBadgerCache(b *testing.B) {
	// Document sizes to test
	sizes := []int{100, 1000, 10000}

	// Operations to test
	operations := []string{"set", "get-hit", "get-miss", "delete", "set-get"}

	for _, size := range sizes {
		for _, op := range operations {
			b.Run(fmt.Sprintf("Size=%d/Op=%s", size, op), func(b *testing.B) {
				runCacheBenchmark(b, "badger", op, size)
			})
		}
	}
}

// BenchmarkRedisCache benchmarks the Redis cache
func BenchmarkRedisCache(b *testing.B) {
	// Check if Redis is available
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Create Redis cache to check availability
	cache, err := NewRedisCache[*BenchDocument](redisAddr, nil)
	if err != nil {
		b.Skipf("Redis not available: %v", err)
		return
	}
	cache.Close()

	// Document sizes to test
	sizes := []int{100, 1000, 10000}

	// Operations to test
	operations := []string{"set", "get-hit", "get-miss", "delete", "set-get"}

	for _, size := range sizes {
		for _, op := range operations {
			b.Run(fmt.Sprintf("Size=%d/Op=%s", size, op), func(b *testing.B) {
				runCacheBenchmark(b, "redis", op, size)
			})
		}
	}
}

// BenchmarkCacheComparison compares all cache implementations
func BenchmarkCacheComparison(b *testing.B) {
	// Cache types to test
	cacheTypes := []string{"memory", "badger"}

	// Check if Redis is available
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	cache, err := NewRedisCache[*BenchDocument](redisAddr, nil)
	if err == nil {
		cache.Close()
		cacheTypes = append(cacheTypes, "redis")
	}

	// Document size for comparison
	size := 1000

	// Operations to test
	operations := []string{"set", "get-hit", "get-miss", "delete", "set-get"}

	for _, op := range operations {
		for _, cacheType := range cacheTypes {
			b.Run(fmt.Sprintf("Cache=%s/Op=%s", cacheType, op), func(b *testing.B) {
				runCacheBenchmark(b, cacheType, op, size)
			})
		}
	}
}
