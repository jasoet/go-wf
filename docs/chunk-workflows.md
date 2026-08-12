# Chunked Sync Workflows

The `datasync/chunk` package provides Temporal-backed partitioned-sync workflows on top of the datasync primitives (`Mapper`, `Sink`, and `job.Definition`). A `ChunkedSync` walks a list of `Partition[K]` in order, running fetch → map → write per partition. Optionally, a `ProgressTracker[K]` persists progress so long-running syncs resume after failure rather than restart from the beginning of the range.

For time-based partitioning, use `DateChunkedSync` — a thin wrapper that converts `time.Time` keys to `int64` Unix-nanosecond keys internally, because `cmp.Ordered` does not include `time.Time`.

## When to use

- Daily or hourly batches over a date range (e.g., 7 days of orders, one workflow execution per day partition)
- Resumable historical backfills where progress must survive worker restarts
- Long-running syncs that need to bound Temporal history length via `MaxPartitionsPerExecution` + `ContinueAsNew`

Use the simpler `SyncJobBuilder` (see [DataSync Workflows](datasync-workflows.md)) when a single fetch-map-write cycle is sufficient and partitioning is not required.

## Quick Example

The following example is adapted from `examples/datasync/chunk_basic.go`:

```go
import (
    "context"
    "time"

    "github.com/jasoet/pkg/v2/temporal"
    "github.com/jasoet/go-wf/v2/datasync"
    "github.com/jasoet/go-wf/v2/datasync/chunk"
    "github.com/jasoet/go-wf/v2/datasync/payload"
    "go.temporal.io/sdk/worker"
)

// TimeFetcher is a function with the signature func(ctx, start, end time.Time) ([]T, error).
var fetcher chunk.TimeFetcher[Order] = func(ctx context.Context, start, end time.Time) ([]Order, error) {
    // query your data store for records in [start, end)
    return fetchOrdersFromDB(ctx, start, end)
}

def, err := chunk.NewDateChunkedSync[Order, Order]("order-sync").
    LookBack(7 * 24 * time.Hour).   // rolling 7-day window
    ChunkSize(24 * time.Hour).       // one partition per calendar day
    Timezone(time.UTC).
    Fetcher(fetcher).
    Mapper(datasync.IdentityMapper[Order]()).
    Sink(&OrderSink{}).
    ScheduleEvery(15 * time.Minute).
    MaxPartitionsPerExecution(50).
    Build()
if err != nil {
    log.Fatal(err)
}

c, err := temporal.NewClient(temporal.DefaultConfig())
if err != nil {
    log.Fatal(err)
}
defer c.Close()

w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)
go w.Run(worker.InterruptCh())

// Execute manually (the schedule handles recurring runs automatically):
input := def.NewInput().(*payload.SyncExecutionInput)
input.JobName = def.Name
run, err := def.Execute(context.Background(), c, input)
```

Inspect the result:

```go
var result chunk.SyncResult[int64]
if err := run.Get(ctx, &result); err != nil {
    log.Fatal(err)
}
fmt.Printf("Partitions: %d, Fetched: %d, Inserted: %d\n",
    result.TotalPartitions, result.TotalFetched, result.TotalInserted)
for _, p := range result.Partitions {
    start := chunk.KeyToTime(p.Start).Format("2006-01-02 15:04")
    end   := chunk.KeyToTime(p.End).Format("2006-01-02 15:04")
    fmt.Printf("  %s..%s  fetched=%d inserted=%d\n", start, end, p.Fetched, p.Inserted)
}
```

## Core API

### ChunkedSync and DateChunkedSync

`chunk.NewChunkedSync[In, Out, K]` is the generic builder. `K` must satisfy `cmp.Ordered`
(e.g., `int64`, `string`).

`chunk.NewDateChunkedSync[In, Out]` is a `time.Time`-keyed wrapper that projects onto
`int64` Unix-nanosecond keys internally. Prefer this for calendar-based workloads.

Both builders produce a `*job.Definition` via `.Build()`.

### DateChunkedSync builder methods

