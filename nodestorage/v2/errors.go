package nodestorage

import (
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	// ErrNotFound is returned when a document is not found
	ErrNotFound = errors.New("document not found")

	// ErrVersionMismatch is returned when there's a version conflict during update
	ErrVersionMismatch = errors.New("document version mismatch")

	// ErrInvalidDocument is returned when a document is invalid
	ErrInvalidDocument = errors.New("invalid document")

	// ErrCacheMiss is returned when a document is not found in cache
	ErrCacheMiss = errors.New("cache miss")

	// ErrTimeout is returned when an operation times out
	ErrTimeout = errors.New("operation timed out")

	// ErrMaxRetriesExceeded is returned when max retries are exceeded
	ErrMaxRetriesExceeded = errors.New("maximum retries exceeded")

	// ErrClosed is returned when operating on a closed storage
	ErrClosed = errors.New("storage is closed")

	// ErrMissingVersionField is returned when the version field is not specified
	ErrMissingVersionField = errors.New("version field is required in options")

	// ErrSectionNotFound is returned when a document section is not found
	ErrSectionNotFound = errors.New("document section not found")

	// ErrTransactionFailed is returned when a transaction fails
	ErrTransactionFailed = errors.New("transaction failed")
)

// VersionError represents a version conflict error with details
type VersionError struct {
	CurrentVersion int64
	StoredVersion  int64
	DocumentID     primitive.ObjectID
	SectionPath    string // Optional, for section-based concurrency control
}

// Error implements the error interface
func (e *VersionError) Error() string {
	if e.SectionPath != "" {
		return fmt.Sprintf("version conflict for document %s section %s: current=%d, stored=%d",
			e.DocumentID, e.SectionPath, e.CurrentVersion, e.StoredVersion)
	}
	return fmt.Sprintf("version conflict for document %s: current=%d, stored=%d",
		e.DocumentID, e.CurrentVersion, e.StoredVersion)
}

// Is checks if the error is of a specific type
func (e *VersionError) Is(target error) bool {
	return target == ErrVersionMismatch
}

// Unwrap returns the underlying error
func (e *VersionError) Unwrap() error {
	return ErrVersionMismatch
}

// NewVersionError creates a new version error
func NewVersionError(docID primitive.ObjectID, current, stored int64) *VersionError {
	return &VersionError{
		CurrentVersion: current,
		StoredVersion:  stored,
		DocumentID:     docID,
	}
}

// NewSectionVersionError creates a new section version error
func NewSectionVersionError(docID primitive.ObjectID, sectionPath string, current, stored int64) *VersionError {
	return &VersionError{
		CurrentVersion: current,
		StoredVersion:  stored,
		DocumentID:     docID,
		SectionPath:    sectionPath,
	}
}
