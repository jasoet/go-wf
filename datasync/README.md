# datasync

Generic data synchronization workflows for Temporal.

Provides a type-safe `Source[T] -> Mapper[T,U] -> Sink[U]` pipeline that runs as a Temporal workflow, with full OTel instrumentation.

## Key Types

- **`Source[T]`** -- Fetches records from an external system
- **`Sink[U]`** -- Writes records to a destination
- **`Mapper[T,U]`** -- Transforms source records to sink records
- **`Job[T,U]`** -- Bundles Source + Mapper + Sink + schedule into a deployable unit
- **`Runner[T,U]`** -- In-process test helper (no Temporal needed)

## Helpers

- **`RecordMapper[T,U]`** -- Per-record conversion with automatic skip tracking
- **`InsertIfAbsentSink[U,ID]`** -- Check-then-insert deduplication pattern
- **`MapperFunc[T,U]`** -- Adapter for simple mapping functions
- **`IdentityMapper[T]`** -- No-op mapper when Source and Sink share a type

## TaskInput/TaskOutput

`SyncExecutionInput` implements `workflow.TaskInput` and `SyncExecutionOutput` implements `workflow.TaskOutput`, so sync jobs compose with Pipeline, Parallel, and DAG orchestration from the `workflow/` package.

## Quick Start

```go
// Define source, mapper, sink
source := mySource{}
mapper := datasync.NewRecordMapper[Raw, Entity]("convert", convertFn)
sink := datasync.NewInsertIfAbsentSink[Entity, string]("db", getID, find, create)

// Build job
job, _ := builder.NewSyncJobBuilder[Raw, Entity]("my-sync").
    WithSource(source).
    WithMapper(mapper).
    WithSink(sink).
    WithSchedule(5 * time.Minute).
    Build()

// Register with Temporal worker
w := worker.New(client, workflow.TaskQueue("my-sync"), worker.Options{})
workflow.RegisterJob(w, job)
```

## Packages

| Package | Purpose |
|---------|---------|
| `datasync/` | Core interfaces and helpers |
| `datasync/payload/` | Temporal payload types (TaskInput/TaskOutput) |
| `datasync/activity/` | SyncData activity with OTel |
| `datasync/workflow/` | Workflow function, registration, scheduling helpers |
| `datasync/builder/` | Fluent builder for Job construction |