| Method | Description |
|---|---|
| `LookBack(time.Duration)` | Total window size from now |
| `ChunkSize(time.Duration)` | Size of each partition; window start is aligned to midnight in the configured timezone |
| `Timezone(*time.Location)` | Midnight-alignment timezone (default: UTC) |
| `Fetcher(TimeFetcher[In])` | Data-fetching function for date-range sync |
| `Mapper(datasync.Mapper[In, Out])` | Standard datasync `Mapper` |
| `Sink(datasync.Sink[Out])` | Standard datasync `Sink` |
| `WithTracker(TimeProgressTracker)` | Storage-backed cursor for resumable sync |
| `ScheduleEvery(time.Duration)` | Fixed-interval Temporal schedule |
| `ScheduleCron(string)` | Cron-expression Temporal schedule |
| `ScheduleRaw(*job.ScheduleSpec)` | Full schedule spec (overlap, jitter, calendar) |
| `MaxPartitionsPerExecution(int)` | Cap partitions per run; issues ContinueAsNew for the rest |
| `PartitionSleep(time.Duration)` | Sleep between partitions (emits heartbeats) |
| `ActivityRetry(temporal.RetryPolicy)` | Override default retry policy |
| `ActivityTimeouts(startToClose, heartbeat time.Duration)` | Override default timeouts |
| `RateLimitRetry(RateLimitOpts)` | Decorator for API rate-limit backoff |
| `Disabled(bool)` | Create schedule in paused state |

The generic `ChunkedSync` builder has the same set of methods, but `Fetcher` accepts
`PartitionFetcher[In, K]` and `WithTracker` accepts `ProgressTracker[K]` instead of
the `time.Time` adapter types.

### Partition and result types

```go
// Partition is a half-open range [Start, End) for one unit of work.
type Partition[K cmp.Ordered] struct {
    Start K `json:"start"`
    End   K `json:"end"`
}

// SyncResult aggregates all partitions processed in one workflow execution.
type SyncResult[K cmp.Ordered] struct {
    JobName         string
    TotalPartitions int
    TotalFetched    int
    TotalInserted   int
    TotalUpdated    int
    TotalSkipped    int
    Partitions      []PartitionResult[K]
}
```

For `DateChunkedSync`, `K` is `int64`. Convert keys back to `time.Time` with
`chunk.KeyToTime(k)` and `time.Time` to keys with `chunk.TimeToKey(t)`.

### Partitioner[K]

The `Partitioner[K]` interface generates the ordered list of partitions for a sync run.
Implementations must be deterministic — the output is captured in Temporal history.

`DatePartitioner` (used internally by `DateChunkedSync`) implements `Partitioner[int64]`
and emits calendar-day-aligned partitions. Library consumers do not normally use it directly.

### PartitionFetcher[T, K]

```go
type PartitionFetcher[T any, K cmp.Ordered] func(ctx context.Context, start, end K) ([]T, error)
```

A function that fetches records of type `T` for a single partition `[start, end)`.
Respect `ctx` cancellation.

For `DateChunkedSync`, use the `TimeFetcher[In]` alias:

```go
type TimeFetcher[In any] func(ctx context.Context, start, end time.Time) ([]In, error)
```

### ProgressTracker[K]

```go
type ProgressTracker[K cmp.Ordered] interface {
    Cursor(ctx context.Context, jobName string) (K, bool, error)
    Advance(ctx context.Context, jobName string, completed K) error
}
```

Persists the last-completed partition end key so the workflow can skip
already-processed partitions on the next execution. The `bool` from `Cursor`
indicates whether a cursor has been recorded — the zero value of `K` may be
a meaningful key.

For `DateChunkedSync`, use `TimeProgressTracker`:

```go
type TimeProgressTracker interface {
    Cursor(ctx context.Context, jobName string) (time.Time, bool, error)
    Advance(ctx context.Context, jobName string, completed time.Time) error
}
```

Implementations are not provided by go-wf. Provide your own (e.g., Postgres-backed)
and pass it via `.WithTracker(...)`.

## Resumable Backfills

When `.WithTracker(t)` is set, the workflow:

1. Calls `t.Cursor(ctx, jobName)` to read the last-completed partition end.
2. Filters out partitions whose `Start` is before the cursor value.
3. Processes remaining partitions in order.
4. After each successful partition, calls `t.Advance(ctx, jobName, partition.End)`.

This ensures that, on the next execution, already-processed partitions are skipped.
The `Advance` call is idempotent — the workflow may retry it on failure.

## ContinueAsNew for Large Partition Lists

Each partition processed adds events to the Temporal workflow history. For very long
backfills, this can exhaust the history budget. Use `MaxPartitionsPerExecution(n)` to
cap the number of partitions per execution:

