# Observability

go-wf integrates with OpenTelemetry (OTel) to provide distributed tracing and metrics across workflows, activities, and storage operations. When OTel is not configured, all instrumentation wrappers act as zero-overhead pass-throughs.

## How It Works

Instrumentation follows a consistent decorator pattern throughout the framework. Each layer provides a wrapper that adds spans and metrics around an inner implementation. OTel configuration is carried via context using `pkgotel.ConfigFromContext(ctx)`. If the config is nil, the wrapper delegates directly to the inner call with no overhead.

## Workflow-Level Instrumentation

The `workflow` package provides instrumented variants of every workflow pattern. These use Temporal's `workflow.GetLogger` for structured logging at workflow boundaries (start, complete, failed) with timing information.

| Instrumented Wrapper                       | Underlying Workflow              |
|--------------------------------------------|----------------------------------|
| `InstrumentedPipelineWorkflow[I, O]`       | `PipelineWorkflow[I, O]`        |
| `InstrumentedParallelWorkflow[I, O]`       | `ParallelWorkflow[I, O]`        |
| `InstrumentedLoopWorkflow[I, O]`           | `LoopWorkflow[I, O]`            |
| `InstrumentedParameterizedLoopWorkflow[I, O]` | `ParameterizedLoopWorkflow[I, O]` |

### Log Messages

Each wrapper emits structured log entries with a dot-separated prefix:

| Event                         | Key Fields                                             |
|-------------------------------|--------------------------------------------------------|
| `pipeline.start`              | `step_count`, `stop_on_error`                          |
| `pipeline.complete`           | `total_steps`, `success_count`, `failure_count`, `duration` |
| `pipeline.failed`             | `error`, `duration`                                    |
| `parallel.start`              | `task_count`, `max_concurrency`, `failure_strategy`    |
| `parallel.complete`           | `total_tasks`, `success_count`, `failure_count`, `duration` |
| `parallel.failed`             | `error`, `duration`                                    |
| `loop.start`                  | `item_count`, `parallel`, `failure_strategy`           |
| `loop.complete`               | `iterations`, `success_count`, `failure_count`, `duration` |
| `loop.failed`                 | `error`, `duration`                                    |
| `parameterized_loop.start`    | `combination_count`, `parallel`, `failure_strategy`    |
| `parameterized_loop.complete` | `iterations`, `success_count`, `failure_count`, `duration` |
| `parameterized_loop.failed`   | `error`, `duration`                                    |

### Usage

Register the instrumented variant instead of the plain workflow:

```go
w.RegisterWorkflow(workflow.InstrumentedPipelineWorkflow[MyInput, MyOutput])
```

## Activity-Level Instrumentation

### Container Activities

**File:** `container/activity/otel.go`

`InstrumentedStartContainerActivity` wraps a container activity function with OTel spans and metrics. It creates a **service-layer** span via `pkgotel.Layers.StartService(ctx, "docker", "StartContainer", ...)`.

**Span attributes:**

| Attribute               | Description                        |
|-------------------------|------------------------------------|
| `container.image`       | Docker image name                  |
| `container.name`        | Container name                     |
| `container.auto_remove` | Whether the container auto-removes |
| `container.command`     | Command (if set)                   |
| `container.work_dir`    | Working directory (if set)         |
| `container.id`          | Container ID (on completion)       |
| `container.exit_code`   | Exit code (on completion)          |
| `container.duration`    | Execution duration (on completion) |
| `container.endpoint`    | Endpoint URL (if available)        |

**Metrics** (meter scope: `go-wf/container/activity`):

| Metric Name                      | Type             | Attributes               | Description                              |
|----------------------------------|------------------|--------------------------|------------------------------------------|
| `go_wf.container.task.total`     | Int64Counter     | `status`, `image`        | Total number of container task executions |
| `go_wf.container.task.duration`  | Float64Histogram | `image`, `exit_code`     | Duration of container tasks in seconds   |

The `image` attribute uses the base image name (without tag) to keep metric cardinality low.

### Function Activities

**File:** `function/activity/otel.go`

`InstrumentedExecuteFunctionActivity` wraps a function activity with OTel spans and metrics. It creates a **service-layer** span via `pkgotel.Layers.StartService(ctx, "function", "Execute", ...)`.

The instrumentation wrapper is automatically registered via an `init()` hook using `fn.SetActivityInstrumenter`, so importing the `function/activity` package enables instrumentation without additional setup.

