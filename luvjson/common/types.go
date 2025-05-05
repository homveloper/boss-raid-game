package common

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// SessionID represents a unique identifier for a session.
// It is implemented as a UUID v7 which provides time-ordered values.
type SessionID uuid.UUID

// NilSessionID is the zero value for SessionID.
var NilSessionID SessionID

// RootID is the fixed LogicalTimestamp used for the root node.
var RootID = LogicalTimestamp{SID: NilSessionID, Counter: 0}

// NilID is the zero value for LogicalTimestamp.
var NilID = LogicalTimestamp{SID: NilSessionID, Counter: 0}

// NewSessionID creates a new SessionID using UUID v7.
// It panics if the UUID cannot be created.
func NewSessionID() SessionID {
	const retry = 3

	var lastErr error
	var id uuid.UUID
	for i := 0; i < retry; i++ {
		id, lastErr = uuid.NewV7()
	}

	if lastErr != nil {
		panic(lastErr)
	}

	return SessionID(id)
}

// SessionIDFromUint64 creates a SessionID from a uint64 value.
// This is used when parsing patch operations where the session ID is represented as a number.
func SessionIDFromUint64(id uint64) SessionID {
	// Convert the uint64 to a UUID
	// For simplicity, we'll just use the lower 16 bytes of the uint64
	uuidBytes := make([]byte, 16)

	// Put the uint64 in the first 8 bytes
	uuidBytes[0] = byte(id >> 56)
	uuidBytes[1] = byte(id >> 48)
	uuidBytes[2] = byte(id >> 40)
	uuidBytes[3] = byte(id >> 32)
	uuidBytes[4] = byte(id >> 24)
	uuidBytes[5] = byte(id >> 16)
	uuidBytes[6] = byte(id >> 8)
	uuidBytes[7] = byte(id)

	// Create a UUID from the bytes
	uuidVal, err := uuid.FromBytes(uuidBytes)
	if err != nil {
		// If there's an error, return NilSessionID
		return NilSessionID
	}

	return SessionID(uuidVal)
}

// String returns the string representation of the SessionID.
func (s SessionID) String() string {
	return uuid.UUID(s).String()
}

// Compare compares two SessionIDs.
// Returns:
//
//	-1 if s < other
//	 0 if s == other
//	 1 if s > other
func (s SessionID) Compare(other SessionID) int {
	// Compare UUIDs lexicographically
	max := len(uuid.UUID(s))
	for i := 0; i < max; i++ {
		if uuid.UUID(s)[i] < uuid.UUID(other)[i] {
			return -1
		}
		if uuid.UUID(s)[i] > uuid.UUID(other)[i] {
			return 1
		}
	}
	return 0
}

