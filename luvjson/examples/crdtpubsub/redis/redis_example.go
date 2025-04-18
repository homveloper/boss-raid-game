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
	options := crdtpubsub.NewOptions()
	options.DefaultFormat = crdtpubsub.EncodingFormatJSON

	// Create Redis PubSub with the client
	redisPubSub, err := crdtpubsub.NewRedisPubSub(redisClient, options)
	if err != nil {
		log.Fatalf("Failed to create Redis PubSub: %v", err)
	}
	defer redisPubSub.Close()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Start publisher
	wg.Add(1)
	go func() {
		defer wg.Done()
		runPublisher(ctx, redisPubSub)
	}()

	// Start subscriber
	wg.Add(1)
	go func() {
		defer wg.Done()
		runSubscriber(ctx, redisPubSub)
	}()

	// Wait for all goroutines to finish
	wg.Wait()
	fmt.Println("Shutdown complete")
}

func runPublisher(ctx context.Context, pubsub crdtpubsub.PubSub) {
	// Create a CRDT document
	doc := crdt.NewDocument(common.NewSessionID())

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

			// Also publish using different formats
			if counter%3 == 0 {
				fmt.Printf("Publishing patch #%d to topic %s with Base64 encoding\n", counter, topic+"-base64")
				if err := pubsub.Publish(ctx, topic+"-base64", patch, crdtpubsub.EncodingFormatBase64); err != nil {
					fmt.Printf("Failed to publish patch with Base64 encoding: %v\n", err)
				}
			}
		}
	}
}

func runSubscriber(ctx context.Context, pubsub crdtpubsub.PubSub) {
	// Create a CRDT document to apply received patches
	doc := crdt.NewDocument(2)

	// Subscribe to the topic
	topic := "crdt-patches"
	fmt.Printf("Subscribing to topic %s\n", topic)
	if err := pubsub.Subscribe(ctx, topic, func(msg crdtpubsub.PatchMessage) error {
		fmt.Printf("Received message from topic %s with format %s\n", msg.Topic, msg.Format)

		// Decode the patch
		decoder, err := crdtpubsub.GetEncoderDecoder(msg.Format)
		if err != nil {
			return fmt.Errorf("failed to get decoder: %w", err)
		}

		patch, err := decoder.Decode(msg.Payload)
		if err != nil {
			return fmt.Errorf("failed to decode patch: %w", err)
		}

		// Apply the patch to the document
		if err := patch.Apply(doc); err != nil {
			return fmt.Errorf("failed to apply patch: %w", err)
		}

		// Get the document view
		view, err := doc.View()
		if err != nil {
			return fmt.Errorf("failed to get document view: %w", err)
		}

		fmt.Printf("Document view after applying patch: %v\n", view)
		return nil
	}); err != nil {
		fmt.Printf("Failed to subscribe to topic: %v\n", err)
		return
	}

	// Also subscribe to the base64 topic
	fmt.Printf("Subscribing to topic %s\n", topic+"-base64")
	if err := pubsub.Subscribe(ctx, topic+"-base64", func(msg crdtpubsub.PatchMessage) error {
		fmt.Printf("Received message from topic %s with format %s\n", msg.Topic, msg.Format)

		// Decode the patch
		decoder, err := crdtpubsub.GetEncoderDecoder(msg.Format)
		if err != nil {
			return fmt.Errorf("failed to get decoder: %w", err)
		}

		patch, err := decoder.Decode(msg.Payload)
		if err != nil {
			return fmt.Errorf("failed to decode patch: %w", err)
		}

		// Apply the patch to the document
		if err := patch.Apply(doc); err != nil {
			return fmt.Errorf("failed to apply patch: %w", err)
		}

		// Get the document view
		view, err := doc.View()
		if err != nil {
			return fmt.Errorf("failed to get document view: %w", err)
		}

		fmt.Printf("Document view after applying patch from base64: %v\n", view)
		return nil
	}); err != nil {
		fmt.Printf("Failed to subscribe to base64 topic: %v\n", err)
		return
	}

	// Wait for context cancellation
	<-ctx.Done()

	// Unsubscribe from the topics
	unsubCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pubsub.Unsubscribe(unsubCtx, topic); err != nil {
		fmt.Printf("Failed to unsubscribe from topic: %v\n", err)
	}
	if err := pubsub.Unsubscribe(unsubCtx, topic+"-base64"); err != nil {
		fmt.Printf("Failed to unsubscribe from base64 topic: %v\n", err)
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
		Value:    map[string]interface{}{fmt.Sprintf("key-%d", counter): fmt.Sprintf("value-%d", counter)},
	}
	patch.AddOperation(insOp)

	return patch
}
