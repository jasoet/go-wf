package chunk

import (
	"cmp"
	"context"
	"time"
)

// IteratePartitions walks parts in order, calling fn for each. Between
// successful partitions, sleeper(ctx, sleep) is called. The sleep is skipped
// after the last partition, and skipped entirely when sleep == 0.
//
// Iteration stops promptly if ctx is canceled (returning ctx.Err()) or if fn
// returns an error.
//
// Intended for non-Temporal callers (utilities, scripts, tests). Inside a
// Temporal workflow, the ChunkedSync builder uses an inline equivalent that
// substitutes workflow.Sleep for the sleeper callback so the deterministic
// clock is used.
func IteratePartitions[K cmp.Ordered](
	ctx context.Context,
	parts []Partition[K],
	sleep time.Duration,
	sleeper func(ctx context.Context, d time.Duration),
	fn func(p Partition[K]) error,
) error {
	for i, p := range parts {
		if err := fn(p); err != nil {
			return err
		}
		if i < len(parts)-1 && sleep > 0 && sleeper != nil {
			sleeper(ctx, sleep)
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	return nil
}
