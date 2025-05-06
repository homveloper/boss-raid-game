package crdtpatch

import (
	"fmt"
	"tictactoe/luvjson/common"
)

// NewSetOperation creates a new operation that sets a value.
func NewSetOperation(targetID common.LogicalTimestamp, value any) (Operation, error) {
	return &InsOperation{
		ID:       common.LogicalTimestamp{SID: common.NewSessionID(), Counter: 1},
		TargetID: targetID,
		Value:    value,
	}, nil
}

// NewObjectInsertOperation creates a new operation that inserts a field into an object.
func NewObjectInsertOperation(targetID common.LogicalTimestamp, key string, valueID common.LogicalTimestamp) (Operation, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	return &InsOperation{
		ID:       common.LogicalTimestamp{SID: common.NewSessionID(), Counter: 1},
		TargetID: targetID,
		Value:    map[string]interface{}{key: valueID},
	}, nil
}

// NewObjectDeleteOperation creates a new operation that deletes a field from an object.
func NewObjectDeleteOperation(targetID common.LogicalTimestamp, key string) (Operation, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	return &DelOperation{
		ID:       common.LogicalTimestamp{SID: common.NewSessionID(), Counter: 1},
		TargetID: targetID,
		Key:      key,
	}, nil
}

// NewArrayInsertOperation creates a new operation that inserts an element into an array.
func NewArrayInsertOperation(targetID common.LogicalTimestamp, index int, valueID common.LogicalTimestamp) (Operation, error) {
	if index < 0 {
		return nil, fmt.Errorf("index cannot be negative")
	}

	// For array operations, we use a special format for the value
	// The key is the index as a string, and the value is the value ID
	return &InsOperation{
		ID:       common.LogicalTimestamp{SID: common.NewSessionID(), Counter: 1},
		TargetID: targetID,
		Value:    map[string]interface{}{fmt.Sprintf("%d", index): valueID},
	}, nil
}

// NewArrayDeleteOperation creates a new operation that deletes an element from an array.
func NewArrayDeleteOperation(targetID common.LogicalTimestamp, index int) (Operation, error) {
	if index < 0 {
		return nil, fmt.Errorf("index cannot be negative")
	}

	// For array operations, we use a special format for the key
	// The key is the index as a string
	return &DelOperation{
		ID:       common.LogicalTimestamp{SID: common.NewSessionID(), Counter: 1},
		TargetID: targetID,
		Key:      fmt.Sprintf("%d", index),
	}, nil
}

// NewStringInsertOperation creates a new operation that inserts text into a string.
func NewStringInsertOperation(targetID common.LogicalTimestamp, index int, text string) (Operation, error) {
	if index < 0 {
		return nil, fmt.Errorf("index cannot be negative")
	}
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// For string operations, we use a special format for the value
	// The value is a map with the index as a string key and the text as the value
	return &InsOperation{
		ID:       common.LogicalTimestamp{SID: common.NewSessionID(), Counter: 1},
		TargetID: targetID,
		Value:    map[string]interface{}{fmt.Sprintf("%d", index): text},
	}, nil
}

// NewStringDeleteOperation creates a new operation that deletes text from a string.
func NewStringDeleteOperation(targetID common.LogicalTimestamp, start, end int) (Operation, error) {
	if start < 0 {
		return nil, fmt.Errorf("start index cannot be negative")
	}
	if end <= start {
		return nil, fmt.Errorf("end index must be greater than start index")
	}

	// For string delete operations, we use StartID and EndID to represent the range
	// Since we don't have actual character IDs, we'll create dummy IDs based on the indices
	dummySID := common.NewSessionID()
	startID := common.LogicalTimestamp{SID: dummySID, Counter: uint64(start)}
	endID := common.LogicalTimestamp{SID: dummySID, Counter: uint64(end)}

	return &DelOperation{
		ID:       common.LogicalTimestamp{SID: common.NewSessionID(), Counter: 1},
		TargetID: targetID,
		StartID:  startID,
		EndID:    endID,
	}, nil
}
