package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// Node represents a CRDT node in the JSON CRDT document.
type Node interface {
	// ID returns the unique identifier of the node.
	ID() common.LogicalTimestamp

	// Type returns the type of the node.
	Type() common.NodeType

	// Value returns the value of the node.
	Value() interface{}

	// MarshalJSON returns a JSON representation of the node.
	json.Marshaler

	// UnmarshalJSON parses a JSON representation of the node.
	json.Unmarshaler
}

// RGAElement represents an element in a Replicated Growable Array.
type RGAElement struct {
	NodeId      common.LogicalTimestamp `json:"id"`
	NodeValue   interface{}             `json:"value"`
	NodeDeleted bool                    `json:"deleted"`
}
