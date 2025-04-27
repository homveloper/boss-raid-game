package v2

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// simulateNetworkLatency simulates network latency with a normal distribution
func simulateNetworkLatency(baseLatencyMs, jitterMs int) {
	// Base latency plus random jitter with normal distribution
	latency := float64(baseLatencyMs) + rand.NormFloat64()*float64(jitterMs)
	if latency < 0 {
		latency = 0
	}
	time.Sleep(time.Duration(latency) * time.Millisecond)
}

// simulateProcessingDelay simulates server-side processing delay
func simulateProcessingDelay(baseDelayMs, jitterMs int) {
	// Base delay plus random jitter with normal distribution
	delay := float64(baseDelayMs) + rand.NormFloat64()*float64(jitterMs)
	if delay < 0 {
		delay = 0
	}
	time.Sleep(time.Duration(delay) * time.Millisecond)
}

// BenchmarkRealisticConcurrencyControl benchmarks FindOneAndUpdate with different timeout and retry settings
// while simulating realistic network latency and processing delays
// go test -bench=BenchmarkRealisticConcurrencyControl -v -benchtime=1x
func BenchmarkRealisticConcurrencyControl(b *testing.B) {
	// Add debug log
	fmt.Println("Starting BenchmarkRealisticConcurrencyControl...")

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Setup MongoDB and storage
	storage, cleanup := setupBenchmarkStorage(b, "memory", 1000)
	defer cleanup()

	// Skip if storage setup failed
	if storage == nil {
		return
	}

	// Create context
	ctx := context.Background()

	// Define network and processing delay configurations
	type DelayConfig struct {
		NetworkLatencyMs      int // Base network latency in milliseconds
		NetworkLatencyJitter  int // Jitter for network latency in milliseconds
		ProcessingDelayMs     int // Base processing delay in milliseconds
		ProcessingDelayJitter int // Jitter for processing delay in milliseconds
	}

	// Define a set of configurations to test
	type Config struct {
		Concurrency int
		Timeout     int // milliseconds
		MaxRetries  int
		RetryDelay  int // milliseconds
		Delay       DelayConfig
		NumDocs     int // number of documents to create for the test
	}

	configs := []Config{
		// Single document tests
		{
			Concurrency: 1,
			Timeout:     500,
			MaxRetries:  0,
			RetryDelay:  1,
			Delay: DelayConfig{
				NetworkLatencyMs:      20,
				NetworkLatencyJitter:  10,
				ProcessingDelayMs:     10,
				ProcessingDelayJitter: 5,
			},
			NumDocs: 1,
		},
		// {
		// 	Concurrency: 4,
		// 	Timeout:     500,
		// 	MaxRetries:  0,
		// 	RetryDelay:  1,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      20,
		// 		NetworkLatencyJitter:  10,
		// 		ProcessingDelayMs:     10,
		// 		ProcessingDelayJitter: 5,
		// 	},
		// 	NumDocs: 1,
		// },
		// // Multiple documents tests
		// {
		// 	Concurrency: 8,
		// 	Timeout:     500,
		// 	MaxRetries:  0,
		// 	RetryDelay:  1,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      20,
		// 		NetworkLatencyJitter:  10,
		// 		ProcessingDelayMs:     10,
		// 		ProcessingDelayJitter: 5,
		// 	},
		// 	NumDocs: 5,
		// },
		// {
		// 	Concurrency: 8,
		// 	Timeout:     500,
		// 	MaxRetries:  0,
		// 	RetryDelay:  1,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      20,
		// 		NetworkLatencyJitter:  10,
		// 		ProcessingDelayMs:     10,
		// 		ProcessingDelayJitter: 5,
		// 	},
		// 	NumDocs: 10,
		// },
		{
			// Realistic concurrency test
			// 1 Document Using Average 20 Users
			Concurrency: 10000,
			Timeout:     500,
			MaxRetries:  0,
			RetryDelay:  1,
			Delay: DelayConfig{
				NetworkLatencyMs:      20,
				NetworkLatencyJitter:  10,
				ProcessingDelayMs:     10,
				ProcessingDelayJitter: 5,
			},
			NumDocs: 500,
		},
		{
			// Exetreme Realistic concurrency test
			// 1 Document Using Average 50 Users
			Concurrency: 100000,
			Timeout:     500,
			MaxRetries:  0,
			RetryDelay:  1,
			Delay: DelayConfig{
				NetworkLatencyMs:      20,
				NetworkLatencyJitter:  10,
				ProcessingDelayMs:     10,
				ProcessingDelayJitter: 5,
			},
			NumDocs: 2000,
		},
		// // With retries
		// {
		// 	Concurrency: 8,
		// 	Timeout:     500,
		// 	MaxRetries:  3,
		// 	RetryDelay:  10,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      20,
		// 		NetworkLatencyJitter:  10,
		// 		ProcessingDelayMs:     10,
		// 		ProcessingDelayJitter: 5,
		// 	},
		// 	NumDocs: 1,
		// },
		// {
		// 	Concurrency: 4,
		// 	Timeout:     1000,
		// 	MaxRetries:  0,
		// 	RetryDelay:  10,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      20,
		// 		NetworkLatencyJitter:  10,
		// 		ProcessingDelayMs:     10,
		// 		ProcessingDelayJitter: 5,
		// 	},
		// },
		// {
		// 	Concurrency: 4,
		// 	Timeout:     500,
		// 	MaxRetries:  0,
		// 	RetryDelay:  10,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      20,
		// 		NetworkLatencyJitter:  10,
		// 		ProcessingDelayMs:     10,
		// 		ProcessingDelayJitter: 5,
		// 	},
		// },
		// // High latency scenario
		// {
		// 	Concurrency: 4,
		// 	Timeout:     1000,
		// 	MaxRetries:  0,
		// 	RetryDelay:  50,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      50,
		// 		NetworkLatencyJitter:  20,
		// 		ProcessingDelayMs:     30,
		// 		ProcessingDelayJitter: 15,
		// 	},
		// },
		// // Low latency scenario
		// {
		// 	Concurrency: 4,
		// 	Timeout:     500,
		// 	MaxRetries:  0,
		// 	RetryDelay:  5,
		// 	Delay: DelayConfig{
		// 		NetworkLatencyMs:      5,
		// 		NetworkLatencyJitter:  2,
		// 		ProcessingDelayMs:     3,
		// 		ProcessingDelayJitter: 1,
		// 	},
		// },
	}

	// Track success rates and execution times
	type Result struct {
		Config         Config
		SuccessRate    float64
		AvgExecTimeMs  float64
		MaxExecTimeMs  float64
		FailureReasons map[string]int
	}

	var results []Result
	var resultsMutex sync.Mutex

	// Run benchmarks for each configuration
	for _, cfg := range configs {
		// Create a descriptive name for this test case
		testName := fmt.Sprintf("C=%d/T=%dms/R=%d/D=%dms/Net=%dms/Proc=%dms/Docs=%d",
			cfg.Concurrency, cfg.Timeout, cfg.MaxRetries, cfg.RetryDelay,
			cfg.Delay.NetworkLatencyMs, cfg.Delay.ProcessingDelayMs, cfg.NumDocs)

		b.Run(testName, func(b *testing.B) {
			// Set parallelism level
			b.SetParallelism(cfg.Concurrency)

			// Create documents based on the configuration
			var docIDs []primitive.ObjectID

			// Use a separate context with a longer timeout for initialization
			initCtx, initCancel := context.WithTimeout(context.Background(), time.Minute)
			defer initCancel()

			// Create the specified number of documents
			for i := 0; i < cfg.NumDocs; i++ {
				doc := createBenchDocument(1000)
				doc.Name = fmt.Sprintf("Document %d", i)
				result, err := storage.FindOneAndUpsert(initCtx, doc)
				if err != nil {
					b.Fatalf("Failed to prepare document %d: %v", i, err)
				}
				docIDs = append(docIDs, result.ID)
			}

			// Reset all documents to a known state
			for i, id := range docIDs {
				// Use direct MongoDB update to avoid potential issues with our retry logic
				_, err := storage.Collection().UpdateOne(
					initCtx,
					bson.M{"_id": id},
					bson.M{
						"$set": bson.M{
							"name":         fmt.Sprintf("Document %d", i),
							"value":        42,
							"vector_clock": 1, // Reset vector clock to a known value
						},
					},
				)
				if err != nil {
					b.Fatalf("Failed to reset document %d: %v", i, err)
				}

				// Skip cache clearing - we'll rely on direct MongoDB updates
				// Cache is private and not accessible through the Storage interface
			}

			// Fix the number of operations to exactly 1 run per test
			const fixedRuns = 50

			// Create channels to collect results
			successCh := make(chan time.Duration, fixedRuns)
			failureCh := make(chan error, fixedRuns)

			// Reset timer before the benchmark loop
			b.ResetTimer()

			// Instead of using b.RunParallel, we'll manually create goroutines
			var wg sync.WaitGroup

			// Create a semaphore to limit concurrency
			sem := make(chan struct{}, cfg.Concurrency)

			// Run exactly fixedRuns operations with cfg.Concurrency parallelism
			for i := 0; i < fixedRuns; i++ {
				wg.Add(1)
				go func() {
					// Acquire semaphore
					sem <- struct{}{}
					defer func() {
						// Release semaphore
						<-sem
					}()
					defer wg.Done()

					// Local random source for this goroutine
					localRand := rand.New(rand.NewSource(time.Now().UnixNano()))
					// Select a random document to update
					docIndex := localRand.Intn(len(docIDs))
					id := docIDs[docIndex]

					// Create edit options for this test case
					editOpts := []EditOption{
						WithTimeout(time.Millisecond * time.Duration(cfg.Timeout)),
						WithMaxRetries(cfg.MaxRetries),
						WithRetryDelay(time.Millisecond * time.Duration(cfg.RetryDelay)),
						WithMaxRetryDelay(time.Millisecond * time.Duration(cfg.RetryDelay*10)),
						WithRetryJitter(0.1),
					}

					// Simulate network latency before the request
					simulateNetworkLatency(
						cfg.Delay.NetworkLatencyMs,
						cfg.Delay.NetworkLatencyJitter,
					)

					// Measure execution time
					startTime := time.Now()

					// Perform update with simulated processing delay
					_, _, err := storage.FindOneAndUpdate(ctx, id, func(d *BenchDocument) (*BenchDocument, error) {
						// Simulate processing delay
						simulateProcessingDelay(
							cfg.Delay.ProcessingDelayMs,
							cfg.Delay.ProcessingDelayJitter,
						)

						// Simulate business logic that might cause conflicts
						currentValue := d.Value
						// Simulate some processing time that might lead to conflicts
						time.Sleep(time.Millisecond * time.Duration(localRand.Intn(5)))
						d.Value = currentValue + 1
						d.Name = fmt.Sprintf("Updated %s at %s", d.Name, time.Now().Format(time.RFC3339Nano))

						return d, nil
					}, editOpts...)

					// Simulate network latency after the request
					simulateNetworkLatency(
						cfg.Delay.NetworkLatencyMs,
						cfg.Delay.NetworkLatencyJitter,
					)

					execTime := time.Since(startTime)

					// Record result
					if err != nil {
						failureCh <- err
					} else {
						successCh <- execTime
					}
				}()
			}

			// Wait for all goroutines to complete
			wg.Wait()

			// Stop timer and collect results
			b.StopTimer()

			// Calculate statistics
			totalOps := len(successCh) + len(failureCh)
			successRate := float64(len(successCh)) / float64(totalOps)

			// Calculate average and max execution times
			var totalExecTime time.Duration
			var maxExecTime time.Duration
			successCount := len(successCh)

			// Process all successful operations
			for i := 0; i < successCount; i++ {
				execTime := <-successCh
				totalExecTime += execTime
				if execTime > maxExecTime {
					maxExecTime = execTime
				}
			}

			var avgExecTimeMs float64
			if successCount > 0 {
				avgExecTimeMs = float64(totalExecTime.Milliseconds()) / float64(successCount)
			}

			// Categorize failure reasons
			failureReasons := make(map[string]int)
			failureCount := len(failureCh)

			// Process all failures
			for i := 0; i < failureCount; i++ {
				err := <-failureCh
				reason := "unknown"
				if err != nil {
					if errors.Is(err, ErrVersionMismatch) {
						reason = "version_mismatch"
					} else if strings.Contains(err.Error(), "timeout") {
						reason = "timeout"
					} else if strings.Contains(err.Error(), "exceeded maximum retries") {
						reason = "max_retries"
					} else {
						reason = fmt.Sprintf("other : %s", err.Error())
					}
				}
				failureReasons[reason]++
			}

			// Store result
			resultsMutex.Lock()
			result := Result{
				Config:         cfg,
				SuccessRate:    successRate,
				AvgExecTimeMs:  avgExecTimeMs,
				MaxExecTimeMs:  float64(maxExecTime.Milliseconds()),
				FailureReasons: failureReasons,
			}
			results = append(results, result)
			resultsMutex.Unlock()

			// Log result
			b.Logf("Success rate: %.2f%%, Avg exec time: %.2f ms, Max exec time: %.2f ms",
				successRate*100, avgExecTimeMs, float64(maxExecTime.Milliseconds()))

			for reason, count := range failureReasons {
				b.Logf("Failure reason '%s': %d (%.2f%%)",
					reason, count, float64(count)/float64(totalOps)*100)
			}
		})
	}

	// Find and report the optimal configuration
	var optimalResult Result
	var optimalScore float64

	for _, result := range results {
		if result.Config.Concurrency == 1 {
			continue
		}

		// Calculate a score based on success rate and execution time
		// Higher success rate is better, lower execution time is better
		// Weight success rate more heavily than execution time
		score := (result.SuccessRate * 100) - (result.AvgExecTimeMs / 200)

		if score > optimalScore {
			optimalScore = score
			optimalResult = result
		}
	}

	// All Results Logging
	for _, result := range results {
		cfg := result.Config
		b.Logf("Configuration: Concurrency=%d, Timeout=%dms, MaxRetries=%d, RetryDelay=%dms",
			cfg.Concurrency, cfg.Timeout, cfg.MaxRetries, cfg.RetryDelay)
		b.Logf("Network/processing delays: Network=%dms±%dms, Processing=%dms±%dms",
			cfg.Delay.NetworkLatencyMs, cfg.Delay.NetworkLatencyJitter,
			cfg.Delay.ProcessingDelayMs, cfg.Delay.ProcessingDelayJitter)
		b.Logf("Configuration stats: Success rate=%.2f%%, Avg exec time=%.2f ms",
			result.SuccessRate*100, result.AvgExecTimeMs)
	}

	b.Logf("\n=========================================\n")

	if optimalScore > 0 {
		cfg := optimalResult.Config
		b.Logf("Optimal configuration: Concurrency=%d, Timeout=%dms, MaxRetries=%d, RetryDelay=%dms",
			cfg.Concurrency, cfg.Timeout, cfg.MaxRetries, cfg.RetryDelay)
		b.Logf("Optimal network/processing delays: Network=%dms±%dms, Processing=%dms±%dms",
			cfg.Delay.NetworkLatencyMs, cfg.Delay.NetworkLatencyJitter,
			cfg.Delay.ProcessingDelayMs, cfg.Delay.ProcessingDelayJitter)
		b.Logf("Optimal configuration stats: Success rate=%.2f%%, Avg exec time=%.2f ms",
			optimalResult.SuccessRate*100, optimalResult.AvgExecTimeMs)
	}
}