// MarshalJSON implements the json.Marshaler interface.
func (s SessionID) MarshalJSON() ([]byte, error) {
	type alias SessionID
	return json.Marshal(alias(s))
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (s *SessionID) UnmarshalText(text []byte) error {
	// Parse the UUID from string
	u, err := uuid.Parse(string(text))
	if err != nil {
		return fmt.Errorf("invalid UUID format: %w", err)
	}
	*s = SessionID(u)
	return nil
}

// MarshalText implements the encoding.TextMarshaler interface.
func (s SessionID) MarshalText() ([]byte, error) {
	return []byte(uuid.UUID(s).String()), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (s *SessionID) UnmarshalJSON(data []byte) error {
	type alias SessionID
	return json.Unmarshal(data, (*alias)(s))
}

// LogicalTimestamp represents a globally unique identifier that can be partially ordered.
// It consists of a session ID (UUID v7) and a sequence counter.
type LogicalTimestamp struct {
	SID     SessionID `json:"sid"`
	Counter uint64    `json:"cnt"`
}

// Compare compares two logical timestamps.
// Returns:
//
//	-1 if t < other
//	 0 if t == other
//	 1 if t > other
func (t LogicalTimestamp) Compare(other LogicalTimestamp) int {
	// First compare SessionIDs
	sidCompare := t.SID.Compare(other.SID)
	if sidCompare != 0 {
		return sidCompare
	}

	// If SessionIDs are equal, compare counters
	if t.Counter < other.Counter {
		return -1
	}
	if t.Counter > other.Counter {
		return 1
	}
	return 0
}

// Next returns the next logical timestamp in the sequence.
func (t LogicalTimestamp) Next() LogicalTimestamp {
	return LogicalTimestamp{
		SID:     t.SID,
		Counter: t.Counter + 1,
	}
}

// Increment increments the counter by the given amount.
func (t LogicalTimestamp) Increment(amount uint64) LogicalTimestamp {
	return LogicalTimestamp{
		SID:     t.SID,
		Counter: t.Counter + amount,
	}
}

// String returns a string representation of the logical timestamp.
func (t LogicalTimestamp) String() string {
	data, _ := json.Marshal(t)
	return string(data)
}

// MarshalJSON implements the json.Marshaler interface.
func (t LogicalTimestamp) MarshalJSON() ([]byte, error) {
	type alias LogicalTimestamp
	return json.Marshal(alias(t))
}

// UnmarshalJSON implements the json.Unmarshaler interface.
func (t *LogicalTimestamp) UnmarshalJSON(data []byte) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}

	// Handle new format
	sidVal, ok := obj["sid"]
	if !ok {
		return ErrInvalidOperation{Message: "missing sid field"}
	}

	cntVal, ok := obj["cnt"]
	if !ok {
		return ErrInvalidOperation{Message: "missing cnt field"}
	}

	// Marshal the sid value back to JSON
	sidJSON, err := json.Marshal(sidVal)
	if err != nil {
		return fmt.Errorf("failed to marshal sid: %w", err)
	}

	// Unmarshal the sid value using SessionID's UnmarshalJSON
	var sid SessionID
	if err := sid.UnmarshalJSON(sidJSON); err != nil {
		return fmt.Errorf("failed to unmarshal sid: %w", err)
	}

	// Set the SID
	t.SID = sid

	// Parse Counter
	switch v := cntVal.(type) {
	case float64:
		t.Counter = uint64(v)
	case int:
		t.Counter = uint64(v)
	case int64:
		t.Counter = uint64(v)
	case uint64:
		t.Counter = v
	case json.Number:
		num, err := v.Int64()
		if err != nil {
			return fmt.Errorf("cnt must be a number: %w", err)
		}
		t.Counter = uint64(num)
	default:
		return ErrInvalidOperation{Message: "cnt must be a number"}
	}

	return nil
}

// NodeType represents the type of a CRDT node.
type NodeType string

const (
	// NodeTypeCon represents a constant value.
	NodeTypeCon NodeType = "con"
	// NodeTypeVal represents a LWW-Value.
	NodeTypeVal NodeType = "val"
	// NodeTypeObj represents a LWW-Object.
	NodeTypeObj NodeType = "obj"
	// NodeTypeVec represents a LWW-Vector.
	NodeTypeVec NodeType = "vec"
	// NodeTypeStr represents an RGA-String.
	NodeTypeStr NodeType = "str"
	// NodeTypeBin represents an RGA-Binary blob.
	NodeTypeBin NodeType = "bin"
	// NodeTypeArr represents an RGA-Array.
	NodeTypeArr NodeType = "arr"
	// NodeTypeRoot represents the root node of a document.
	NodeTypeRoot NodeType = "root"
)

// OperationType represents the type of a CRDT patch operation.
type OperationType string

const (
	// OperationTypeNew creates a new CRDT node.
	OperationTypeNew OperationType = "new"
	// OperationTypeIns updates an existing CRDT node.
	OperationTypeIns OperationType = "ins"
	// OperationTypeDel deletes contents from an existing CRDT node.
	OperationTypeDel OperationType = "del"
	// OperationTypeNop is a no-op operation.
	OperationTypeNop OperationType = "nop"
)

// EncodingFormat represents the format used to encode CRDT documents and patches.
type EncodingFormat string

const (
	// EncodingFormatVerbose is a verbose human-readable JSON encoding.
	EncodingFormatVerbose EncodingFormat = "verbose"
	// EncodingFormatCompact is a JSON encoding which follows Compact JSON encoding scheme.
	EncodingFormatCompact EncodingFormat = "compact"
	// EncodingFormatBinary is a custom designed minimal binary encoding.
	EncodingFormatBinary EncodingFormat = "binary"
)
