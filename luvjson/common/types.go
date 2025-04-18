package common

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// SessionID represents a unique identifier for a session.
// It is implemented as a UUID v7 which provides time-ordered values.
type SessionID uuid.UUID

// NewSessionID creates a new SessionID using UUID v7.
// It panics if the UUID cannot be created.
func NewSessionID() SessionID {
	uuid, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Sprintf("failed to create SessionID: %v", err))
	}
	return SessionID(uuid)
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
	for i := 0; i < 16; i++ {
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
	// Convert UUID to byte array
	bytes := [16]byte(uuid.UUID(s))
	return json.Marshal(bytes[:])
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
	// Only unmarshal as byte array
	var bytes []byte
	if err := json.Unmarshal(data, &bytes); err != nil {
		// If it's not a byte array, it might be a string
		// In that case, let UnmarshalText handle it
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return fmt.Errorf("sid must be a byte array: %w", err)
		}
		return s.UnmarshalText([]byte(str))
	}

	// Check if we have the correct length
	if len(bytes) != 16 {
		return fmt.Errorf("invalid UUID length: %d", len(bytes))
	}

	// Check if all bytes are zero (Zero UUID)
	allZero := true
	for _, b := range bytes {
		if b != 0 {
			allZero = false
			break
		}
	}

	if allZero {
		// This is a Zero UUID
		*s = SessionID{}
		return nil
	}

	// Regular UUID
	var uuidBytes [16]byte
	copy(uuidBytes[:], bytes)
	*s = SessionID(uuid.UUID(uuidBytes))
	return nil
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
	// Convert UUID to byte array
	bytes := [16]byte(uuid.UUID(t.SID))

	return json.Marshal(map[string]interface{}{
		"sid": bytes[:],
		"cnt": t.Counter,
	})
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
