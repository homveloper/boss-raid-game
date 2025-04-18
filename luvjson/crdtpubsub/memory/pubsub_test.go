package memory

import (
	"context"
	"testing"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
)

func TestMemoryPubSub(t *testing.T) {
	// Create a context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a new Memory PubSub
	pubsub, err := NewPubSub()
	if err != nil {
		t.Fatalf("Failed to create Memory PubSub: %v", err)
	}
	defer pubsub.Close()

	// Create a patch
	sid := common.NewSessionID()
	id := common.LogicalTimestamp{SID: sid, Counter: 1}
	patch := crdtpatch.NewPatch(id)

	// Create a new constant node operation
	op := &crdtpatch.NewOperation{
		ID:       id,
		NodeType: common.NodeTypeCon,
		Value:    "test-value",
	}
	patch.AddOperation(op)

	// Create a channel to receive updates
	updateReceived := make(chan struct{})

	// Subscribe to a topic
	topic := "test-topic"
	subscriberID := "test-subscriber"
	if err := pubsub.Subscribe(ctx, topic, subscriberID, func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		// Decode the patch
		encoderDecoder, err := crdtpubsub.GetEncoderDecoder(format)
		if err != nil {
			t.Errorf("Failed to get encoder/decoder: %v", err)
			return err
		}

		receivedPatch, err := encoderDecoder.Decode(data)
		if err != nil {
			t.Errorf("Failed to decode patch: %v", err)
			return err
		}

		// Verify the patch
		if receivedPatch.ID().Counter != patch.ID().Counter {
			t.Errorf("Expected patch counter %d, got %d", patch.ID().Counter, receivedPatch.ID().Counter)
		}

		// Signal that we received the update
		updateReceived <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}

	// Publish the patch
	if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		t.Fatalf("Failed to publish patch: %v", err)
	}

	// Wait for the subscriber to receive the update or timeout
	select {
	case <-updateReceived:
		// Success, continue
		t.Logf("Received update successfully")
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for subscriber to receive update")
	}

	// Unsubscribe
	if err := pubsub.Unsubscribe(ctx, topic, subscriberID); err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// Publish again, should not receive an update
	if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		t.Fatalf("Failed to publish patch: %v", err)
	}

	// Wait a short time to ensure no update is received
	select {
	case <-updateReceived:
		t.Fatalf("Received update after unsubscribing")
	case <-time.After(100 * time.Millisecond):
		// Success, no update received
		t.Logf("No update received after unsubscribing, as expected")
	}
}

func TestMemoryPubSubMultipleSubscribers(t *testing.T) {
	// Create a context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a new Memory PubSub
	pubsub, err := NewPubSub()
	if err != nil {
		t.Fatalf("Failed to create Memory PubSub: %v", err)
	}
	defer pubsub.Close()

	// Create a patch
	sid := common.NewSessionID()
	id := common.LogicalTimestamp{SID: sid, Counter: 1}
	patch := crdtpatch.NewPatch(id)

	// Create a new constant node operation
	op := &crdtpatch.NewOperation{
		ID:       id,
		NodeType: common.NodeTypeCon,
		Value:    "test-value",
	}
	patch.AddOperation(op)

	// Create channels to receive updates
	updateReceived1 := make(chan struct{})
	updateReceived2 := make(chan struct{})

	// Subscribe to a topic with two subscribers
	topic := "test-topic"
	subscriberID1 := "test-subscriber-1"
	subscriberID2 := "test-subscriber-2"

	// First subscriber
	if err := pubsub.Subscribe(ctx, topic, subscriberID1, func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		// Signal that we received the update
		updateReceived1 <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}

	// Second subscriber
	if err := pubsub.Subscribe(ctx, topic, subscriberID2, func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		// Signal that we received the update
		updateReceived2 <- struct{}{}
		return nil
	}); err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}

	// Publish the patch
	if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		t.Fatalf("Failed to publish patch: %v", err)
	}

	// Wait for both subscribers to receive the update or timeout
	for i := 0; i < 2; i++ {
		select {
		case <-updateReceived1:
			// Success, continue
		case <-updateReceived2:
			// Success, continue
		case <-time.After(2 * time.Second):
			t.Fatalf("Timed out waiting for subscribers to receive update")
		}
	}

	// Unsubscribe the first subscriber
	if err := pubsub.Unsubscribe(ctx, topic, subscriberID1); err != nil {
		t.Fatalf("Failed to unsubscribe: %v", err)
	}

	// Publish again, only the second subscriber should receive an update
	if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err != nil {
		t.Fatalf("Failed to publish patch: %v", err)
	}

	// Wait for the second subscriber to receive the update or timeout
	select {
	case <-updateReceived2:
		// Success, continue
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for second subscriber to receive update")
	}

	// Make sure the first subscriber did not receive an update
	select {
	case <-updateReceived1:
		t.Fatalf("First subscriber received update after unsubscribing")
	case <-time.After(100 * time.Millisecond):
		// Success, no update received
	}
}

func TestMemoryPubSubClose(t *testing.T) {
	// Create a context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a new Memory PubSub
	pubsub, err := NewPubSub()
	if err != nil {
		t.Fatalf("Failed to create Memory PubSub: %v", err)
	}

	// Create a patch
	sid := common.NewSessionID()
	id := common.LogicalTimestamp{SID: sid, Counter: 1}
	patch := crdtpatch.NewPatch(id)

	// Create a new constant node operation
	op := &crdtpatch.NewOperation{
		ID:       id,
		NodeType: common.NodeTypeCon,
		Value:    "test-value",
	}
	patch.AddOperation(op)

	// Subscribe to a topic
	topic := "test-topic"
	subscriberID := "test-subscriber"
	if err := pubsub.Subscribe(ctx, topic, subscriberID, func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		return nil
	}); err != nil {
		t.Fatalf("Failed to subscribe to topic: %v", err)
	}

	// Close the PubSub
	if err := pubsub.Close(); err != nil {
		t.Fatalf("Failed to close PubSub: %v", err)
	}

	// Publish after close should fail
	if err := pubsub.Publish(ctx, topic, patch, crdtpubsub.EncodingFormatJSON); err == nil {
		t.Fatalf("Expected error when publishing after close")
	}

	// Subscribe after close should fail
	if err := pubsub.Subscribe(ctx, topic, "new-subscriber", func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		return nil
	}); err == nil {
		t.Fatalf("Expected error when subscribing after close")
	}

	// Unsubscribe after close should fail
	if err := pubsub.Unsubscribe(ctx, topic, subscriberID); err == nil {
		t.Fatalf("Expected error when unsubscribing after close")
	}

	// Close again should succeed
	if err := pubsub.Close(); err != nil {
		t.Fatalf("Failed to close PubSub again: %v", err)
	}
}
