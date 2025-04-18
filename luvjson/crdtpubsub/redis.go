package crdtpubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"tictactoe/luvjson/crdtpatch"

	"github.com/go-redis/redis/v8"
)

// RedisPubSub implements the PubSub interface using Redis.
type RedisPubSub struct {
	// client is the Redis client.
	client *redis.Client
	// pubsub is the Redis pubsub client.
	pubsub *redis.PubSub
	// options contains the configuration options.
	options *Options
	// subscriptions is a map of topic to subscription.
	subscriptions map[string]*redisSubscription
	// mutex protects the subscriptions map.
	mutex sync.RWMutex
	// closed indicates whether the PubSub has been closed.
	closed bool
}

// redisSubscription represents a subscription to a Redis topic.
type redisSubscription struct {
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
	// done is a channel that is closed when the subscription is done.
	done chan struct{}
}

// NewRedisPubSub creates a new RedisPubSub with the specified Redis client and options.
func NewRedisPubSub(client *redis.Client, options *Options) (*RedisPubSub, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	if options == nil {
		options = NewOptions()
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisPubSub{
		client:        client,
		pubsub:        nil,
		options:       options,
		subscriptions: make(map[string]*redisSubscription),
		mutex:         sync.RWMutex{},
		closed:        false,
	}, nil
}

// Publish publishes a patch to the specified topic.
func (ps *RedisPubSub) Publish(ctx context.Context, topic string, patch *crdtpatch.Patch, format EncodingFormat) error {
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

	// Encode the message
	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Publish the message
	return ps.client.Publish(ctx, topic, msgData).Err()
}

// PublishRaw publishes raw data to the specified topic.
func (ps *RedisPubSub) PublishRaw(ctx context.Context, topic string, data []byte, format EncodingFormat) error {
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

	// Encode the message
	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Publish the message
	return ps.client.Publish(ctx, topic, msgData).Err()
}

// Subscribe subscribes to the specified topic and calls the handler for each received message.
// This method implements the Subscriber interface.
func (ps *RedisPubSub) Subscribe(ctx context.Context, topic string, subscriberID string, handler SubscriberFunc) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Check if already subscribed with this subscriberID
	if sub, ok := ps.subscriptions[topic]; ok && sub.subscriberID == subscriberID {
		return fmt.Errorf("already subscribed to topic: %s with subscriberID: %s", topic, subscriberID)
	}

	// Create a new pubsub client if needed
	if ps.pubsub == nil {
		ps.pubsub = ps.client.Subscribe(ctx)
	}

	// Subscribe to the topic
	if err := ps.pubsub.Subscribe(ctx, topic); err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	// Create a message handler that calls the subscriber function
	messageHandler := func(msg PatchMessage) error {
		return handler(ctx, msg.Topic, msg.Payload, msg.Format)
	}

	// Create a new subscription
	subCtx, cancel := context.WithCancel(ctx)
	subscription := &redisSubscription{
		topic:          topic,
		subscriberID:   subscriberID,
		handler:        messageHandler,
		subscriberFunc: handler,
		ctx:            subCtx,
		cancel:         cancel,
		done:           make(chan struct{}),
	}
	ps.subscriptions[topic] = subscription

	// Start a goroutine to handle messages
	go ps.handleMessages(subscription)

	return nil
}

// SubscribeWithHandler subscribes to the specified topic with a MessageHandler.
// This is a convenience method that wraps the Subscribe method.
func (ps *RedisPubSub) SubscribeWithHandler(ctx context.Context, topic string, handler MessageHandler) error {
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

// handleMessages handles messages for a subscription.
func (ps *RedisPubSub) handleMessages(subscription *redisSubscription) {
	defer close(subscription.done)

	ch := ps.pubsub.Channel()
	for {
		select {
		case <-subscription.ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if msg.Channel != subscription.topic {
				continue
			}

			// Decode the message
			var patchMsg PatchMessage
			if err := json.Unmarshal([]byte(msg.Payload), &patchMsg); err != nil {
				// Log the error but continue
				fmt.Printf("failed to decode message: %v\n", err)
				continue
			}

			// Call the handler
			if err := subscription.handler(patchMsg); err != nil {
				// Log the error but continue
				fmt.Printf("failed to handle message: %v\n", err)
				continue
			}
		}
	}
}

// Unsubscribe unsubscribes from the specified topic.
// This method implements the Subscriber interface.
func (ps *RedisPubSub) Unsubscribe(ctx context.Context, topic string, subscriberID string) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Check if subscribed with this subscriberID
	subscription, ok := ps.subscriptions[topic]
	if !ok || subscription.subscriberID != subscriberID {
		return fmt.Errorf("not subscribed to topic: %s with subscriberID: %s", topic, subscriberID)
	}

	// Unsubscribe from the topic
	if err := ps.pubsub.Unsubscribe(ctx, topic); err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	// Cancel the subscription context
	subscription.cancel()

	// Wait for the subscription to finish
	<-subscription.done

	// Remove the subscription
	delete(ps.subscriptions, topic)

	return nil
}

// UnsubscribeAll unsubscribes all subscribers from the specified topic.
// This is a convenience method that removes all subscriptions for a topic.
func (ps *RedisPubSub) UnsubscribeAll(ctx context.Context, topic string) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Check if subscribed
	subscription, ok := ps.subscriptions[topic]
	if !ok {
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}

	// Unsubscribe from the topic
	if err := ps.pubsub.Unsubscribe(ctx, topic); err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	// Cancel the subscription context
	subscription.cancel()

	// Wait for the subscription to finish
	<-subscription.done

	// Remove the subscription
	delete(ps.subscriptions, topic)

	return nil
}

// UnsubscribeHandler unsubscribes a specific handler from the specified topic.
// This is a convenience method that wraps the Unsubscribe method.
func (ps *RedisPubSub) UnsubscribeHandler(ctx context.Context, topic string, handler MessageHandler) error {
	if ps.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Generate the same subscriber ID that would have been used in SubscribeWithHandler
	subscriberID := fmt.Sprintf("handler-%p", handler)

	// Use the standard Unsubscribe method
	return ps.Unsubscribe(ctx, topic, subscriberID)
}

// Close closes the PubSub.
func (ps *RedisPubSub) Close() error {
	if ps.closed {
		return nil
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// Mark as closed
	ps.closed = true

	// Cancel all subscriptions
	for _, subscription := range ps.subscriptions {
		subscription.cancel()
	}

	// Wait for all subscriptions to finish
	for _, subscription := range ps.subscriptions {
		<-subscription.done
	}

	// Close the pubsub client
	if ps.pubsub != nil {
		if err := ps.pubsub.Close(); err != nil {
			return fmt.Errorf("failed to close pubsub client: %w", err)
		}
	}

	// Close the Redis client
	if err := ps.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}

	return nil
}
