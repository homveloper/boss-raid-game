package memory

import (
	"context"
	"fmt"
	"sync"

	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
)

// PubSub is an in-memory implementation of the crdtpubsub.PubSub interface.
type PubSub struct {
	// subscribers is a map of topic to a map of subscriber ID to subscriber.
	subscribers map[string]map[string]crdtpubsub.SubscriberFunc

	// encoderDecoders is a map of encoding format to encoder/decoder.
	encoderDecoders map[crdtpubsub.EncodingFormat]crdtpubsub.EncoderDecoder

	// mutex is used to protect access to the subscribers map.
	mutex sync.RWMutex

	// closed indicates whether the PubSub has been closed.
	closed bool
}

// NewPubSub creates a new in-memory PubSub.
func NewPubSub() (*PubSub, error) {
	// Initialize the encoders/decoders
	encoderDecoders := make(map[crdtpubsub.EncodingFormat]crdtpubsub.EncoderDecoder)
	for _, format := range []crdtpubsub.EncodingFormat{
		crdtpubsub.EncodingFormatJSON,
		crdtpubsub.EncodingFormatBinary,
		crdtpubsub.EncodingFormatText,
		crdtpubsub.EncodingFormatBase64,
	} {
		encoderDecoder, err := crdtpubsub.GetEncoderDecoder(format)
		if err != nil {
			return nil, fmt.Errorf("failed to get encoder/decoder for format %s: %w", format, err)
		}
		encoderDecoders[format] = encoderDecoder
	}

	return &PubSub{
		subscribers:    make(map[string]map[string]crdtpubsub.SubscriberFunc),
		encoderDecoders: encoderDecoders,
		closed:         false,
	}, nil
}

// Publish publishes a patch to a topic.
func (p *PubSub) Publish(ctx context.Context, topic string, patch *crdtpatch.Patch, format crdtpubsub.EncodingFormat) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Get the encoder/decoder for the specified format
	encoderDecoder, ok := p.encoderDecoders[format]
	if !ok {
		return fmt.Errorf("unsupported encoding format: %s", format)
	}

	// Encode the patch
	data, err := encoderDecoder.Encode(patch)
	if err != nil {
		return fmt.Errorf("failed to encode patch: %w", err)
	}

	// Publish the encoded patch to all subscribers
	return p.PublishRaw(ctx, topic, data, format)
}

// PublishRaw publishes raw data to a topic.
func (p *PubSub) PublishRaw(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Get the subscribers for the topic
	subscribers, ok := p.subscribers[topic]
	if !ok || len(subscribers) == 0 {
		// No subscribers, nothing to do
		return nil
	}

	// Publish to all subscribers
	for _, subscriber := range subscribers {
		// Check if the context is done
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue
		}

		// Call the subscriber function
		if err := subscriber(ctx, topic, data, format); err != nil {
			// Log the error but continue publishing to other subscribers
			fmt.Printf("Error publishing to subscriber: %v\n", err)
		}
	}

	return nil
}

// Subscribe subscribes to a topic.
func (p *PubSub) Subscribe(ctx context.Context, topic string, subscriberID string, subscriber crdtpubsub.SubscriberFunc) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Create the topic if it doesn't exist
	if _, ok := p.subscribers[topic]; !ok {
		p.subscribers[topic] = make(map[string]crdtpubsub.SubscriberFunc)
	}

	// Add the subscriber
	p.subscribers[topic][subscriberID] = subscriber

	return nil
}

// Unsubscribe unsubscribes from a topic.
func (p *PubSub) Unsubscribe(ctx context.Context, topic string, subscriberID string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return fmt.Errorf("pubsub is closed")
	}

	// Check if the topic exists
	subscribers, ok := p.subscribers[topic]
	if !ok {
		return nil
	}

	// Remove the subscriber
	delete(subscribers, subscriberID)

	// Remove the topic if there are no more subscribers
	if len(subscribers) == 0 {
		delete(p.subscribers, topic)
	}

	return nil
}

// Close closes the PubSub.
func (p *PubSub) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	p.subscribers = nil

	return nil
}