// Keep the original benchmark for comparison
// BenchmarkConcurrencyControl benchmarks FindOneAndUpdate with different timeout and retry settings
// to find the optimal configuration for optimistic concurrency control
func BenchmarkConcurrencyControl(b *testing.B) {
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

	// Define a smaller set of configurations to test
	type Config struct {
		Concurrency int
		Timeout     int // milliseconds
		MaxRetries  int
		RetryDelay  int // milliseconds
	}

	configs := []Config{
		{Concurrency: 1, Timeout: 1000, MaxRetries: 3, RetryDelay: 10},
		{Concurrency: 2, Timeout: 1000, MaxRetries: 3, RetryDelay: 10},
		{Concurrency: 4, Timeout: 1000, MaxRetries: 3, RetryDelay: 10},
		{Concurrency: 8, Timeout: 1000, MaxRetries: 3, RetryDelay: 10},
		{Concurrency: 4, Timeout: 500, MaxRetries: 3, RetryDelay: 10},
		{Concurrency: 4, Timeout: 2000, MaxRetries: 3, RetryDelay: 10},
		{Concurrency: 4, Timeout: 1000, MaxRetries: 1, RetryDelay: 10},
		{Concurrency: 4, Timeout: 1000, MaxRetries: 5, RetryDelay: 10},
		{Concurrency: 4, Timeout: 1000, MaxRetries: 3, RetryDelay: 5},
		{Concurrency: 4, Timeout: 1000, MaxRetries: 3, RetryDelay: 20},
	}

	// Track success rates and execution times
	type Result struct {
		Config         Config
		SuccessRate    float64
		AvgExecTimeMs  float64
		MaxExecTimeMs  float64
		FailureReasons map[string]int
	}

	var results []Result

	// Run benchmarks for each configuration
	for _, cfg := range configs {
		// Create a descriptive name for this test case
		testName := fmt.Sprintf("C=%d/T=%dms/R=%d/D=%dms",
			cfg.Concurrency, cfg.Timeout, cfg.MaxRetries, cfg.RetryDelay)

		b.Run(testName, func(b *testing.B) {
			// Set parallelism level
			b.SetParallelism(cfg.Concurrency)

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

			// Limit the number of operations to make the test faster
			b.N = 100

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
						WithTimeout(time.Millisecond * time.Duration(cfg.Timeout)),
						WithMaxRetries(cfg.MaxRetries),
						WithRetryDelay(time.Millisecond * time.Duration(cfg.RetryDelay)),
						WithMaxRetryDelay(time.Millisecond * time.Duration(cfg.RetryDelay*10)),
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
			successCount := len(successCh)

			// Process all successful operations
			for i := 0; i < successCount; i++ {
				execTime := <-successCh
				totalExecTime += execTime
				if execTime > maxExecTime {
					maxExecTime = execTime
				}
			}

			var avgExecTimeMs float64
			if successCount > 0 {
				avgExecTimeMs = float64(totalExecTime.Milliseconds()) / float64(successCount)
			}

			// Categorize failure reasons
			failureReasons := make(map[string]int)
			failureCount := len(failureCh)

			// Process all failures
			for i := 0; i < failureCount; i++ {
				err := <-failureCh
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
			}

			// Store result
			result := Result{
				Config:         cfg,
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
		cfg := optimalResult.Config
		b.Logf("Optimal configuration: Concurrency=%d, Timeout=%dms, MaxRetries=%d, RetryDelay=%dms",
			cfg.Concurrency, cfg.Timeout, cfg.MaxRetries, cfg.RetryDelay)
		b.Logf("Optimal configuration stats: Success rate=%.2f%%, Avg exec time=%.2f ms",
			optimalResult.SuccessRate*100, optimalResult.AvgExecTimeMs)
	}
}
