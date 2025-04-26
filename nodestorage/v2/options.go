package v2

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// Options represents configuration options for the storage.
// These options control various aspects of the storage behavior, including
// concurrency control, caching, change streams, and transactions.
//
// The options can be provided when creating a new storage instance:
//
//	options := &v2.Options{
//	    VersionField: "vector_clock",
//	    CacheTTL:     time.Hour,
//	}
//	storage, err := v2.NewStorage[*Document](ctx, client, collection, cache, options)
type Options struct {
	// Concurrency control options

	// MaxRetries is the maximum number of retry attempts for optimistic concurrency control.
	// When a version conflict occurs, the operation will be retried up to this many times.
	// A value of 0 means unlimited retries (bounded by the context timeout).
	MaxRetries int

	// RetryDelay is the initial delay between retry attempts.
	// This delay increases exponentially with each retry, up to MaxRetryDelay.
	RetryDelay time.Duration

	// MaxRetryDelay is the maximum delay between retry attempts.
	// The delay will not exceed this value, regardless of how many retries have occurred.
	MaxRetryDelay time.Duration

	// RetryJitter is a random factor applied to retry delays to prevent thundering herd problems.
	// The actual delay will be the base delay plus a random value between 0 and RetryJitter * base delay.
	// Should be a value between 0.0 and 1.0.
	RetryJitter float64

	// OperationTimeout is the default timeout for operations.
	// If an operation takes longer than this, it will be aborted with a timeout error.
	// A value of 0 means no timeout (bounded by the context timeout).
	OperationTimeout time.Duration

	// VersionField is the name of the field used for optimistic concurrency control.
	// This field will be automatically incremented on each update to detect conflicts.
	// The field must be of type int64 in the document.
	VersionField string

	// Cache options

	// CacheTTL is the time-to-live for cached documents.
	// Documents will be automatically removed from the cache after this duration.
	// A value of 0 means no expiration (documents remain in cache until evicted).
	CacheTTL time.Duration

	// CacheQueryResults determines whether to cache results from FindMany queries.
	// If true, the results of FindMany queries will be cached individually.
	// This can improve performance for repeated queries but may increase memory usage.
	CacheQueryResults bool

	// Watch options

	// WatchEnabled determines whether to enable change stream watching.
	// If false, the Watch method will return an error.
	WatchEnabled bool

	// WatchFilter is a MongoDB change stream filter pipeline.
	// This pipeline is used to filter change events before they are sent to the client.
	WatchFilter []bson.D

	// WatchFullDocument specifies the fullDocument option for the change stream.
	// Valid values are "updateLookup", "required", "whenAvailable", and "default".
	// See MongoDB documentation for details on these options.
	WatchFullDocument string

	// WatchMaxAwaitTime is the maximum time the server waits for new data before returning.
	// This affects the responsiveness of the change stream.
	WatchMaxAwaitTime time.Duration

	// WatchBatchSize is the batch size for the change stream.
	// This affects the number of events returned in each batch from MongoDB.
	WatchBatchSize int32

	// Section options

	// SectionVersionField is the name of the field used for section version control.
	// This field will be automatically incremented on each update to a section.
	// The field must be of type int64 in the section document.
	SectionVersionField string

	// Transaction options

	// DefaultTransactionOptions contains the default options for MongoDB transactions.
	// These options are used when no specific options are provided to WithTransaction.
	DefaultTransactionOptions *TransactionOptions
}

// TransactionOptions represents options for MongoDB transactions.
// These options control the behavior of transactions created by the WithTransaction method.
//
// For more information on MongoDB transactions, see the MongoDB documentation:
// https://docs.mongodb.com/manual/core/transactions/
type TransactionOptions struct {
	// ReadPreference determines which nodes in a replica set are used for read operations.
	// Valid values are:
	// - "primary": Read from the primary node only (default)
	// - "primaryPreferred": Read from the primary if available, otherwise from a secondary
	// - "secondary": Read from a secondary node only
	// - "secondaryPreferred": Read from a secondary if available, otherwise from the primary
	// - "nearest": Read from the node with the lowest network latency
	ReadPreference string

	// ReadConcern determines the consistency and isolation properties of read operations.
	// Valid values are:
	// - "local": Returns the most recent data available on the node (default)
	// - "majority": Returns data that has been acknowledged by a majority of nodes
	// - "linearizable": Returns data that reflects all successful writes that completed before the read
	// - "snapshot": Returns data from a consistent snapshot of the database
	// - "available": Returns data from the instance with no guarantee of durability
	ReadConcern string

	// WriteConcern determines the level of acknowledgment requested from MongoDB for write operations.
	// Valid values are:
	// - "majority": Wait for a majority of nodes to acknowledge the write (default)
	// - "1", "2", etc.: Wait for the specified number of nodes to acknowledge the write
	WriteConcern string

	// MaxCommitTime is the maximum amount of time to allow for a transaction to commit.
	// If the transaction takes longer than this to commit, it will be aborted.
	MaxCommitTime time.Duration

	// RetryWrites determines whether write operations are automatically retried if they fail due to transient errors.
	// This is only applicable for MongoDB 4.2 and later.
	RetryWrites bool

	// RetryReads determines whether read operations are automatically retried if they fail due to transient errors.
	// This is only applicable for MongoDB 4.2 and later.
	RetryReads bool
}