```go
def, err := chunk.NewDateChunkedSync[Order, Order]("orders-backfill").
    LookBack(365 * 24 * time.Hour).  // 1 year of history
    ChunkSize(24 * time.Hour).        // 365 partitions total
    Fetcher(fetcher).
    Mapper(mapper).
    Sink(sink).
    WithTracker(pgTracker).           // required with MaxPartitionsPerExecution
    MaxPartitionsPerExecution(30).    // process 30 days per execution
    Build()
```

After processing 30 partitions, the workflow issues `ContinueAsNew` with the same
input. The tracker ensures the next execution resumes at partition 31 rather than
restarting from day 1.

`MaxPartitionsPerExecution` requires `WithTracker` — without a tracker, every
execution would re-process the same prefix indefinitely. The builder panics at
startup if this combination is invalid.

## Rate-Limit Handling

Decorate a fetcher with exponential-backoff retry on API rate-limit errors:

```go
rateLimited := chunk.WithRateLimitRetry(fetcher, chunk.RateLimitOpts{
    Detector:    isHTTP429,    // func(error) bool
    MaxAttempts: 5,
    Sleep:       30 * time.Second,
    Sleeper:     chunk.HeartbeatSleeper, // emits Temporal heartbeats during sleep
})

def, err := chunk.NewDateChunkedSync[Order, Order]("order-sync").
    Fetcher(chunk.TimeFetcher[Order](func(ctx context.Context, start, end time.Time) ([]Order, error) {
        return rateLimited(ctx, chunk.TimeToKey(start), chunk.TimeToKey(end))
    })).
    // ...
    Build()
```

Or use `.RateLimitRetry(opts)` on the builder to apply the decorator automatically:

```go
def, err := chunk.NewDateChunkedSync[Order, Order]("order-sync").
    Fetcher(myFetcher).
    RateLimitRetry(chunk.RateLimitOpts{
        Detector:    isHTTP429,
        MaxAttempts: 5,
        Sleep:       30 * time.Second,
    }).
    Build()
```

### HeartbeatSleeper

`chunk.HeartbeatSleeper` sleeps for a duration while emitting Temporal activity
heartbeats every 10 seconds, so the activity does not time out during long sleeps:

```go
func HeartbeatSleeper(ctx context.Context, d time.Duration)
```

It is the default `Sleeper` in `RateLimitOpts`. Panics if called outside an
activity context (where `activity.RecordHeartbeat` is not valid).

## Scheduling

Use one of the three schedule setters on the builder before calling `.Build()`:

| Method | Underlying `job.ScheduleSpec` field |
|---|---|
| `.ScheduleEvery(d time.Duration)` | `Interval: d` |
| `.ScheduleCron(expr string)` | `Cron: expr` |
| `.ScheduleRaw(spec *job.ScheduleSpec)` | Full spec — use for jitter, overlap, calendar specs, or paused-on-create |

After building, apply the schedule to Temporal:

```go
if err := def.ApplySchedule(ctx, c); err != nil {
    log.Fatalf("schedule: %v", err)
}
```

`ApplySchedule` creates the schedule if it does not exist, or updates it if it does.
The schedule ID equals `def.Name`.

See [Job Definition](job-definition.md) for the full `*job.ScheduleSpec` shape,
overlap policies, and schedule lifecycle methods (`PauseSchedule`, `ResumeSchedule`,
`TriggerSchedule`, `DeleteSchedule`, `DescribeSchedule`).

## Activity Defaults

When no explicit timeouts or retry policy are configured:

| Setting | Default |
|---|---|
| `StartToCloseTimeout` (partition activity) | 20 minutes |
| `HeartbeatTimeout` (partition activity) | 15 minutes |
| `MaximumAttempts` | 3 |
| `InitialInterval` | 60 seconds |
| `BackoffCoefficient` | 2.0 |
| `MaximumInterval` | 5 minutes |

Cursor read/advance activities use shorter timeouts (10 seconds start-to-close,
3 retries with 2-second initial interval) because they are lightweight metadata
operations against a local store.

## See Also

- [DataSync Workflows](datasync-workflows.md) — the simpler single-activity sync pattern
- [Job Definition](job-definition.md) — the `*job.Definition` shape every builder produces
- `examples/datasync/chunk_basic.go` — complete runnable example
- `datasync/chunk/doc.go` — package-level godoc
