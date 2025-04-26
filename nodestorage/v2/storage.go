// Package v2 provides an enhanced version of the nodestorage package with improved
// MongoDB integration, optimistic concurrency control, and multiple caching strategies.
//
// This package is designed for distributed environments where data consistency
// is critical while maintaining high performance. It offers a generic Storage interface
// that works with any type implementing the Cachable interface.
//
// Key features:
//   - Optimistic concurrency control using version fields
//   - Multiple cache implementations (Memory, BadgerDB, Redis)
//   - Section-based versioning for fine-grained concurrency control
//   - MongoDB native operations support (update operators, aggregation pipelines)
//   - Transaction support for atomic operations
//   - Change stream watching for real-time updates
//   - Generic support for any document type
//
// Basic usage example:
//
//	// Connect to MongoDB
//	client, _ := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
//	collection := client.Database("mydb").Collection("documents")
//
//	// Create a memory cache
//	memCache := cache.NewMemoryCache[*MyDocument](nil)
//
//	// Create storage with options
//	options := &v2.Options{VersionField: "vector_clock"}
//	storage, _ := v2.NewStorage[*MyDocument](ctx, client, collection, memCache, options)
//
//	// Use the storage
//	doc, _ := storage.FindOne(ctx, id)
package v2

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Cachable is an interface for objects that can be cached and stored in the database.
// The type parameter T must be a pointer type to ensure proper modification.
//
// Implementing types must provide a Copy method that creates a deep copy of the object.
// This is essential for the optimistic concurrency control mechanism to work correctly,
// as it needs to compare and manipulate copies of documents without affecting the originals.
//
// Example implementation:
//
//	type Document struct {
//	    ID          primitive.ObjectID `bson:"_id"`
//	    Name        string             `bson:"name"`
//	    Value       int                `bson:"value"`
//	    VectorClock int64              `bson:"vector_clock"`
//	}
//
//	func (d *Document) Copy() *Document {
//	    if d == nil {
//	        return nil
//	    }
//	    return &Document{
//	        ID:          d.ID,
//	        Name:        d.Name,
//	        Value:       d.Value,
//	        VectorClock: d.VectorClock,
//	    }
//	}
type Cachable[T any] interface {
	// Copy creates a deep copy of the object.
	// It must not share any mutable state with the original object.
	Copy() T
}

// EditFunc is a function type that defines how to modify a document.
// It takes a document of type T, applies modifications to it, and returns the modified document
// along with any error that occurred during the modification process.
//
// This function type is used primarily with FindOneAndUpdate to provide a way to
// modify documents with optimistic concurrency control.
//
// Example usage:
//
//	updateFn := func(doc *Document) (*Document, error) {
//	    doc.Name = "Updated Name"
//	    doc.Value += 10
//	    return doc, nil
//	}
//
//	updatedDoc, diff, err := storage.FindOneAndUpdate(ctx, id, updateFn)
type EditFunc[T Cachable[T]] func(doc T) (T, error)

// WatchEvent represents a document change event emitted by the Watch method.
// It contains information about what changed in the document, including the document ID,
// the type of operation that occurred, the current state of the document, and optionally
// a diff showing what changed.
//
// The Operation field can have one of the following values:
//   - "create": A new document was created
//   - "update": An existing document was updated
//   - "delete": A document was deleted
//
// For "delete" operations, the Data field may be nil or contain the last known state
// of the document before deletion, depending on the MongoDB change stream configuration.
type WatchEvent[T Cachable[T]] struct {
	// ID is the unique identifier of the document that changed
	ID primitive.ObjectID `json:"id"`

	// Operation indicates the type of change: "create", "update", or "delete"
	Operation string `json:"operation"`

	// Data contains the current state of the document after the change
	// For delete operations, this may be nil or the last known state
	Data T `json:"data"`

	// Diff contains the differences between the previous and current versions
	// This is only populated for update operations
	Diff *Diff `json:"diff,omitempty"`
}

