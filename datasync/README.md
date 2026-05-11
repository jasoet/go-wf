# Datasync Package

Generic data synchronization workflows for Temporal. Provides a type-safe `Source[T] -> Mapper[T,U] -> Sink[U]` pipeline that runs as a Temporal workflow, with full OpenTelemetry instrumentation.

## Key Features

- **Type-safe pipeline** — `Source[T]`, `Mapper[T,U]`, `Sink[U]` interfaces
- **Fluent builder** — construct sync jobs with `SyncJobBuilder`; `Build()` returns `*job.Definition`
- **Built-in helpers** — `RecordMapper`, `InsertIfAbsentSink`, `IdentityMapper`
- **Composable** — implements `workflow.TaskInput`/`TaskOutput` for use with Pipeline, Parallel, and DAG orchestration
- **Scheduled execution** — run sync jobs on a recurring interval
- **Partitioned sync** — `datasync/chunk/` for cursor-based, date-chunked sync workflows with progress tracking
- **OTel instrumented** — full observability out of the box

## Documentation

- [Datasync Workflows Guide](../docs/datasync-workflows.md) — comprehensive usage guide with examples
- [Architecture](../docs/architecture.md) — how this package fits in the overall system
- [Workflow Patterns](../docs/workflow-patterns.md) — orchestration patterns
- [Getting Started](../docs/getting-started.md) — quick start guide
- [Job Definition API](../docs/job-definition.md) — `*job.Definition` lifecycle, scheduling, and registry

## Quick Example

```go
source := mySource{}
mapper := datasync.NewRecordMapper[Raw, Entity]("convert", convertFn)
sink   := datasync.NewInsertIfAbsentSink[Entity, string]("db", getID, find, create)

def, err := builder.NewSyncJobBuilder[Raw, Entity]("my-sync").
    WithSource(source).
    WithMapper(mapper).
    WithSink(sink).
    WithSchedule(5 * time.Minute).
    Build()

// def is a *job.Definition
w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)
run, err := def.Execute(ctx, c, def.NewInput())
```

See [docs/job-definition.md](../docs/job-definition.md) for the full `*job.Definition` API.

## Partitioned Sync (`datasync/chunk/`)

For large datasets that must be processed in date or key-range partitions, use `datasync/chunk/`. It provides `ChunkedSync[In, Out, K]` and `NewDateChunkedSync` — a builder that walks partitions with cursor-based resume, optional rate-limit retry, and ContinueAsNew for large partition lists.

```go
def, err := chunk.NewDateChunkedSync[Order, Order]("order-sync").
    LookBack(72 * time.Hour).
    ChunkSize(24 * time.Hour).
    Timezone(time.UTC).
    Fetcher(fetcher).
    Mapper(datasync.IdentityMapper[Order]()).
    Sink(&OrderSink{}).
    ScheduleCron("0 2 * * *").
    Build()
```

Schedule methods: `.ScheduleEvery(d)`, `.ScheduleCron(expr)`, `.ScheduleRaw(spec)`. See [examples/datasync/chunk_basic.go](../examples/datasync/chunk_basic.go) for a runnable example.
