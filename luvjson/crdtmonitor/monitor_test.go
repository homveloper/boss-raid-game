package crdtmonitor

import (
	"context"
	"sync"
	"testing"
	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
	"time"
)

// MockPubSub is a mock implementation of the PubSub interface for testing.
type MockPubSub struct {
	publishedMessages map[string][]crdtpubsub.PatchMessage
	subscribers       map[string]map[string]crdtpubsub.SubscriberFunc
	mutex             sync.RWMutex
}

// NewMockPubSub creates a new MockPubSub.
func NewMockPubSub() *MockPubSub {
	return &MockPubSub{
		publishedMessages: make(map[string][]crdtpubsub.PatchMessage),
		subscribers:       make(map[string]map[string]crdtpubsub.SubscriberFunc),
		mutex:             sync.RWMutex{},
	}
}

// Publish publishes a patch to the specified topic.
func (ps *MockPubSub) Publish(ctx context.Context, topic string, patch *crdtpatch.Patch, format crdtpubsub.EncodingFormat) error {
	// Encode the patch
	encoder, err := crdtpubsub.GetEncoderDecoder(format)
	if err != nil {
		return err
	}
	data, err := encoder.Encode(patch)
	if err != nil {
		return err
	}

	// Create a message
	msg := crdtpubsub.PatchMessage{
		Topic:   topic,
		Payload: data,
		Format:  format,
		Metadata: map[string]string{
			"format": string(format),
		},
	}

	// Store the message
	ps.mutex.Lock()
	ps.publishedMessages[topic] = append(ps.publishedMessages[topic], msg)
	ps.mutex.Unlock()

	// Notify subscribers
	ps.mutex.RLock()
	subscribers := ps.subscribers[topic]
	ps.mutex.RUnlock()

	for _, subscriber := range subscribers {
		go func(sub crdtpubsub.SubscriberFunc) {
			_ = sub(ctx, topic, msg.Payload, msg.Format)
		}(subscriber)
	}

	return nil
}

// PublishRaw publishes raw data to the specified topic.
func (ps *MockPubSub) PublishRaw(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
	// Create a message
	msg := crdtpubsub.PatchMessage{
		Topic:   topic,
		Payload: data,
		Format:  format,
		Metadata: map[string]string{
			"format": string(format),
		},
	}

	// Store the message
	ps.mutex.Lock()
	ps.publishedMessages[topic] = append(ps.publishedMessages[topic], msg)
	ps.mutex.Unlock()

	// Notify subscribers
	ps.mutex.RLock()
	subscribers := ps.subscribers[topic]
	ps.mutex.RUnlock()

	for _, subscriber := range subscribers {
		go func(sub crdtpubsub.SubscriberFunc) {
			_ = sub(ctx, topic, msg.Payload, msg.Format)
		}(subscriber)
	}

	return nil
}

// Subscribe subscribes to the specified topic and calls the handler for each received message.
func (ps *MockPubSub) Subscribe(ctx context.Context, topic string, subscriberID string, handler crdtpubsub.SubscriberFunc) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Initialize the subscribers map for this topic if it doesn't exist
	if ps.subscribers[topic] == nil {
		ps.subscribers[topic] = make(map[string]crdtpubsub.SubscriberFunc)
	}

	// Add the subscriber
	ps.subscribers[topic][subscriberID] = handler
	return nil
}

// SubscribeWithHandler subscribes to the specified topic with a MessageHandler.
func (ps *MockPubSub) SubscribeWithHandler(ctx context.Context, topic string, handler crdtpubsub.MessageHandler) error {
	// Generate a unique subscriber ID
	subscriberID := "handler-mock"

	// Create a wrapper function that converts MessageHandler to SubscriberFunc
	subscriberFunc := func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		msg := crdtpubsub.PatchMessage{
			Topic:   topic,
			Payload: data,
			Format:  format,
			Metadata: map[string]string{
				"format": string(format),
			},
		}
		return handler(msg)
	}

	return ps.Subscribe(ctx, topic, subscriberID, subscriberFunc)
}

// Unsubscribe unsubscribes from the specified topic.
func (ps *MockPubSub) Unsubscribe(ctx context.Context, topic string, subscriberID string) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Remove the subscriber
	if subscribers, ok := ps.subscribers[topic]; ok {
		delete(subscribers, subscriberID)
		// If there are no more subscribers for this topic, remove the topic
		if len(subscribers) == 0 {
			delete(ps.subscribers, topic)
		}
	}
	return nil
}

// UnsubscribeAll unsubscribes all subscribers from the specified topic.
func (ps *MockPubSub) UnsubscribeAll(ctx context.Context, topic string) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Remove all subscribers for this topic
	delete(ps.subscribers, topic)
	return nil
}

// Close closes the PubSub.
func (ps *MockPubSub) Close() error {
	ps.mutex.Lock()
	ps.subscribers = make(map[string]map[string]crdtpubsub.SubscriberFunc)
	ps.mutex.Unlock()
	return nil
}

