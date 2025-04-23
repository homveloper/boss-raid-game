package nodestorage

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Options represents configuration options for the storage
type Options struct {
	// Concurrency control options
	MaxRetries       int
	RetryDelay       time.Duration
	MaxRetryDelay    time.Duration
	RetryJitter      float64 // Random jitter factor (0.0-1.0) to add to retry delay
	OperationTimeout time.Duration
	VersionField     string // Field name used for optimistic concurrency control

	// Cache options
	CacheTTL time.Duration

	// Watch options
	WatchEnabled      bool
	WatchFilter       []bson.D // MongoDB change stream filter pipeline
	WatchFullDocument string   // "updateLookup", "required", etc.
	WatchMaxAwaitTime time.Duration
	WatchBatchSize    int32
}

// DefaultOptions returns the default options
func DefaultOptions() *Options {
	return &Options{
		// Concurrency control defaults
		MaxRetries:       0, // 0 means unlimited retries
		RetryDelay:       time.Millisecond * 100,
		MaxRetryDelay:    time.Second * 2,
		RetryJitter:      0.1,
		OperationTimeout: time.Second * 30,
		VersionField:     "version",

		// Cache defaults
		CacheTTL: time.Hour * 24,

		// Watch defaults
		WatchEnabled: true,
		WatchFilter: []bson.D{
			{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "delete"}}}}}}},
		},
		WatchFullDocument: "updateLookup",
		WatchMaxAwaitTime: time.Second * 1,
		WatchBatchSize:    100,
	}
}

// EditOption is a function that configures EditOptions
type EditOption func(*EditOptions)

// EditOptions represents options for document editing
type EditOptions struct {
	// Optimistic concurrency control options
	MaxRetries    int
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
	RetryJitter   float64

	// Operation timeout
	Timeout time.Duration
}

// WithMaxRetries sets the maximum number of retries for optimistic concurrency control
func WithMaxRetries(maxRetries int) EditOption {
	return func(opts *EditOptions) {
		opts.MaxRetries = maxRetries
	}
}

// WithRetryDelay sets the initial delay between retries
func WithRetryDelay(delay time.Duration) EditOption {
	return func(opts *EditOptions) {
		opts.RetryDelay = delay
	}
}

// WithMaxRetryDelay sets the maximum delay between retries
func WithMaxRetryDelay(maxDelay time.Duration) EditOption {
	return func(opts *EditOptions) {
		opts.MaxRetryDelay = maxDelay
	}
}

// WithRetryJitter sets the jitter factor for retry delays
func WithRetryJitter(jitter float64) EditOption {
	return func(opts *EditOptions) {
		opts.RetryJitter = jitter
	}
}

// WithTimeout sets the operation timeout
func WithTimeout(timeout time.Duration) EditOption {
	return func(opts *EditOptions) {
		opts.Timeout = timeout
	}
}

// NewEditOptions creates a new EditOptions with the given options applied
func NewEditOptions(opts ...EditOption) *EditOptions {
	// Default values
	options := &EditOptions{
		MaxRetries:    0, // 0 means unlimited retries
		RetryDelay:    time.Millisecond * 10,
		MaxRetryDelay: time.Millisecond * 100,
		RetryJitter:   0.1,
		Timeout:       time.Second * 10,
	}

	// Apply options
	for _, opt := range opts {
		opt(options)
	}

	return options
}
