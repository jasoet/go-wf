package datasync

import "context"

// Source produces data of type T from an external system.
type Source[T any] interface {
	// Name returns a unique identifier for this source.
	Name() string
	// Fetch retrieves all records from the source.
	Fetch(ctx context.Context) ([]T, error)
}

// ParamSource is a Source that exposes its configuration parameters.
// Implement this interface to make source parameters visible in Temporal UI.
type ParamSource[T any, P any] interface {
	Source[T]
	// Params returns the source's configuration parameters.
	Params() P
}
