package common

import (
	"fmt"
)

// ErrInvalidNodeType is returned when an invalid node type is encountered.
type ErrInvalidNodeType struct {
	Type string
}

func (e ErrInvalidNodeType) Error() string {
	return fmt.Sprintf("invalid node type: %s", e.Type)
}

// ErrInvalidOperationType is returned when an invalid operation type is encountered.
type ErrInvalidOperationType struct {
	Type string
}

func (e ErrInvalidOperationType) Error() string {
	return fmt.Sprintf("invalid operation type: %s", e.Type)
}

// ErrInvalidEncoding is returned when an invalid encoding format is encountered.
type ErrInvalidEncoding struct {
	Format string
}

func (e ErrInvalidEncoding) Error() string {
	return fmt.Sprintf("invalid encoding format: %s", e.Format)
}

// ErrNodeNotFound is returned when a node with the specified ID is not found.
type ErrNodeNotFound struct {
	ID LogicalTimestamp
}

func (e ErrNodeNotFound) Error() string {
	return fmt.Sprintf("node not found: %v", e.ID)
}

// ErrInvalidOperation is returned when an operation is invalid.
type ErrInvalidOperation struct {
	Message string
}

func (e ErrInvalidOperation) Error() string {
	return fmt.Sprintf("invalid operation: %s", e.Message)
}

// ErrInvalidNode is returned when a node is invalid.
type ErrInvalidNode struct {
	Message string
}

func (e ErrInvalidNode) Error() string {
	return fmt.Sprintf("invalid node: %s", e.Message)
}

// ErrNotFound is returned when a resource is not found.
type ErrNotFound struct {
	Message string
}

func (e ErrNotFound) Error() string {
	return fmt.Sprintf("not found: %s", e.Message)
}