// DefaultOptions returns the default options for the storage.
// These defaults provide a good starting point for most applications,
// but can be customized as needed.
//
// The default options include:
// - Unlimited retries for optimistic concurrency control
// - Exponential backoff with jitter for retries
// - 30-second operation timeout
// - "vector_clock" as the version field
// - 24-hour cache TTL
// - Enabled change stream watching
// - "updateLookup" for full document in change streams
// - "majority" write concern for transactions
func DefaultOptions() *Options {
	return &Options{
		// Concurrency control defaults
		MaxRetries:       0, // 0 means unlimited retries
		RetryDelay:       time.Millisecond * 100,
		MaxRetryDelay:    time.Second * 2,
		RetryJitter:      0.1,
		OperationTimeout: time.Second * 30,
		VersionField:     "vector_clock",

		// Cache defaults
		CacheTTL:          time.Hour * 24,
		CacheQueryResults: true,

		// Watch defaults
		WatchEnabled: true,
		WatchFilter: []bson.D{
			{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "delete"}}}}}}},
		},
		WatchFullDocument: "updateLookup",
		WatchMaxAwaitTime: time.Second * 1,
		WatchBatchSize:    100,

		// Section defaults
		SectionVersionField: "vector_clock",

		// Transaction defaults
		DefaultTransactionOptions: &TransactionOptions{
			ReadPreference: "primary",
			ReadConcern:    "local",
			WriteConcern:   "majority",
			MaxCommitTime:  time.Second * 10,
			RetryWrites:    true,
			RetryReads:     true,
		},
	}
}

// WithMaxRetries sets the maximum number of retries for optimistic concurrency control.
// When a version conflict occurs, the operation will be retried up to this many times.
// A value of 0 means unlimited retries (bounded by the context timeout).
//
// Example:
//
//	result, diff, err := storage.FindOneAndUpdate(ctx, id, updateFn, WithMaxRetries(5))
func WithMaxRetries(maxRetries int) EditOption {
	return func(opts *EditOptions) {
		opts.MaxRetries = maxRetries
	}
}

// WithRetryDelay sets the initial delay between retry attempts.
// This delay increases exponentially with each retry, up to MaxRetryDelay.
//
// Example:
//
//	result, diff, err := storage.FindOneAndUpdate(ctx, id, updateFn, WithRetryDelay(time.Millisecond * 100))
func WithRetryDelay(delay time.Duration) EditOption {
	return func(opts *EditOptions) {
		opts.RetryDelay = int64(delay)
	}
}

// WithMaxRetryDelay sets the maximum delay between retry attempts.
// The delay will not exceed this value, regardless of how many retries have occurred.
//
// Example:
//
//	result, diff, err := storage.FindOneAndUpdate(ctx, id, updateFn, WithMaxRetryDelay(time.Second * 2))
func WithMaxRetryDelay(maxDelay time.Duration) EditOption {
	return func(opts *EditOptions) {
		opts.MaxRetryDelay = int64(maxDelay)
	}
}

// WithRetryJitter sets the jitter factor for retry delays.
// The actual delay will be the base delay plus a random value between 0 and RetryJitter * base delay.
// Should be a value between 0.0 and 1.0.
//
// This helps prevent the "thundering herd" problem where many clients retry at exactly the same time.
//
// Example:
//
//	result, diff, err := storage.FindOneAndUpdate(ctx, id, updateFn, WithRetryJitter(0.1))
func WithRetryJitter(jitter float64) EditOption {
	return func(opts *EditOptions) {
		opts.RetryJitter = jitter
	}
}

// WithTimeout sets the maximum time to spend on the operation, including all retries.
// If the operation takes longer than this, it will be aborted with a timeout error.
// A value of 0 means no timeout (bounded by the context timeout).
//
// Example:
//
//	result, diff, err := storage.FindOneAndUpdate(ctx, id, updateFn, WithTimeout(time.Second * 10))
func WithTimeout(timeout time.Duration) EditOption {
	return func(opts *EditOptions) {
		opts.Timeout = int64(timeout)
	}
}

// NewEditOptions creates a new EditOptions with the given options applied.
// This function is used internally by the Storage implementation to create
// EditOptions from the provided EditOption functions.
//
// The default values are:
// - Unlimited retries (MaxRetries = 0)
// - 10ms initial retry delay
// - 100ms maximum retry delay
// - 0.1 jitter factor
// - 10-second timeout
//
// Example:
//
//	options := NewEditOptions(
//	    WithMaxRetries(5),
//	    WithRetryDelay(time.Millisecond * 100),
//	    WithTimeout(time.Second * 10),
//	)
func NewEditOptions(opts ...EditOption) *EditOptions {
	// Default values
	options := &EditOptions{
		MaxRetries:    0, // 0 means unlimited retries
		RetryDelay:    int64(time.Millisecond * 10),
		MaxRetryDelay: int64(time.Millisecond * 100),
		RetryJitter:   0.1,
		Timeout:       int64(time.Second * 10),
	}

	// Apply options
	for _, opt := range opts {
		opt(options)
	}

	return options
}
