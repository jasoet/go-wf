package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// JSONCodec serializes and deserializes values as JSON.
type JSONCodec[T any] struct{}

// Encode serializes a value to JSON.
func (c *JSONCodec[T]) Encode(value T) (io.Reader, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("json encode: %w", err)
	}
	return bytes.NewReader(data), nil
}

// Decode deserializes a value from JSON.
func (c *JSONCodec[T]) Decode(reader io.Reader) (T, error) {
	var zero T

	data, err := io.ReadAll(reader)
	if err != nil {
		return zero, fmt.Errorf("json decode read: %w", err)
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return zero, fmt.Errorf("json decode: %w", err)
	}
	return value, nil
}

// BytesCodec is a pass-through codec for []byte values.
type BytesCodec struct{}

// Encode wraps the byte slice in a reader.
func (c *BytesCodec) Encode(value []byte) (io.Reader, error) {
	return bytes.NewReader(value), nil
}

// Decode reads all bytes from the reader.
func (c *BytesCodec) Decode(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("bytes decode: %w", err)
	}
	return data, nil
}
