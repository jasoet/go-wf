# Datasync Package

Generic data synchronization workflows for Temporal. Provides a type-safe `Source[T] -> Mapper[T,U] -> Sink[U]` pipeline that runs as a Temporal workflow, with full OpenTelemetry instrumentation.

## Key Features

- **Type-safe pipeline** — `Source[T]`, `Mapper[T,U]`, `Sink[U]` interfaces
- **Fluent builder** — construct sync jobs with `SyncJobBuilder`
- **Built-in helpers** — `RecordMapper`, `InsertIfAbsentSink`, `IdentityMapper`
- **Composable** — implements `workflow.TaskInput`/`TaskOutput` for use with Pipeline, Parallel, and DAG orchestration
- **Scheduled execution** — run sync jobs on a recurring interval
- **OTel instrumented** — full observability out of the box

## Documentation

- [Datasync Workflows Guide](../docs/datasync-workflows.md) — comprehensive usage guide with examples
- [Architecture](../docs/architecture.md) — how this package fits in the overall system
- [Workflow Patterns](../docs/workflow-patterns.md) — orchestration patterns
- [Getting Started](../docs/getting-started.md) — quick start guide

## Quick Example

```go
source := mySource{}
mapper := datasync.NewRecordMapper[Raw, Entity]("convert", convertFn)
sink   := datasync.NewInsertIfAbsentSink[Entity, string]("db", getID, find, create)

job, _ := builder.NewSyncJobBuilder[Raw, Entity]("my-sync").
    WithSource(source).
    WithMapper(mapper).
    WithSink(sink).
    WithSchedule(5 * time.Minute).
    Build()
```
