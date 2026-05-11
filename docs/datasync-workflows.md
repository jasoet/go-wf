# Data Synchronization Workflows

The `datasync` package implements a **Source -> Mapper -> Sink** pipeline for moving data between systems, with optional Temporal orchestration for scheduling, retries, and observability.

## Core Interfaces

### Source

A `Source[T]` fetches records of type `T` from an external system:

```go
type Source[T any] interface {
    Name() string
    Fetch(ctx context.Context) ([]T, error)
}
```

If your source has configuration parameters you want visible in Temporal UI, implement `ParamSource`:

```go
type ParamSource[T any, P any] interface {
    Source[T]
    Params() P
}
```

### Sink

A `Sink[U]` writes transformed records to a destination and returns write statistics:

```go
type Sink[U any] interface {
    Name() string
    Write(ctx context.Context, records []U) (WriteResult, error)
}
```

`WriteResult` tracks how many records were inserted, updated, or skipped:

```go
type WriteResult struct {
    Inserted int `json:"inserted"`
    Updated  int `json:"updated"`
    Skipped  int `json:"skipped"`
}
```

### Mapper

A `Mapper[T, U]` transforms a batch of source records into sink records:

```go
type Mapper[T any, U any] interface {
    Map(ctx context.Context, records []T) ([]U, error)
}
```

Use `MapperFunc` as a shorthand for simple transformations:

```go
mapper := datasync.MapperFunc[APIUser, DBUser](func(ctx context.Context, users []APIUser) ([]DBUser, error) {
    result := make([]DBUser, len(users))
    for i, u := range users {
        result[i] = DBUser{ID: u.ID, Name: u.FullName}
    }
    return result, nil
})
```

When source and sink share the same type, use `IdentityMapper`:

```go
mapper := datasync.IdentityMapper[Record]()
```

## Advanced Mapping

### RecordMapper and DetailedMapper

`RecordMapper` applies a per-record conversion function. Records that return an error are skipped with a warning log rather than failing the entire batch.

```go
type RecordMapFunc[T any, U any] func(record *T) (U, error)
```

Create one with `NewRecordMapper`:

```go
mapper := datasync.NewRecordMapper[APIUser, DBUser]("user-mapper", func(u *APIUser) (DBUser, error) {
    if u.Email == "" {
        return DBUser{}, fmt.Errorf("missing email")
    }
    return DBUser{ID: u.ID, Name: u.FullName, Email: u.Email}, nil
})
```

`RecordMapper` implements `DetailedMapper`, which provides `MapDetailed` returning a `MapResult` with skip tracking:

```go
type MapResult[U any] struct {
    Records     []U      `json:"records"`
    Skipped     int      `json:"skipped"`
    SkipReasons []string `json:"skipReasons,omitempty"`
}

type DetailedMapper[T any, U any] interface {
    Mapper[T, U]
    MapDetailed(ctx context.Context, records []T) MapResult[U]
}
```

## InsertIfAbsentSink

`InsertIfAbsentSink` implements an idempotent write pattern: look up each record by ID, skip if it already exists, create otherwise.

```go
sink := datasync.NewInsertIfAbsentSink[DBUser, string](
    "user-sink",
    func(u *DBUser) string { return u.ID },       // getID
    func(ctx context.Context, id string) (*DBUser, error) {  // find
        return repo.FindByID(ctx, id)
    },
    func(ctx context.Context, u *DBUser) error {   // create
        return repo.Create(ctx, u)
    },
)
```

The constructor takes four arguments:

| Parameter | Type | Purpose |
|-----------|------|---------|
| `name` | `string` | Sink identifier |
| `getID` | `func(r *U) ID` | Extract the record's unique key |
| `find` | `FindFunc[U, ID]` | Look up by ID; return nil if absent |
| `create` | `CreateFunc[U]` | Persist a new record |

## Job

A `Job[T, U]` combines source, mapper, and sink into a complete sync pipeline:

```go
type Job[T any, U any] struct {
    Name     string
    Source   Source[T]
    Mapper   Mapper[T, U]
    Sink     Sink[U]
    Schedule time.Duration

    // Temporal activity configuration
    ActivityTimeout         time.Duration
    HeartbeatTimeout        time.Duration
    MaxRetries              int32
    RetryInitialInterval    time.Duration
    RetryBackoffCoefficient float64
    RetryMaxInterval        time.Duration

    Metadata any
    Store    store.RawStore
}
```

## Builder API

Use `SyncJobBuilder` for fluent job construction with validation:

```go
import (
    "github.com/jasoet/go-wf/datasync/builder"
    "github.com/jasoet/pkg/v2/temporal/job"
)

def, err := builder.NewSyncJobBuilder[APIUser, DBUser]("user-sync").
    WithSource(apiSource).
    WithMapper(mapper).
    WithSink(dbSink).
    WithSchedule(15 * time.Minute).
    WithActivityTimeout(10 * time.Minute).
    WithHeartbeatTimeout(30 * time.Second).
    WithMaxRetries(3).
    WithRetryBackoffCoefficient(2.0).
    WithMetadata(map[string]string{"team": "platform"}).
    Build()
if err != nil {
    log.Fatal(err)
}
```

`Build()` returns `(*job.Definition, error)` and validates that name, source, mapper, sink, and a positive schedule are all set. A non-nil `*job.Definition` is ready to register with a worker and execute via the Temporal client.

The caller registers and starts the workflow using the Definition methods directly:

```go
// Register on a worker
w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)

// Execute a run
input := def.NewInput().(*payload.SyncExecutionInput)
input.JobName = def.Name
run, err := def.Execute(ctx, c, input)
```

For recurring execution, attach a schedule at build time and apply it:

```go
// Schedule is already embedded when WithSchedule is set in the builder.
// Apply it to Temporal after the client is available:
err = def.ApplySchedule(ctx, c)
```

See [Job Definition](job-definition.md) for the full `*job.Definition` API surface.

## Runner

`Runner` executes a single fetch-map-write cycle in-process, useful for testing and simple sync without Temporal.
Unlike the builder, `Runner` does not involve Temporal at all — it is a plain Go struct.

```go
runner := datasync.NewRunner(source, mapper, sink)
result, err := runner.Run(ctx)

fmt.Printf("Fetched: %d, Inserted: %d, Skipped: %d, Duration: %s\n",
    result.TotalFetched,
    result.WriteResult.Inserted,
    result.WriteResult.Skipped,
    result.ProcessingTime)
```

### Result

`Result` captures the outcome of a sync run:

```go
type Result struct {
    TotalFetched   int           `json:"totalFetched"`
    WriteResult    WriteResult   `json:"writeResult"`
    ProcessingTime time.Duration `json:"processingTime"`
}
```

## Worker Setup

A complete worker setup using the builder:

```go
import (
    "github.com/jasoet/go-wf/datasync"
    "github.com/jasoet/go-wf/datasync/builder"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/worker"
)

func main() {
    // Build the Definition
    def, err := builder.NewSyncJobBuilder[APIUser, DBUser]("user-sync").
        WithSource(newAPISource()).
        WithMapper(datasync.NewRecordMapper[APIUser, DBUser]("user-mapper", convertUser)).
        WithSink(datasync.NewInsertIfAbsentSink[DBUser, string]("user-sink", getID, findUser, createUser)).
        WithSchedule(15 * time.Minute).
        WithMaxRetries(3).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // Create Temporal client
    c, err := temporal.NewClient(temporal.DefaultConfig())
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()

    // Create worker and register
    w := worker.New(c, def.TaskQueue, worker.Options{})
    def.Register(w)

    // Optionally create or update the Temporal schedule
    if err := def.ApplySchedule(context.Background(), c); err != nil {
        log.Printf("schedule apply: %v", err)
    }

    w.Run(worker.InterruptCh())
}
```

## Payload Types

The `datasync/payload` package defines the workflow input/output types that implement the core `workflow.TaskInput` and `workflow.TaskOutput` interfaces:

- **`SyncExecutionInput`** -- carries `JobName`, `SourceName`, `SinkName`, and optional `Metadata`. Validates with `go-playground/validator`.
- **`SyncExecutionOutput`** -- reports `TotalFetched`, `Inserted`, `Updated`, `Skipped`, `ProcessingTime`, `Success`, and `Error`.

## Observability

The activity layer automatically records OpenTelemetry metrics and spans:

- **Metrics**: `syncOpsTotal` (counter with success/error status), `syncOpsDuration` (histogram), `syncRecordsFetched`, `syncRecordsWritten`.
- **Traces**: Nested spans for Fetch, Map, and Write operations using the `pkgotel.Layers` API.
- **Heartbeats**: Temporal activity heartbeats are sent after fetch and write phases.

## Partitioned and Date-Range Sync

For workloads that need to process data in date/time partitions — for example, syncing orders day-by-day over a rolling 7-day window — see [Chunked Sync Workflows](chunk-workflows.md). The `datasync/chunk` package provides cursor-based resume, `ContinueAsNew` for unbounded history, and a `DateChunkedSync` helper that handles time-to-key projection automatically.

See also [Job Definition](job-definition.md) for the `*job.Definition` type that every builder produces.