**Span attributes:**

| Attribute             | Description                          |
|-----------------------|--------------------------------------|
| `function.name`       | Function name                        |
| `function.has_data`   | Whether input data is provided       |
| `function.work_dir`   | Working directory                    |
| `function.duration`   | Execution duration (on completion)   |
| `function.has_result` | Whether output contains a result     |
| `function.error`      | Error message (on failure)           |

**Metrics** (meter scope: `go-wf/function/activity`):

| Metric Name                      | Type             | Attributes                  | Description                              |
|----------------------------------|------------------|-----------------------------|------------------------------------------|
| `go_wf.function.task.total`      | Int64Counter     | `status`, `function_name`   | Total number of function task executions  |
| `go_wf.function.task.duration`   | Float64Histogram | `function_name`             | Duration of function tasks in seconds     |

## Store Instrumentation

**File:** `workflow/store/otel.go`

`InstrumentedStore` is a decorator that wraps any `RawStore` with OTel spans and metrics. Each store operation (Upload, Download, Delete, Exists, List) gets a **repository-layer** span via `pkgotel.Layers.StartRepository(ctx, "store", "<Operation>", ...)`.

```go
raw := store.NewLocalStore("/tmp/data")
instrumented := store.NewInstrumentedStore(raw)
```

**Span attributes:**

| Attribute       | Description                              |
|-----------------|------------------------------------------|
| `store.key`     | Object key (Upload, Download, Delete, Exists) |
| `store.prefix`  | Key prefix (List)                        |
| `store.exists`  | Whether key exists (Exists)              |
| `store.count`   | Number of keys returned (List)           |

**Metrics** (meter scope: `go-wf/store`):

| Metric Name                       | Type             | Attributes              | Description                        |
|-----------------------------------|------------------|-------------------------|------------------------------------|
| `go_wf.store.operation.total`     | Int64Counter     | `operation`, `status`   | Total number of store operations   |
| `go_wf.store.operation.duration`  | Float64Histogram | `operation`             | Duration of store operations in seconds |

## DataSync Instrumentation

**File:** `datasync/activity/otel.go`, `datasync/activity/sync.go`

The datasync activity (`SyncData`) creates a multi-span trace for the full fetch-map-write pipeline:

1. **Operations layer** (`datasync/Execute`) -- top-level span covering the entire sync
2. **Service layer** (`datasync/Fetch`) -- source fetch phase
3. **Service layer** (`datasync/Map`) -- mapper phase
4. **Repository layer** (`datasync/Write`) -- sink write phase

**Span attributes per phase:**

| Phase   | Attributes                                      |
|---------|-------------------------------------------------|
| Execute | `job`, `source`, `sink`                         |
| Fetch   | `source`, `records` (on success)                |
| Map     | `job`, `records`, `mapped` (on success)         |
| Write   | `sink`, `records`, `inserted`, `updated`, `skipped` (on success) |

**Metrics** (meter scope: `go-wf/datasync`):

| Metric Name                                   | Type             | Attributes                   | Description                          |
|-----------------------------------------------|------------------|------------------------------|--------------------------------------|
| `go_wf.datasync.operations_total`             | Int64Counter     | `job`, `source`, `sink`, `status` | Total datasync operations       |
| `go_wf.datasync.operation_duration_seconds`   | Float64Histogram | `job`, `source`, `sink`      | Datasync operation duration in seconds |
| `go_wf.datasync.records_fetched`              | Int64Counter     | `job`, `source`, `sink`      | Records fetched from source          |
| `go_wf.datasync.records_written`              | Int64Counter     | `job`, `source`, `sink`      | Records written to sink              |

## Span Naming Conventions

All spans follow the pattern `<domain>/<Operation>`:

- `docker/StartContainer` -- container activity
- `function/Execute` -- function activity
- `store/Upload`, `store/Download`, etc. -- store operations
- `datasync/Execute`, `datasync/Fetch`, `datasync/Map`, `datasync/Write` -- datasync phases

## Enabling Observability

1. **Configure OTel providers** using `github.com/jasoet/pkg/v2/otel` and attach the config to context.
2. **Use instrumented wrappers** -- replace plain workflow registrations with `Instrumented*` variants, wrap stores with `NewInstrumentedStore`, and import `container/activity` or `function/activity` packages.
3. When OTel config is absent from context, every wrapper falls through to the inner implementation with no allocation or overhead.
