package store

import (
	"context"
	"io"
)

// RawStore is a byte-level storage interface.
// Implementations handle raw byte persistence with string keys.
type RawStore interface {
	// Upload stores data under the given key.
	Upload(ctx context.Context, key string, data io.Reader) error

	// Download retrieves data for the given key.
	// The caller must close the returned ReadCloser.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the data stored under the given key.
	Delete(ctx context.Context, key string) error

	// Exists checks whether data exists under the given key.
	Exists(ctx context.Context, key string) (bool, error)

	// List returns all keys matching the given prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Close releases any resources held by the store.
	Close() error
}

// Store is a typed storage interface with automatic serialization.
type Store[T any] interface {
	// Save serializes and stores the value under the given key.
	Save(ctx context.Context, key string, value T) error

	// Load retrieves and deserializes the value stored under the given key.
	Load(ctx context.Context, key string) (T, error)

	// Delete removes the data stored under the given key.
	Delete(ctx context.Context, key string) error

	// Exists checks whether data exists under the given key.
	Exists(ctx context.Context, key string) (bool, error)

	// List returns all keys matching the given prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Close releases any resources held by the store.
	Close() error
}

// Codec defines a serialization strategy for type T.
type Codec[T any] interface {
	// Encode serializes a value into a reader.
	Encode(value T) (io.Reader, error)

	// Decode deserializes a value from a reader.
	Decode(reader io.Reader) (T, error)
}

// TypedStore adapts a RawStore with a Codec to implement Store[T].
type TypedStore[T any] struct {
	raw   RawStore
	codec Codec[T]
}

// NewTypedStore creates a new TypedStore from a RawStore and Codec.
func NewTypedStore[T any](raw RawStore, codec Codec[T]) Store[T] {
	return &TypedStore[T]{
		raw:   raw,
		codec: codec,
	}
}

// Save serializes the value and uploads it to the underlying RawStore.
func (s *TypedStore[T]) Save(ctx context.Context, key string, value T) error {
	reader, err := s.codec.Encode(value)
	if err != nil {
		return err
	}
	return s.raw.Upload(ctx, key, reader)
}

// Load downloads data from the underlying RawStore and deserializes it.
func (s *TypedStore[T]) Load(ctx context.Context, key string) (T, error) {
	var zero T
	rc, err := s.raw.Download(ctx, key)
	if err != nil {
		return zero, err
	}
	defer rc.Close() //nolint:errcheck // best-effort close after read
	return s.codec.Decode(rc)
}

// Delete removes the data stored under the given key.
func (s *TypedStore[T]) Delete(ctx context.Context, key string) error {
	return s.raw.Delete(ctx, key)
}

// Exists checks whether data exists under the given key.
func (s *TypedStore[T]) Exists(ctx context.Context, key string) (bool, error) {
	return s.raw.Exists(ctx, key)
}

// List returns all keys matching the given prefix.
func (s *TypedStore[T]) List(ctx context.Context, prefix string) ([]string, error) {
	return s.raw.List(ctx, prefix)
}

// Close releases any resources held by the underlying RawStore.
func (s *TypedStore[T]) Close() error {
	return s.raw.Close()
}

// NewJSONStore creates a Store[T] that serializes values as JSON.
func NewJSONStore[T any](raw RawStore) Store[T] {
	return NewTypedStore[T](raw, &JSONCodec[T]{})
}

// NewBytesStore creates a Store[[]byte] that passes through raw bytes.
func NewBytesStore(raw RawStore) Store[[]byte] {
	return NewTypedStore[[]byte](raw, &BytesCodec{})
}
