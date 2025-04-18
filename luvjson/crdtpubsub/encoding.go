package crdtpubsub

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"tictactoe/luvjson/crdtpatch"
)

// Encoder encodes a CRDT patch into a byte array using the specified format.
type Encoder interface {
	// Encode encodes a CRDT patch into a byte array.
	Encode(patch *crdtpatch.Patch) ([]byte, error)
}

// Decoder decodes a byte array into a CRDT patch using the specified format.
type Decoder interface {
	// Decode decodes a byte array into a CRDT patch.
	Decode(data []byte) (*crdtpatch.Patch, error)
}

// EncoderDecoder combines the Encoder and Decoder interfaces.
type EncoderDecoder interface {
	Encoder
	Decoder
}

// JSONEncoderDecoder implements the EncoderDecoder interface using JSON encoding.
type JSONEncoderDecoder struct{}

// Encode encodes a CRDT patch into a JSON byte array.
func (ed *JSONEncoderDecoder) Encode(patch *crdtpatch.Patch) ([]byte, error) {
	return json.Marshal(patch)
}

// Decode decodes a JSON byte array into a CRDT patch.
func (ed *JSONEncoderDecoder) Decode(data []byte) (*crdtpatch.Patch, error) {
	var patch crdtpatch.Patch
	if err := json.Unmarshal(data, &patch); err != nil {
		return nil, err
	}
	return &patch, nil
}

// BinaryEncoderDecoder implements the EncoderDecoder interface using binary encoding.
// Note: This is a placeholder. Actual binary encoding would depend on the specific requirements.
type BinaryEncoderDecoder struct{}

// Encode encodes a CRDT patch into a binary byte array.
func (ed *BinaryEncoderDecoder) Encode(patch *crdtpatch.Patch) ([]byte, error) {
	// Convert to JSON first, then we could implement a more efficient binary encoding
	jsonData, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	// For now, just return the JSON data as binary
	// In a real implementation, this would use a more efficient binary format
	return jsonData, nil
}

// Decode decodes a binary byte array into a CRDT patch.
func (ed *BinaryEncoderDecoder) Decode(data []byte) (*crdtpatch.Patch, error) {
	// For now, just treat the binary data as JSON
	// In a real implementation, this would use a more efficient binary format
	var patch crdtpatch.Patch
	if err := json.Unmarshal(data, &patch); err != nil {
		return nil, err
	}
	return &patch, nil
}

// TextEncoderDecoder implements the EncoderDecoder interface using text encoding.
type TextEncoderDecoder struct{}

// Encode encodes a CRDT patch into a text byte array.
func (ed *TextEncoderDecoder) Encode(patch *crdtpatch.Patch) ([]byte, error) {
	// Convert to JSON, which is already text-based
	return json.Marshal(patch)
}

// Decode decodes a text byte array into a CRDT patch.
func (ed *TextEncoderDecoder) Decode(data []byte) (*crdtpatch.Patch, error) {
	var patch crdtpatch.Patch
	if err := json.Unmarshal(data, &patch); err != nil {
		return nil, err
	}
	return &patch, nil
}

// Base64EncoderDecoder implements the EncoderDecoder interface using base64 encoding.
type Base64EncoderDecoder struct {
	// The underlying encoder/decoder to use before/after base64 encoding/decoding.
	underlying EncoderDecoder
}

// NewBase64EncoderDecoder creates a new Base64EncoderDecoder with the specified underlying encoder/decoder.
func NewBase64EncoderDecoder(underlying EncoderDecoder) *Base64EncoderDecoder {
	if underlying == nil {
		underlying = &JSONEncoderDecoder{}
	}
	return &Base64EncoderDecoder{
		underlying: underlying,
	}
}

// Encode encodes a CRDT patch into a base64 byte array.
func (ed *Base64EncoderDecoder) Encode(patch *crdtpatch.Patch) ([]byte, error) {
	// Encode using the underlying encoder
	data, err := ed.underlying.Encode(patch)
	if err != nil {
		return nil, err
	}
	// Encode to base64
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(encoded, data)
	return encoded, nil
}

// Decode decodes a base64 byte array into a CRDT patch.
func (ed *Base64EncoderDecoder) Decode(data []byte) (*crdtpatch.Patch, error) {
	// Decode from base64
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(decoded, data)
	if err != nil {
		return nil, err
	}
	// Decode using the underlying decoder
	return ed.underlying.Decode(decoded[:n])
}

// GetEncoderDecoder returns an EncoderDecoder for the specified format.
func GetEncoderDecoder(format EncodingFormat) (EncoderDecoder, error) {
	switch format {
	case EncodingFormatJSON:
		return &JSONEncoderDecoder{}, nil
	case EncodingFormatBinary:
		return &BinaryEncoderDecoder{}, nil
	case EncodingFormatText:
		return &TextEncoderDecoder{}, nil
	case EncodingFormatBase64:
		return NewBase64EncoderDecoder(&JSONEncoderDecoder{}), nil
	default:
		return nil, fmt.Errorf("unsupported encoding format: %s", format)
	}
}
