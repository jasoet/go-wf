package chunk

import (
	"cmp"
	"context"
)

// PartitionFetcher fetches records of type T for a single partition [start, end).
// Implementations should respect ctx cancellation. The returned slice may be
// empty (no records in range) but should not be nil unless an error is also
// returned.
type PartitionFetcher[T any, K cmp.Ordered] func(ctx context.Context, start, end K) ([]T, error)
