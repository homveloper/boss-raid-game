package crdtpubsub

import (
	"context"
	"tictactoe/luvjson/crdtpatch"
)

// EncodingFormat represents the format used to encode CRDT patches.
type EncodingFormat string

const (
	// EncodingFormatJSON represents JSON encoding.
	EncodingFormatJSON EncodingFormat = "json"
	// EncodingFormatBinary represents binary encoding.
	EncodingFormatBinary EncodingFormat = "binary"
	// EncodingFormatText represents text encoding.
	EncodingFormatText EncodingFormat = "text"
	// EncodingFormatBase64 represents base64 encoding.
	EncodingFormatBase64 EncodingFormat = "base64"
)

// PatchMessage represents a message containing a CRDT patch.
type PatchMessage struct {
	// Topic is the topic the message was published to.
	Topic string
	// Payload is the encoded patch data.
	Payload []byte
	// Format is the encoding format used for the payload.
	Format EncodingFormat
	// Metadata is optional metadata associated with the message.
	Metadata map[string]string
}

// MessageHandler is a function that handles a received patch message.
type MessageHandler func(msg PatchMessage) error

// SubscriberFunc is a function that handles a received patch message with raw data.
type SubscriberFunc func(ctx context.Context, topic string, data []byte, format EncodingFormat) error

// Publisher defines the interface for publishing CRDT patches.
type Publisher interface {
	// Publish publishes a patch to the specified topic.
	Publish(ctx context.Context, topic string, patch *crdtpatch.Patch, format EncodingFormat) error
	// PublishRaw publishes raw data to the specified topic.
	PublishRaw(ctx context.Context, topic string, data []byte, format EncodingFormat) error
	// Close closes the publisher.
	Close() error
}

// Subscriber defines the interface for subscribing to CRDT patches.
type Subscriber interface {
	// Subscribe subscribes to the specified topic and calls the handler for each received message.
	Subscribe(ctx context.Context, topic string, subscriberID string, handler SubscriberFunc) error
	// Unsubscribe unsubscribes from the specified topic.
	Unsubscribe(ctx context.Context, topic string, subscriberID string) error
	// Close closes the subscriber.
	Close() error
}

// PubSub combines the Publisher and Subscriber interfaces.
type PubSub interface {
	Publisher
	Subscriber
}

// Options represents configuration options for a PubSub implementation.
type Options struct {
	// DefaultFormat is the default encoding format to use.
	DefaultFormat EncodingFormat
	// ConnectionString is the connection string for the PubSub service.
	ConnectionString string
	// ClientID is the client ID to use for the PubSub service.
	ClientID string
	// Credentials contains authentication credentials.
	Credentials map[string]string
	// AdditionalOptions contains additional implementation-specific options.
	AdditionalOptions map[string]interface{}
}

// NewOptions creates a new Options with default values.
func NewOptions() *Options {
	return &Options{
		DefaultFormat:     EncodingFormatJSON,
		Credentials:       make(map[string]string),
		AdditionalOptions: make(map[string]interface{}),
	}
}
