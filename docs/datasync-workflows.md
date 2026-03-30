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
import "github.com/jasoet/go-wf/datasync/builder"

job, err := builder.NewSyncJobBuilder[APIUser, DBUser]("user-sync").
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
```

`Build()` validates that name, source, mapper, sink, and a positive schedule are all set, returning an error if any are missing.

## Runner

`Runner` executes a single fetch-map-write cycle in-process, useful for testing and simple sync without Temporal:

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

## Registration and Temporal Workflows

### JobRegistration

`BuildRegistration` extracts type-erased metadata from a typed `Job`, useful for listing registered jobs without generic type information:

```go
reg := datasync.BuildRegistration(job, false)
// reg.Name, reg.Schedule, reg.Disabled, reg.SourceName, reg.SinkName
```

### Workflow Registration

The `datasync/workflow` package provides `RegisterJob` and `BuildJobRegistration` to wire jobs into a Temporal worker:

```go
import (
    syncwf "github.com/jasoet/go-wf/datasync/workflow"
    "go.temporal.io/sdk/worker"
)

// Register a job with a Temporal worker
w := worker.New(client, syncwf.TaskQueue("user-sync"), worker.Options{})
syncwf.RegisterJob(w, job)
```

`BuildJobRegistration` creates a `FullJobRegistration` that bundles everything needed to register and schedule a job:

```go
reg := syncwf.BuildJobRegistration(job, false)
// reg.Name, reg.TaskQueue, reg.Schedule, reg.WorkflowInput
// reg.Register(w) -- registers workflow + activities with a worker
```

Each job gets its own task queue named `sync-<jobName>`, and the workflow and activity are registered with names derived from the job name.

## Worker Setup

A complete worker setup typically looks like this:

```go
func main() {
    // Build the job
    job, err := builder.NewSyncJobBuilder[APIUser, DBUser]("user-sync").
        WithSource(newAPISource()).
        WithMapper(datasync.NewRecordMapper[APIUser, DBUser]("user-mapper", convertUser)).
        WithSink(datasync.NewInsertIfAbsentSink[DBUser, string]("user-sink", getID, findUser, createUser)).
        WithSchedule(15 * time.Minute).
        WithMaxRetries(3).
        Build()
    if err != nil {
        log.Fatal(err)
    }

    // Create Temporal client and worker
    c, _ := client.Dial(client.Options{})
    w := worker.New(c, syncwf.TaskQueue(job.Name), worker.Options{})

    // Register and start
    syncwf.RegisterJob(w, job)
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
