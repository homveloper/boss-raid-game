package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtmonitor"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Create a context that will be canceled on SIGINT or SIGTERM
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("Received signal, shutting down...")
		cancel()
	}()

	// Create Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Change this to your Redis server address
		Password: "",               // No password by default
		DB:       0,                // Default DB
	})

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create Redis PubSub options
	pubsubOptions := crdtpubsub.NewOptions()
	pubsubOptions.DefaultFormat = crdtpubsub.EncodingFormatJSON

	// Create Redis PubSub with the client
	redisPubSub, err := crdtpubsub.NewRedisPubSub(redisClient, pubsubOptions)
	if err != nil {
		log.Fatalf("Failed to create Redis PubSub: %v", err)
	}
	defer redisPubSub.Close()

	// Create CRDT document
	doc := crdt.NewDocument(common.NewSessionID())

	// Create monitor options
	monitorOptions := crdtmonitor.NewMonitorOptions()
	monitorOptions.DocumentID = "example-doc"
	monitorOptions.PubSubTopic = "crdt-patches"
	monitorOptions.LogEvents = true

	// Create CRDT monitor
	monitor, err := crdtmonitor.NewCRDTMonitor(redisPubSub, doc, monitorOptions)
	if err != nil {
		log.Fatalf("Failed to create CRDT monitor: %v", err)
	}

	// Add custom event handler
	monitor.AddEventHandler(crdtmonitor.EventTypePatchReceived, func(event crdtmonitor.MonitorEvent) {
		fmt.Printf("Custom handler: Received patch with ID %v\n", event.PatchID)
	})

	// Start the monitor
	if err := monitor.Start(ctx); err != nil {
		log.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Create web monitor options
	webOptions := crdtmonitor.NewWebMonitorOptions()
	webOptions.Addr = ":8080" // Change this to your preferred port

	// Create web monitor
	webMonitor, err := crdtmonitor.NewWebMonitor(monitor, webOptions)
	if err != nil {
		log.Fatalf("Failed to create web monitor: %v", err)
	}

	// Start the web monitor
	if err := webMonitor.Start(ctx); err != nil {
		log.Fatalf("Failed to start web monitor: %v", err)
	}
	defer webMonitor.Stop()

	fmt.Println("CRDT Monitor started. Web interface available at http://localhost:8080")
	fmt.Println("Press Ctrl+C to stop")

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start publisher
	wg.Add(1)
	go func() {
		defer wg.Done()
		runPublisher(ctx, redisPubSub, doc)
	}()

	// Wait for all goroutines to finish
	wg.Wait()
	fmt.Println("Shutdown complete")
}

func runPublisher(ctx context.Context, pubsub crdtpubsub.PubSub, doc *crdt.Document) {
	// Create a ticker to publish patches periodically
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	counter := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Create a patch
			patch := createSamplePatch(doc, counter)
			counter++

			// Publish the patch
			topic := "crdt-patches"
			fmt.Printf("Publishing patch #%d to topic %s\n", counter, topic)
			if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
				fmt.Printf("Failed to publish patch: %v\n", err)
				continue
			}

			// Simulate some errors and conflicts occasionally
			if counter%5 == 0 {
				// Create a conflicting patch
				conflictPatch := createConflictingPatch(doc, counter)
				fmt.Printf("Publishing conflicting patch #%d to topic %s\n", counter, topic)
				if err := pubsub.Publish(ctx, topic, conflictPatch, crdtpubsub.EncodingFormatJSON); err != nil {
					fmt.Printf("Failed to publish conflicting patch: %v\n", err)
				}
			}

			if counter%7 == 0 {
				// Create an invalid patch
				invalidPatch := createInvalidPatch(doc, counter)
				fmt.Printf("Publishing invalid patch #%d to topic %s\n", counter, topic)
				if err := pubsub.Publish(ctx, topic, invalidPatch, crdtpubsub.EncodingFormatJSON); err != nil {
					fmt.Printf("Failed to publish invalid patch: %v\n", err)
				}
			}
		}
	}
}

func createSamplePatch(doc *crdt.Document, counter int) *crdtpatch.Patch {
	// Create a new patch
	patchID := common.LogicalTimestamp{SID: 1, Counter: uint64(counter)}
	patch := crdtpatch.NewPatch(patchID)

	// Create a new operation
	valueID := common.LogicalTimestamp{SID: 1, Counter: uint64(counter) + 1}
	valueOp := &crdtpatch.NewOperation{
		ID:       valueID,
		NodeType: common.NodeTypeCon,
		Value:    fmt.Sprintf("value-%d", counter),
	}
	patch.AddOperation(valueOp)

	// Create an insert operation
	insID := common.LogicalTimestamp{SID: 1, Counter: uint64(counter) + 2}
	insOp := &crdtpatch.InsOperation{
		ID:       insID,
		TargetID: common.LogicalTimestamp{SID: 0, Counter: 0},
		Value:    map[string]any{fmt.Sprintf("key-%d", counter): fmt.Sprintf("value-%d", counter)},
	}
	patch.AddOperation(insOp)

	return patch
}

func createConflictingPatch(doc *crdt.Document, counter int) *crdtpatch.Patch {
	// Create a new patch with a different session ID but same counter
	patchID := common.LogicalTimestamp{SID: 2, Counter: uint64(counter)}
	patch := crdtpatch.NewPatch(patchID)

	// Create a new operation
	valueID := common.LogicalTimestamp{SID: 2, Counter: uint64(counter) + 1}
	valueOp := &crdtpatch.NewOperation{
		ID:       valueID,
		NodeType: common.NodeTypeCon,
		Value:    fmt.Sprintf("conflict-value-%d", counter),
	}
	patch.AddOperation(valueOp)

	// Create an insert operation for the same key
	insID := common.LogicalTimestamp{SID: 2, Counter: uint64(counter) + 2}
	insOp := &crdtpatch.InsOperation{
		ID:       insID,
		TargetID: common.LogicalTimestamp{SID: 0, Counter: 0},
		Value:    map[string]any{fmt.Sprintf("key-%d", counter-1): fmt.Sprintf("conflict-value-%d", counter)},
	}
	patch.AddOperation(insOp)

	return patch
}

func createInvalidPatch(doc *crdt.Document, counter int) *crdtpatch.Patch {
	// Create a new patch
	patchID := common.LogicalTimestamp{SID: 3, Counter: uint64(counter)}
	patch := crdtpatch.NewPatch(patchID)

	// Create an invalid operation (e.g., referencing a non-existent target)
	insID := common.LogicalTimestamp{SID: 3, Counter: uint64(counter) + 1}
	insOp := &crdtpatch.InsOperation{
		ID:       insID,
		TargetID: common.LogicalTimestamp{SID: 999, Counter: 999}, // Non-existent target
		Value:    map[string]any{fmt.Sprintf("invalid-key-%d", counter): fmt.Sprintf("invalid-value-%d", counter)},
	}
	patch.AddOperation(insOp)

	return patch
}