// TestCRDTMonitor tests the CRDTMonitor.
func TestCRDTMonitor(t *testing.T) {
	// Create a mock PubSub
	pubsub := NewMockPubSub()

	// Create a CRDT document
	docSID := common.NewSessionID()
	doc := crdt.NewDocument(docSID)

	// Create monitor options
	options := NewMonitorOptions()
	options.DocumentID = "test-doc"
	options.PubSubTopic = "test-topic"
	options.LogEvents = false

	// Create a monitor
	monitor, err := NewCRDTMonitor(pubsub, doc, options)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Add event handlers
	eventsCh := make(chan MonitorEvent, 10)
	monitor.AddEventHandler(EventTypePatchReceived, func(event MonitorEvent) {
		eventsCh <- event
	})
	monitor.AddEventHandler(EventTypePatchApplied, func(event MonitorEvent) {
		eventsCh <- event
	})

	// Start the monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}

	// Create a patch
	sid1 := common.NewSessionID()
	patchID := common.LogicalTimestamp{SID: sid1, Counter: 1}
	patch := crdtpatch.NewPatch(patchID)

	sid2 := common.NewSessionID()
	valueID := common.LogicalTimestamp{SID: sid2, Counter: 2}
	valueOp := &crdtpatch.NewOperation{
		ID:       valueID,
		NodeType: common.NodeTypeCon,
		Value:    "test-value",
	}
	patch.AddOperation(valueOp)

	sid3 := common.NewSessionID()
	insID := common.LogicalTimestamp{SID: sid3, Counter: 3}
	zeroSID := common.SessionID{}
	insOp := &crdtpatch.InsOperation{
		ID:       insID,
		TargetID: common.LogicalTimestamp{SID: zeroSID, Counter: 0},
		Value:    map[string]any{"test-key": "test-value"},
	}
	patch.AddOperation(insOp)

	// Publish the patch
	if err := pubsub.Publish(ctx, "test-topic", patch, crdtpubsub.EncodingFormatJSON); err != nil {
		t.Fatalf("Failed to publish patch: %v", err)
	}

	// Wait for events
	receivedEvents := 0
	timeout := time.After(5 * time.Second)
	for receivedEvents < 2 {
		select {
		case <-eventsCh:
			receivedEvents++
		case <-timeout:
			t.Fatalf("Timed out waiting for events, received %d events", receivedEvents)
		}
	}

	// Check stats
	stats := monitor.GetStats()
	if stats.TotalPatchesReceived != 1 {
		t.Errorf("Expected 1 patch received, got %d", stats.TotalPatchesReceived)
	}
	if stats.TotalPatchesApplied != 1 {
		t.Errorf("Expected 1 patch applied, got %d", stats.TotalPatchesApplied)
	}
	if _, ok := stats.SessionStats[sid1]; !ok {
		t.Errorf("Expected session stats for session %v", sid1)
	} else if stats.SessionStats[sid1].TotalPatchesSent != 1 {
		t.Errorf("Expected 1 patch sent by session %v, got %d", sid1, stats.SessionStats[sid1].TotalPatchesSent)
	}

	// Stop the monitor
	if err := monitor.Stop(); err != nil {
		t.Fatalf("Failed to stop monitor: %v", err)
	}

	// Check that the monitor is not running
	if monitor.IsRunning() {
		t.Errorf("Expected monitor to not be running")
	}
}

// TestWebMonitor tests the WebMonitor.
func TestWebMonitor(t *testing.T) {
	// Create a mock PubSub
	pubsub := NewMockPubSub()

	// Create a CRDT document
	docSID := common.NewSessionID()
	doc := crdt.NewDocument(docSID)

	// Create monitor options
	options := NewMonitorOptions()
	options.DocumentID = "test-doc"
	options.PubSubTopic = "test-topic"
	options.LogEvents = false

	// Create a monitor
	monitor, err := NewCRDTMonitor(pubsub, doc, options)
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}

	// Start the monitor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := monitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop()

	// Create web monitor options
	webOptions := NewWebMonitorOptions()
	webOptions.Addr = ":0" // Use a random port

	// Create web monitor
	webMonitor, err := NewWebMonitor(monitor, webOptions)
	if err != nil {
		t.Fatalf("Failed to create web monitor: %v", err)
	}

	// Start the web monitor
	if err := webMonitor.Start(ctx); err != nil {
		t.Fatalf("Failed to start web monitor: %v", err)
	}
	defer webMonitor.Stop()

	// Check that the web monitor is running
	if !webMonitor.IsRunning() {
		t.Errorf("Expected web monitor to be running")
	}

	// Create a patch and publish it to test event handling
	sid1 := common.NewSessionID()
	patchID := common.LogicalTimestamp{SID: sid1, Counter: 1}
	patch := crdtpatch.NewPatch(patchID)

	sid2 := common.NewSessionID()
	valueID := common.LogicalTimestamp{SID: sid2, Counter: 2}
	valueOp := &crdtpatch.NewOperation{
		ID:       valueID,
		NodeType: common.NodeTypeCon,
		Value:    "test-value",
	}
	patch.AddOperation(valueOp)

	// Publish the patch
	if err := pubsub.Publish(ctx, "test-topic", patch, crdtpubsub.EncodingFormatJSON); err != nil {
		t.Fatalf("Failed to publish patch: %v", err)
	}

	// Wait a bit for the event to be processed
	time.Sleep(100 * time.Millisecond)

	// Stop the web monitor
	if err := webMonitor.Stop(); err != nil {
		t.Fatalf("Failed to stop web monitor: %v", err)
	}

	// Check that the web monitor is not running
	if webMonitor.IsRunning() {
		t.Errorf("Expected web monitor to not be running")
	}
}
