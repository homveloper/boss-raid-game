package crdtpubsub

import (
	"context"
	"fmt"
	"sync"
	"tictactoe/luvjson/crdtpatch"
)

// MemoryPubSub implements the PubSub interface using in-memory channels.
type MemoryPubSub struct {
	// options contains the configuration options.
	options *Options
	// subscriptions is a map of topic to subscriptions.
	subscriptions map[string][]*memorySubscription
	// mutex protects the subscriptions map.
	mutex sync.RWMutex
	// closed indicates whether the PubSub has been closed.
	closed bool
}

// memorySubscription represents a subscription to an in-memory topic.
type memorySubscription struct {
	// topic is the topic being subscribed to.
	topic string
	// subscriberID is the unique identifier for the subscriber.
	subscriberID string
	// handler is the message handler.
	handler MessageHandler
	// subscriberFunc is the subscriber function.
	subscriberFunc SubscriberFunc
	// ctx is the context for the subscription.
	ctx context.Context
	// cancel is the cancel function for the context.
	cancel context.CancelFunc
}

// NewMemoryPubSub creates a new MemoryPubSub with the specified options.
func NewMemoryPubSub(options *Options) (*MemoryPubSub, error) {
	if options == nil {
		options = NewOptions()
	}

	return &MemoryPubSub{
		options:       options,
		subscriptions: make(map[string][]*memorySubscription),
		mutex:         sync.RWMutex{},
		closed:        false,
	}, nil
}

// Publish publishes a patch to the specified topic.
func (ps *MemoryPubSub) Publish(ctx context.Context, topic string, patch *crdtpatch.Patch, format EncodingFormat) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Use the specified format or the default format
	if format == "" {
		format = ps.options.DefaultFormat
	}

	// Get the encoder for the format
	encoder, err := GetEncoderDecoder(format)
	if err != nil {
		return err
	}

	// Encode the patch
	data, err := encoder.Encode(patch)
	if err != nil {
		return fmt.Errorf("failed to encode patch: %w", err)
	}

	// Create metadata
	metadata := map[string]string{
		"format": string(format),
	}

	// Create message
	msg := PatchMessage{
		Topic:    topic,
		Payload:  data,
		Format:   format,
		Metadata: metadata,
	}

	// Deliver the message to all subscribers
	return ps.deliverMessage(ctx, msg)
}

// PublishRaw publishes raw data to the specified topic.
func (ps *MemoryPubSub) PublishRaw(ctx context.Context, topic string, data []byte, format EncodingFormat) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Use the specified format or the default format
	if format == "" {
		format = ps.options.DefaultFormat
	}

	// Create metadata
	metadata := map[string]string{
		"format": string(format),
	}

	// Create message
	msg := PatchMessage{
		Topic:    topic,
		Payload:  data,
		Format:   format,
		Metadata: metadata,
	}

	// Deliver the message to all subscribers
	return ps.deliverMessage(ctx, msg)
}

// deliverMessage delivers a message to all subscribers of the specified topic.
func (ps *MemoryPubSub) deliverMessage(ctx context.Context, msg PatchMessage) error {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Get the subscribers for the topic
	subscribers, ok := ps.subscriptions[msg.Topic]
	if !ok || len(subscribers) == 0 {
		// No subscribers, message is dropped
		return nil
	}

	// Deliver the message to each subscriber
	for _, sub := range subscribers {
		// Check if the subscription context is done
		select {
		case <-sub.ctx.Done():
			continue
		default:
			// Call the handler in a goroutine to avoid blocking
			go func(s *memorySubscription, m PatchMessage) {
				// Check context again before calling handler
				select {
				case <-s.ctx.Done():
					return
				default:
					if err := s.handler(m); err != nil {
						// Log the error but continue
						fmt.Printf("failed to handle message: %v\n", err)
					}
				}
			}(sub, msg)
		}
	}

	return nil
}

// Subscribe subscribes to the specified topic and calls the handler for each received message.
// This method implements the Subscriber interface.
func (ps *MemoryPubSub) Subscribe(ctx context.Context, topic string, subscriberID string, handler SubscriberFunc) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Create a new subscription
	subCtx, cancel := context.WithCancel(ctx)
	subscription := &memorySubscription{
		topic:          topic,
		subscriberID:   subscriberID,
		subscriberFunc: handler,
		handler: func(msg PatchMessage) error {
			// Convert MessageHandler to SubscriberFunc
			return handler(ctx, msg.Topic, msg.Payload, msg.Format)
		},
		ctx:    subCtx,
		cancel: cancel,
	}

	// Add the subscription to the map
	ps.subscriptions[topic] = append(ps.subscriptions[topic], subscription)

	return nil
}

// SubscribeWithHandler subscribes to the specified topic with a MessageHandler.
// This is a convenience method that wraps the Subscribe method.
func (ps *MemoryPubSub) SubscribeWithHandler(ctx context.Context, topic string, handler MessageHandler) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Generate a unique subscriber ID
	subscriberID := fmt.Sprintf("handler-%p", handler)

	// Create a wrapper function that converts MessageHandler to SubscriberFunc
	subscriberFunc := func(ctx context.Context, topic string, data []byte, format EncodingFormat) error {
		msg := PatchMessage{
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
// This method implements the Subscriber interface.
func (ps *MemoryPubSub) Unsubscribe(ctx context.Context, topic string, subscriberID string) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Check if subscribed
	subscribers, ok := ps.subscriptions[topic]
	if !ok || len(subscribers) == 0 {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	// Find and remove the specific subscriber
	var newSubscribers []*memorySubscription
	subscriberFound := false
	for _, sub := range subscribers {
		if sub.subscriberID == subscriberID {
			sub.cancel()
			subscriberFound = true
		} else {
			newSubscribers = append(newSubscribers, sub)
		}
	}

	if !subscriberFound {
		return fmt.Errorf("subscriber not found for topic: %s", topic)
	}

	// Update the subscriptions
	if len(newSubscribers) == 0 {
		delete(ps.subscriptions, topic)
	} else {
		ps.subscriptions[topic] = newSubscribers
	}

	return nil
}

// UnsubscribeAll unsubscribes all subscribers from the specified topic.
// This is a convenience method that removes all subscriptions for a topic.
func (ps *MemoryPubSub) UnsubscribeAll(ctx context.Context, topic string) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Check if subscribed
	subscribers, ok := ps.subscriptions[topic]
	if !ok || len(subscribers) == 0 {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	// Cancel all subscriptions for the topic
	for _, sub := range subscribers {
		sub.cancel()
	}

	// Remove the subscriptions
	delete(ps.subscriptions, topic)

	return nil
}

// UnsubscribeHandler unsubscribes a specific handler from the specified topic.
// This is an additional method not required by the PubSub interface but useful for in-memory implementation.
func (ps *MemoryPubSub) UnsubscribeHandler(ctx context.Context, topic string, handler MessageHandler) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Generate the same subscriber ID that would have been used in SubscribeWithHandler
	subscriberID := fmt.Sprintf("handler-%p", handler)

	// Use the standard Unsubscribe method
	return ps.Unsubscribe(ctx, topic, subscriberID)
}

// Close closes the PubSub.
func (ps *MemoryPubSub) Close() error {
	if ps.closed {
		return nil
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Mark as closed
	ps.closed = true

	// Cancel all subscriptions
	for _, subscribers := range ps.subscriptions {
		for _, sub := range subscribers {
			sub.cancel()
		}
	}

	// Clear the subscriptions map
	ps.subscriptions = make(map[string][]*memorySubscription)

	return nil
}