// Diff represents the difference between two document versions.
// It provides two different formats for representing the changes:
//  1. JSONPatch: A sequence of operations to transform one document into another (RFC 6902)
//  2. MergePatch: A partial document that can be merged with the original (RFC 7396)
//
// These formats allow clients to efficiently apply changes to their local copies
// without having to transfer the entire document.
type Diff struct {
	// JSONPatch contains operations according to RFC 6902 JSON Patch specification.
	// This is typically an array of operations like add, remove, replace, move, copy, and test.
	// The type is interface{} to accommodate different JSON patch implementations.
	JSONPatch interface{} `json:"jsonPatch,omitempty"`

	// MergePatch contains a partial document according to RFC 7396 JSON Merge Patch specification.
	// This is a simpler alternative to JSONPatch where the patch is just the parts of the
	// document that changed, with null values indicating deletions.
	MergePatch []byte `json:"mergePatch,omitempty"`
}

// Storage is the main interface for document storage operations with generic support.
// It provides enhanced MongoDB integration while maintaining optimistic concurrency control
// through a version field in the documents.
//
// The interface is generic over type T, which must implement the Cachable interface.
// This allows the storage to work with any document type that can be copied.
//
// The implementation uses a cache to improve read performance, with various cache
// backends available (Memory, BadgerDB, Redis). The cache is automatically invalidated
// when documents are modified.
type Storage[T Cachable[T]] interface {
	// Basic CRUD operations

	// FindOne retrieves a document by its ID.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - id: The unique identifier of the document to retrieve
	//   - opts: Optional MongoDB FindOne options
	//
	// Returns:
	//   - The retrieved document
	//   - ErrNotFound if the document does not exist
	//   - Other errors that may occur during the operation
	FindOne(ctx context.Context, id primitive.ObjectID, opts ...*options.FindOneOptions) (T, error)

	// FindMany retrieves multiple documents that match the given filter.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - filter: A MongoDB query filter to match documents
	//   - opts: Optional MongoDB Find options (sorting, projection, etc.)
	//
	// Returns:
	//   - A slice of matching documents
	//   - An empty slice if no documents match
	//   - Any error that occurred during the operation
	FindMany(ctx context.Context, filter interface{}, opts ...*options.FindOptions) ([]T, error)

	// FindOneAndUpsert creates a new document or returns the existing one if it already exists.
	// This function is safe to use in distributed environments as it implements "CreateIfNotExists" semantics.
	//
	// If the document already exists (based on its ID), the existing document is returned without modification.
	// If the document does not exist, it is created with an initial version number.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - data: The document to create or retrieve
	//
	// Returns:
	//   - The created or existing document
	//   - Any error that occurred during the operation
	FindOneAndUpsert(ctx context.Context, data T) (T, error)

	// FindOneAndUpdate edits a document with optimistic concurrency control using a function.
	// The function is called with the current version of the document, and the returned document
	// is saved if the version has not changed in the meantime.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - id: The unique identifier of the document to update
	//   - updateFn: A function that modifies the document
	//   - opts: Optional edit options (retries, timeouts, etc.)
	//
	// Returns:
	//   - The updated document
	//   - A diff showing what changed in the document
	//   - ErrNotFound if the document does not exist
	//   - ErrVersionMismatch if the document was modified concurrently
	//   - Other errors that may occur during the operation
	FindOneAndUpdate(ctx context.Context, id primitive.ObjectID, updateFn EditFunc[T], opts ...EditOption) (T, *Diff, error)

	// DeleteOne deletes a document by its ID.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - id: The unique identifier of the document to delete
	//
	// Returns:
	//   - nil if the document was deleted successfully or did not exist
	//   - Any error that occurred during the operation
	DeleteOne(ctx context.Context, id primitive.ObjectID) error

	// MongoDB native feature access

	// UpdateOne allows direct use of MongoDB update operators while maintaining optimistic concurrency control.
	// This provides access to MongoDB's powerful update capabilities while still ensuring
	// that concurrent updates do not overwrite each other.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - id: The unique identifier of the document to update
	//   - update: A MongoDB update document with operators like $set, $inc, etc.
	//   - opts: Optional edit options (retries, timeouts, etc.)
	//
	// Returns:
	//   - The updated document
	//   - ErrNotFound if the document does not exist
	//   - ErrVersionMismatch if the document was modified concurrently
	//   - Other errors that may occur during the operation
	UpdateOne(ctx context.Context, id primitive.ObjectID, update bson.M, opts ...EditOption) (T, error)

	// UpdateOneWithPipeline allows use of MongoDB aggregation pipeline for updates while maintaining optimistic concurrency control.
	// This provides access to MongoDB's aggregation pipeline for complex updates while still ensuring
	// that concurrent updates do not overwrite each other.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - id: The unique identifier of the document to update
	//   - pipeline: A MongoDB aggregation pipeline for the update
	//   - opts: Optional edit options (retries, timeouts, etc.)
	//
	// Returns:
	//   - The updated document
	//   - ErrNotFound if the document does not exist
	//   - ErrVersionMismatch if the document was modified concurrently
	//   - Other errors that may occur during the operation
	UpdateOneWithPipeline(ctx context.Context, id primitive.ObjectID, pipeline mongo.Pipeline, opts ...EditOption) (T, error)

	// Section-based concurrency control

	// UpdateSection edits a specific section of a document with optimistic concurrency control.
	// This allows for fine-grained concurrency control where different sections of a document
	// can be updated independently without conflicts.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - id: The unique identifier of the document to update
	//   - sectionPath: The path to the section within the document (e.g., "metadata", "items.0")
	//   - updateFn: A function that modifies the section
	//   - opts: Optional edit options (retries, timeouts, etc.)
	//
	// Returns:
	//   - The updated document
	//   - ErrNotFound if the document does not exist
	//   - ErrVersionMismatch if the section was modified concurrently
	//   - Other errors that may occur during the operation
	UpdateSection(ctx context.Context, id primitive.ObjectID, sectionPath string, updateFn func(interface{}) (interface{}, error), opts ...EditOption) (T, error)

	// Transaction support

	// WithTransaction executes the provided function within a MongoDB transaction.
	// This ensures that multiple operations are executed atomically, either all succeeding
	// or all failing together.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - fn: A function that executes operations within the transaction
	//
	// Returns:
	//   - Any error that occurred during the transaction
	WithTransaction(ctx context.Context, fn func(sessCtx mongo.SessionContext) error) error

	// Change monitoring

	// Watch watches for changes to documents with optional MongoDB pipeline and options.
	// This provides real-time notifications when documents are created, updated, or deleted.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - pipeline: A MongoDB aggregation pipeline to filter change events
	//   - opts: Optional MongoDB ChangeStream options
	//
	// Returns:
	//   - A channel that receives change events
	//   - Any error that occurred while setting up the watch
	Watch(ctx context.Context, pipeline mongo.Pipeline, opts ...*options.ChangeStreamOptions) (<-chan WatchEvent[T], error)

	// Utility methods

	// Collection returns the underlying MongoDB collection.
	// This provides direct access to the MongoDB collection for operations
	// not covered by the Storage interface.
	//
	// Returns:
	//   - The MongoDB collection used by this storage
	Collection() *mongo.Collection

	// Close closes the storage and releases any resources it holds.
	// This should be called when the storage is no longer needed to ensure
	// proper cleanup of resources.
	//
	// Returns:
	//   - Any error that occurred while closing the storage
	Close() error
}

