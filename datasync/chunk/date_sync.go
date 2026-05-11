package chunk

import (
	"context"
	"time"

	"github.com/jasoet/pkg/v2/temporal/job"
	"go.temporal.io/sdk/temporal"

	"github.com/jasoet/go-wf/datasync"
)

// DateChunkedSync wraps ChunkedSync[In, Out, int64] with a time.Time-based
// API. Internally, time.Time keys are projected onto Unix-nanosecond int64
// keys so the generic ChunkedSync builder (which requires K cmp.Ordered)
// can be used unchanged.
type DateChunkedSync[In, Out any] struct {
	inner    *ChunkedSync[In, Out, int64]
	loc      *time.Location
	lookBack time.Duration
	chunkSz  time.Duration
}

// NewDateChunkedSync starts a DateChunkedSync builder for a job named name.
func NewDateChunkedSync[In, Out any](name string) *DateChunkedSync[In, Out] {
	return &DateChunkedSync[In, Out]{
		inner: NewChunkedSync[In, Out, int64](name),
		loc:   time.UTC,
	}
}

func (d *DateChunkedSync[In, Out]) LookBack(window time.Duration) *DateChunkedSync[In, Out] {
	d.lookBack = window
	return d
}

func (d *DateChunkedSync[In, Out]) ChunkSize(size time.Duration) *DateChunkedSync[In, Out] {
	d.chunkSz = size
	return d
}

func (d *DateChunkedSync[In, Out]) Timezone(loc *time.Location) *DateChunkedSync[In, Out] {
	if loc != nil {
		d.loc = loc
	}
	return d
}

// TimeFetcher is the time.Time-keyed counterpart to PartitionFetcher.
// It is used by DateChunkedSync.Fetcher instead of the generic
// PartitionFetcher[In, K] because time.Time does not satisfy cmp.Ordered.
type TimeFetcher[In any] func(ctx context.Context, start, end time.Time) ([]In, error)

// TimeProgressTracker is the time.Time-keyed counterpart to ProgressTracker.
// It is used by DateChunkedSync.WithTracker instead of the generic
// ProgressTracker[K] because time.Time does not satisfy cmp.Ordered.
type TimeProgressTracker interface {
	Cursor(ctx context.Context, jobName string) (time.Time, bool, error)
	Advance(ctx context.Context, jobName string, completed time.Time) error
}

// Fetcher accepts a time.Time-based fetcher; it is wrapped to satisfy the
// inner int64-keyed builder.
func (d *DateChunkedSync[In, Out]) Fetcher(f TimeFetcher[In]) *DateChunkedSync[In, Out] {
	d.inner.Fetcher(func(ctx context.Context, start, end int64) ([]In, error) {
		return f(ctx, KeyToTime(start), KeyToTime(end))
	})
	return d
}

func (d *DateChunkedSync[In, Out]) Mapper(m datasync.Mapper[In, Out]) *DateChunkedSync[In, Out] {
	d.inner.Mapper(m)
	return d
}

func (d *DateChunkedSync[In, Out]) Sink(s datasync.Sink[Out]) *DateChunkedSync[In, Out] {
	d.inner.Sink(s)
	return d
}

// WithTracker adapts a time.Time-keyed tracker to the inner int64 representation.
func (d *DateChunkedSync[In, Out]) WithTracker(t TimeProgressTracker) *DateChunkedSync[In, Out] {
	d.inner.WithTracker(timeTrackerAdapter{inner: t})
	return d
}

func (d *DateChunkedSync[In, Out]) PartitionSleep(s time.Duration) *DateChunkedSync[In, Out] {
	d.inner.PartitionSleep(s)
	return d
}

func (d *DateChunkedSync[In, Out]) Schedule(s time.Duration) *DateChunkedSync[In, Out] {
	d.inner.Schedule(s)
	return d
}

func (d *DateChunkedSync[In, Out]) RateLimitRetry(opts RateLimitOpts) *DateChunkedSync[In, Out] {
	d.inner.RateLimitRetry(opts)
	return d
}

func (d *DateChunkedSync[In, Out]) ActivityRetry(p temporal.RetryPolicy) *DateChunkedSync[In, Out] {
	d.inner.ActivityRetry(p)
	return d
}

func (d *DateChunkedSync[In, Out]) ActivityTimeouts(startToClose, hb time.Duration) *DateChunkedSync[In, Out] {
	d.inner.ActivityTimeouts(startToClose, hb)
	return d
}

func (d *DateChunkedSync[In, Out]) MaxPartitionsPerExecution(n int) *DateChunkedSync[In, Out] {
	d.inner.MaxPartitionsPerExecution(n)
	return d
}

func (d *DateChunkedSync[In, Out]) Disabled(b bool) *DateChunkedSync[In, Out] {
	d.inner.Disabled(b)
	return d
}

// Build configures the DatePartitioner from LookBack/ChunkSize/Timezone and
// delegates to ChunkedSync.Build.
func (d *DateChunkedSync[In, Out]) Build() (*job.Definition, error) {
	d.inner.Partitioner(&DatePartitioner{
		Loc:       d.loc,
		LookBack:  d.lookBack,
		ChunkSize: d.chunkSz,
	})
	return d.inner.Build()
}

// timeTrackerAdapter projects a TimeProgressTracker onto the int64-keyed
// ProgressTracker interface used by the inner ChunkedSync.
type timeTrackerAdapter struct {
	inner TimeProgressTracker
}

func (a timeTrackerAdapter) Cursor(ctx context.Context, jobName string) (int64, bool, error) {
	t, exists, err := a.inner.Cursor(ctx, jobName)
	if err != nil {
		return 0, false, err
	}
	if !exists {
		return 0, false, nil
	}
	return TimeToKey(t), true, nil
}

func (a timeTrackerAdapter) Advance(ctx context.Context, jobName string, completed int64) error {
	return a.inner.Advance(ctx, jobName, KeyToTime(completed))
}
