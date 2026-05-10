package chunk

import (
	"cmp"
	"context"
)

// ProgressTracker persists progress so a chunked sync can resume after failure
// rather than restart from the configured range start.
//
// Implementations are NOT provided by go-wf — define your own (e.g.,
// Postgres-backed) and pass it via ChunkedSync.WithTracker.
type ProgressTracker[K cmp.Ordered] interface {
	// Cursor returns the last-completed partition.End for the named job.
	// The bool reports whether a cursor has been recorded — the zero value
	// of K may be a meaningful key (e.g., int64(0)).
	Cursor(ctx context.Context, jobName string) (K, bool, error)

	// Advance records that all partitions ending at completed (inclusive)
	// have been successfully processed. Implementations should be
	// idempotent; the workflow may retry the call.
	Advance(ctx context.Context, jobName string, completed K) error
}