// EditOption is a function that configures EditOptions.
// This follows the functional options pattern, allowing for flexible and extensible
// configuration of edit operations.
//
// Example usage:
//
//	result, diff, err := storage.FindOneAndUpdate(ctx, id, updateFn,
//	    WithMaxRetries(5),
//	    WithRetryDelay(time.Millisecond * 100),
//	    WithTimeout(time.Second * 10),
//	)
type EditOption func(*EditOptions)

// EditOptions represents options for document editing operations.
// These options control the behavior of optimistic concurrency control,
// including retry behavior and timeouts.
type EditOptions struct {
	// MaxRetries is the maximum number of retry attempts for optimistic concurrency control.
	// When a version conflict occurs, the operation will be retried up to this many times.
	// A value of 0 means unlimited retries (bounded by the context timeout).
	MaxRetries int

	// RetryDelay is the initial delay between retry attempts in nanoseconds.
	// This delay increases exponentially with each retry, up to MaxRetryDelay.
	RetryDelay int64

	// MaxRetryDelay is the maximum delay between retry attempts in nanoseconds.
	// The delay will not exceed this value, regardless of how many retries have occurred.
	MaxRetryDelay int64

	// RetryJitter is a random factor applied to retry delays to prevent thundering herd problems.
	// The actual delay will be the base delay plus a random value between 0 and RetryJitter * base delay.
	// Should be a value between 0.0 and 1.0.
	RetryJitter float64

	// Timeout is the maximum time in nanoseconds to spend on the operation, including all retries.
	// If the operation takes longer than this, it will be aborted with a timeout error.
	// A value of 0 means no timeout (bounded by the context timeout).
	Timeout int64
}
